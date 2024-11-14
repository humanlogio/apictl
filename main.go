package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"connectrpc.com/connect"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aybabtme/hmachttp"
	"github.com/aybabtme/rgbterm"
	"github.com/blang/semver"
	cliupdatepb "github.com/humanlogio/api/go/svc/cliupdate/v1"
	"github.com/humanlogio/api/go/svc/cliupdate/v1/cliupdatev1connect"
	productpb "github.com/humanlogio/api/go/svc/product/v1"
	"github.com/humanlogio/api/go/svc/product/v1/productv1connect"
	releasepb "github.com/humanlogio/api/go/svc/release/v1"
	"github.com/humanlogio/api/go/svc/release/v1/releasev1connect"
	typesv1 "github.com/humanlogio/api/go/types/v1"
	"github.com/humanlogio/apictl/pkg/selfupdate"
	"github.com/mattn/go-colorable"
	"github.com/urfave/cli"
)

var (
	versionMajor      = "0"
	versionMinor      = "0"
	versionPatch      = "0"
	versionPrerelease = "devel"
	versionBuild      = ""
	version           = func() *typesv1.Version {
		var prerelease []string
		if versionPrerelease != "" {
			for _, pre := range strings.Split(versionPrerelease, ".") {
				if pre != "" {
					prerelease = append(prerelease, pre)
				}
			}
		}
		return &typesv1.Version{
			Major:       int32(mustatoi(versionMajor)),
			Minor:       int32(mustatoi(versionMinor)),
			Patch:       int32(mustatoi(versionPatch)),
			Prereleases: prerelease,
			Build:       versionBuild,
		}
	}()
	semverVersion = func() semver.Version {
		v, err := version.AsSemver()
		if err != nil {
			panic(err)
		}
		return v
	}()
)

func main() {
	app := newApp()

	prefix := rgbterm.FgString(app.Name+"> ", 99, 99, 99)

	log.SetOutput(colorable.NewColorableStderr())
	log.SetFlags(0)
	log.SetPrefix(prefix)
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

const (
	flagAPIURL         = "api.url"
	flagHMACKeyID      = "hmac.key_id"
	flagHMACPrivateKey = "hmac.private_key"
)

func newApp() *cli.App {

	app := cli.NewApp()
	app.Author = "Antoine Grondin"
	app.Email = "antoinegrondin@gmail.com"
	app.Name = "apictl"
	app.Version = semverVersion.String()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  flagAPIURL,
			Value: "https://api.humanlog.io",
		},
		cli.StringFlag{
			Name:   flagHMACKeyID,
			Value:  "",
			EnvVar: "HMAC_KEY_ID",
		},
		cli.StringFlag{
			Name:   flagHMACPrivateKey,
			Value:  "",
			EnvVar: "HMAC_PRIVATE_KEY",
		},
	}

	var (
		ctx    context.Context
		cancel context.CancelFunc
		client *http.Client
	)
	app.Before = func(cctx *cli.Context) error {
		ctx, cancel = signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
		client = &http.Client{
			Transport: hmachttp.RoundTripper(
				http.DefaultTransport,
				hmachttp.HeaderKey,
				cctx.GlobalString(flagHMACKeyID),
				[]byte(cctx.GlobalString(flagHMACPrivateKey)),
			),
		}
		return nil
	}
	app.After = func(cctx *cli.Context) error { cancel(); return nil }

	const (
		flagProjectName             = "project"
		flagCategory                = "category"
		flagChannelName             = "channel"
		flagChannelPriority         = "priority"
		flagVersion                 = "version"
		flagVersionMajor            = "major"
		flagVersionMinor            = "minor"
		flagVersionPatch            = "patch"
		flagVersionPrereleases      = "pre"
		flagVersionBuild            = "build"
		flagArtifactUrl             = "url"
		flagArtifactSha256          = "sha256"
		flagArtifactSignature       = "sig"
		flagArtifactArchitecture    = "arch"
		flagArtifactOperatingSystem = "os"
		flagEnvironmentId           = "environment.id"
		flagMachineId               = "machine.id"
		flagS3AccessKey             = "s3.access_key"
		flagS3SecretKey             = "s3.secret_key"
		flagS3Endpoint              = "s3.endpoint"
		flagS3Region                = "s3.region"
		flagS3Bucket                = "s3.bucket"
		flagS3Directory             = "s3.directory"
		flagS3UsePathStyle          = "s3.use_path_style"
		flagS3ACL                   = "s3.acl"
		flagS3CacheControl          = "s3.cache_control"
		flagFilepath                = "filepath"
	)

	parseVersion := func(cctx *cli.Context) (*typesv1.Version, error) {
		if v := cctx.String(flagVersion); v != "" {
			v = strings.TrimPrefix(v, "v")
			vv, err := semver.Parse(v)
			if err != nil {
				return nil, err
			}
			out := &typesv1.Version{
				Major: int32(vv.Major),
				Minor: int32(vv.Minor),
				Patch: int32(vv.Patch),
			}
			for _, pre := range vv.Pre {
				out.Prereleases = append(out.Prereleases, pre.String())
			}
			out.Build = strings.Join(vv.Build, ".")
			return out, nil
		}
		out := &typesv1.Version{
			Major: int32(cctx.Int(flagVersionMajor)),
			Minor: int32(cctx.Int(flagVersionMinor)),
			Patch: int32(cctx.Int(flagVersionPatch)),
			Build: cctx.String(flagVersionBuild),
		}
		for _, pres := range cctx.StringSlice(flagVersionPrereleases) {
			for _, pre := range strings.Split(pres, ".") {
				if pre != "" {
					out.Prereleases = append(out.Prereleases, pre)
				}
			}
		}
		return out, nil
	}

	app.Commands = append(app.Commands, cli.Command{
		Name: "get",
		Subcommands: cli.Commands{
			{
				Name: "next-update",
				Flags: []cli.Flag{
					cli.StringFlag{Name: flagProjectName, Required: true},
					cli.IntFlag{Name: flagEnvironmentId, Required: true},
					cli.IntFlag{Name: flagMachineId, Required: true},
					cli.StringFlag{Name: flagVersion},
					cli.IntFlag{Name: flagVersionMajor},
					cli.IntFlag{Name: flagVersionMinor},
					cli.IntFlag{Name: flagVersionPatch},
					cli.StringSliceFlag{Name: flagVersionPrereleases},
					cli.StringFlag{Name: flagVersionBuild},
					cli.StringFlag{Name: flagArtifactArchitecture, Required: true},
					cli.StringFlag{Name: flagArtifactOperatingSystem, Required: true},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					updateClient := cliupdatev1connect.NewUpdateServiceClient(client, apiURL)
					machineId := cctx.Int64(flagMachineId)
					version, err := parseVersion(cctx)
					if err != nil {
						return err
					}
					cursv, _ := version.AsSemver()
					log.Printf("verifying next update for version %s", cursv)
					res, err := updateClient.GetNextUpdate(ctx, connect.NewRequest(&cliupdatepb.GetNextUpdateRequest{
						ProjectName:            cctx.String(flagProjectName),
						CurrentVersion:         version,
						MachineArchitecture:    cctx.String(flagArtifactArchitecture),
						MachineOperatingSystem: cctx.String(flagArtifactOperatingSystem),
						Meta: &typesv1.ReqMeta{
							MachineId: machineId,
						},
					}))
					if err != nil {
						return err
					}
					msg := res.Msg
					meta := msg.Meta

					if machineId != meta.MachineId {
						log.Printf("a machine id was assigned: %d", meta.MachineId)
					}
					sv, err := msg.NextVersion.AsSemver()
					if err != nil {
						log.Printf("invalid version received: %v", err)
					} else {
						if err := json.NewEncoder(os.Stdout).Encode(sv); err != nil {
							log.Printf("can't encode response to stdout: %v", err)
						}
					}
					log.Printf("version %q is available here:", sv)
					log.Printf("- url: %s", msg.NextArtifact.Url)
					log.Printf("- sha256: %s", msg.NextArtifact.Sha256)
					log.Printf("- sig: %s", msg.NextArtifact.Signature)
					return nil
				},
			},
		},
	})

	app.Commands = append(app.Commands, cli.Command{
		Name: "create",
		Subcommands: cli.Commands{
			{
				Name: "release-channel",
				Flags: []cli.Flag{
					cli.StringFlag{Name: flagProjectName, Required: true},
					cli.StringFlag{Name: flagChannelName, Required: true},
					cli.IntFlag{Name: flagChannelPriority, Required: true},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					releaseClient := releasev1connect.NewReleaseServiceClient(client, apiURL)
					req := &releasepb.CreateReleaseChannelRequest{
						ProjectName:     cctx.String(flagProjectName),
						ChannelName:     cctx.String(flagChannelName),
						ChannelPriority: int32(cctx.Int(flagChannelPriority)),
					}
					res, err := releaseClient.CreateReleaseChannel(ctx, connect.NewRequest(req))
					if err != nil {
						return err
					}
					_ = res
					log.Printf("created")
					return nil
				},
			},
			{
				Name: "published-version",
				Flags: []cli.Flag{
					cli.StringFlag{Name: flagProjectName, Required: true},
					cli.StringFlag{Name: flagChannelName, Required: true},
					cli.StringFlag{Name: flagVersion},
					cli.IntFlag{Name: flagVersionMajor},
					cli.IntFlag{Name: flagVersionMinor},
					cli.IntFlag{Name: flagVersionPatch},
					cli.StringSliceFlag{Name: flagVersionPrereleases},
					cli.StringFlag{Name: flagVersionBuild},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					releaseClient := releasev1connect.NewReleaseServiceClient(client, apiURL)
					version, err := parseVersion(cctx)
					if err != nil {
						return err
					}
					req := &releasepb.PublishVersionRequest{
						ProjectName:        cctx.String(flagProjectName),
						ReleaseChannelName: cctx.String(flagChannelName),
						Version:            version,
					}
					res, err := releaseClient.PublishVersion(ctx, connect.NewRequest(req))
					if err != nil {
						return err
					}
					_ = res
					log.Printf("created")
					return nil
				},
			},
			{
				Name: "s3-artifact",
				Flags: []cli.Flag{
					cli.StringFlag{Name: flagFilepath, Required: true},
					cli.StringFlag{Name: flagS3AccessKey, Required: true},
					cli.StringFlag{Name: flagS3SecretKey, Required: true},
					cli.StringFlag{Name: flagS3Endpoint, Required: true},
					cli.StringFlag{Name: flagS3Region, Required: true},
					cli.StringFlag{Name: flagS3Bucket, Required: true},
					cli.StringFlag{Name: flagS3Directory, Required: true},
					cli.BoolFlag{Name: flagS3UsePathStyle},
					cli.StringFlag{Name: flagS3ACL, Value: string(types.ObjectCannedACLPublicRead)},
					cli.StringFlag{Name: flagS3CacheControl, Value: `max-age=9999,public`},
				},
				Action: func(cctx *cli.Context) error {
					accessKey := cctx.String(flagS3AccessKey)
					secretKey := cctx.String(flagS3SecretKey)
					endpoint := cctx.String(flagS3Endpoint)
					region := cctx.String(flagS3Region)
					bucket := cctx.String(flagS3Bucket)
					directory := cctx.String(flagS3Directory)
					usePathStyle := cctx.Bool(flagS3UsePathStyle)
					acl := cctx.String(flagS3ACL)
					cacheControl := cctx.String(flagS3CacheControl)
					filepath := cctx.String(flagFilepath)

					client := s3.New(s3.Options{
						Region:       region,
						BaseEndpoint: &endpoint,
						UsePathStyle: usePathStyle,
						Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
							accessKey,
							secretKey,
							"",
						)),
					})

					file, err := os.Open(filepath)
					if err != nil {
						return fmt.Errorf("opening filepath %q: %v", filepath, err)
					}
					defer file.Close()

					output, err := client.PutObject(ctx, &s3.PutObjectInput{
						Bucket:       aws.String(bucket),
						Key:          aws.String(directory),
						Body:         file,
						CacheControl: aws.String(cacheControl),
						ACL:          types.ObjectCannedACL(acl),
					})
					if err != nil {
						return fmt.Errorf("putting object %q: %v", filepath, err)
					}
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					if err := enc.Encode(output); err != nil {
						log.Printf("operation succeeded but error printing result: %v", err)
					}
					log.Printf("created in object storage")
					return err
				},
			},
			{
				Name: "version-artifact",
				Flags: []cli.Flag{
					cli.StringFlag{Name: flagProjectName, Required: true},
					cli.StringFlag{Name: flagVersion},
					cli.IntFlag{Name: flagVersionMajor},
					cli.IntFlag{Name: flagVersionMinor},
					cli.IntFlag{Name: flagVersionPatch},
					cli.StringSliceFlag{Name: flagVersionPrereleases},
					cli.StringFlag{Name: flagVersionBuild},
					cli.StringFlag{Name: flagArtifactUrl, Required: true},
					cli.StringFlag{Name: flagArtifactSha256, Required: true},
					cli.StringFlag{Name: flagArtifactSignature, Required: false},
					cli.StringFlag{Name: flagArtifactArchitecture, Required: true},
					cli.StringFlag{Name: flagArtifactOperatingSystem, Required: true},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					releaseClient := releasev1connect.NewReleaseServiceClient(client, apiURL)
					version, err := parseVersion(cctx)
					if err != nil {
						return err
					}
					req := &releasepb.CreateVersionArtifactRequest{
						ProjectName: cctx.String(flagProjectName),
						Version:     version,
						Artifact: &typesv1.VersionArtifact{
							Url:             cctx.String(flagArtifactUrl),
							Sha256:          cctx.String(flagArtifactSha256),
							Signature:       cctx.String(flagArtifactSignature),
							Architecture:    cctx.String(flagArtifactArchitecture),
							OperatingSystem: cctx.String(flagArtifactOperatingSystem),
						},
					}
					res, err := releaseClient.CreateVersionArtifact(ctx, connect.NewRequest(req))
					if err != nil {
						return err
					}
					_ = res
					log.Printf("created")
					// TODO: do something
					return nil
				},
			},
		},
	})
	app.Commands = append(app.Commands, cli.Command{
		Name: "delete",
		Subcommands: cli.Commands{
			{
				Name: "published-version",
				Flags: []cli.Flag{
					cli.StringFlag{Name: flagProjectName, Required: true},
					cli.StringFlag{Name: flagChannelName, Required: true},
					cli.StringFlag{Name: flagVersion},
					cli.IntFlag{Name: flagVersionMajor},
					cli.IntFlag{Name: flagVersionMinor},
					cli.IntFlag{Name: flagVersionPatch},
					cli.StringSliceFlag{Name: flagVersionPrereleases},
					cli.StringFlag{Name: flagVersionBuild},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					releaseClient := releasev1connect.NewReleaseServiceClient(client, apiURL)
					version, err := parseVersion(cctx)
					if err != nil {
						return err
					}
					req := &releasepb.UnpublishVersionRequest{
						ProjectName:        cctx.String(flagProjectName),
						ReleaseChannelName: cctx.String(flagChannelName),
						Version:            version,
					}
					res, err := releaseClient.UnpublishVersion(ctx, connect.NewRequest(req))
					if err != nil {
						return err
					}
					_ = res
					log.Printf("deleted")
					return nil
				},
			},
			{
				Name: "version-artifact",
				Flags: []cli.Flag{
					cli.StringFlag{Name: flagProjectName, Required: true},
					cli.StringFlag{Name: flagVersion},
					cli.IntFlag{Name: flagVersionMajor},
					cli.IntFlag{Name: flagVersionMinor},
					cli.IntFlag{Name: flagVersionPatch},
					cli.StringSliceFlag{Name: flagVersionPrereleases},
					cli.StringFlag{Name: flagVersionBuild},
					cli.StringFlag{Name: flagArtifactUrl, Required: true},
					cli.StringFlag{Name: flagArtifactSha256, Required: true},
					cli.StringFlag{Name: flagArtifactSignature, Required: true},
					cli.StringFlag{Name: flagArtifactArchitecture, Required: true},
					cli.StringFlag{Name: flagArtifactOperatingSystem, Required: true},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					releaseClient := releasev1connect.NewReleaseServiceClient(client, apiURL)
					version, err := parseVersion(cctx)
					if err != nil {
						return err
					}
					req := &releasepb.DeleteVersionArtifactRequest{
						ProjectName: cctx.String(flagProjectName),
						Version:     version,
						Artifact: &typesv1.VersionArtifact{
							Url:             cctx.String(flagArtifactUrl),
							Sha256:          cctx.String(flagArtifactSha256),
							Signature:       cctx.String(flagArtifactSignature),
							Architecture:    cctx.String(flagArtifactArchitecture),
							OperatingSystem: cctx.String(flagArtifactOperatingSystem),
						},
					}
					res, err := releaseClient.DeleteVersionArtifact(ctx, connect.NewRequest(req))
					if err != nil {
						return err
					}
					_ = res
					log.Printf("deleted")
					return nil
				},
			},
		},
	})
	const (
		flagCursor = "cursor"
		flagLimit  = "limit"
	)

	app.Commands = append(app.Commands, cli.Command{
		Name: "list",
		Subcommands: cli.Commands{
			{
				Name: "release-channel",
				Flags: []cli.Flag{
					cli.StringFlag{Name: flagProjectName, Required: true},
					cli.StringFlag{Name: flagCursor},
					cli.Int64Flag{Name: flagLimit},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					releaseClient := releasev1connect.NewReleaseServiceClient(client, apiURL)
					var cursor *typesv1.Cursor
					if opaque := cctx.String(flagCursor); opaque != "" {
						cursor = &typesv1.Cursor{Opaque: []byte(opaque)}
					}
					req := &releasepb.ListReleaseChannelRequest{
						ProjectName: cctx.String(flagProjectName),
						Cursor:      cursor,
						Limit:       cctx.Int64(flagLimit),
					}
					res, err := releaseClient.ListReleaseChannel(ctx, connect.NewRequest(req))
					if err != nil {
						return err
					}
					enc := json.NewEncoder(os.Stdout)
					for _, item := range res.Msg.Items {
						if err := enc.Encode(item.ReleaseChannel); err != nil {
							log.Fatalf("encoding json: %v", err)
						}
					}
					log.Printf("%d results", len(res.Msg.Items))
					if res.Msg.Next != nil {
						log.Printf("more results with --%s=%q", flagCursor, string(res.Msg.Next.Opaque))
					}
					return nil
				},
			},
			{
				Name: "version-artifact",
				Flags: []cli.Flag{
					cli.StringFlag{Name: flagProjectName, Required: true},
					cli.StringFlag{Name: flagCursor},
					cli.Int64Flag{Name: flagLimit},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					releaseClient := releasev1connect.NewReleaseServiceClient(client, apiURL)
					var cursor *typesv1.Cursor
					if opaque := cctx.String(flagCursor); opaque != "" {
						cursor = &typesv1.Cursor{Opaque: []byte(opaque)}
					}
					req := &releasepb.ListVersionArtifactRequest{
						ProjectName: cctx.String(flagProjectName),
						Cursor:      cursor,
						Limit:       cctx.Int64(flagLimit),
					}
					res, err := releaseClient.ListVersionArtifact(ctx, connect.NewRequest(req))
					if err != nil {
						return err
					}
					enc := json.NewEncoder(os.Stdout)
					for _, item := range res.Msg.Items {
						if err := enc.Encode(item); err != nil {
							log.Fatalf("encoding json: %v", err)
						}
					}
					log.Printf("%d results", len(res.Msg.Items))
					if res.Msg.Next != nil {
						log.Printf("more results with --%s=%q", flagCursor, string(res.Msg.Next.Opaque))
					}
					return nil
				},
			},
			{
				Name: "product",
				Flags: []cli.Flag{
					cli.StringFlag{Name: flagCategory},
					cli.StringFlag{Name: flagCursor},
					cli.Int64Flag{Name: flagLimit},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					productClient := productv1connect.NewProductServiceClient(client, apiURL)
					var cursor *typesv1.Cursor
					if opaque := cctx.String(flagCursor); opaque != "" {
						cursor = &typesv1.Cursor{Opaque: []byte(opaque)}
					}
					req := &productpb.ListProductRequest{
						Cursor:   cursor,
						Limit:    int32(cctx.Int(flagLimit)),
						Category: cctx.String(flagCategory),
					}

					res, err := productClient.ListProduct(ctx, connect.NewRequest(req))
					if err != nil {
						return err
					}
					enc := json.NewEncoder(os.Stdout)
					for _, item := range res.Msg.Items {
						if err := enc.Encode(item); err != nil {
							log.Fatalf("encoding json: %v", err)
						}
					}
					log.Printf("%d results", len(res.Msg.Items))
					if res.Msg.Next != nil {
						log.Printf("more results with --%s=%q", flagCursor, string(res.Msg.Next.Opaque))
					}
					return nil
				},
			},
		},
	})

	app.Commands = append(app.Commands, cli.Command{
		Name: "version",
		Subcommands: cli.Commands{
			{
				Name: "check",
				Flags: []cli.Flag{
					cli.IntFlag{Name: flagMachineId, Value: -1},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					updateClient := cliupdatev1connect.NewUpdateServiceClient(client, apiURL)
					machineId := cctx.Int64(flagMachineId)
					res, err := updateClient.GetNextUpdate(ctx, connect.NewRequest(&cliupdatepb.GetNextUpdateRequest{
						ProjectName:            "apictl",
						CurrentVersion:         version,
						MachineArchitecture:    runtime.GOARCH,
						MachineOperatingSystem: runtime.GOOS,
						Meta: &typesv1.ReqMeta{
							MachineId: machineId,
						},
					}))
					if err != nil {
						return err
					}
					msg := res.Msg
					meta := msg.Meta
					if machineId != meta.MachineId {
						log.Printf("a machine id was assigned: %d", meta.MachineId)
					}

					currentSV, err := version.AsSemver()
					if err != nil {
						return err
					}
					nextSV, err := msg.NextVersion.AsSemver()
					if err != nil {
						return err
					}
					if currentSV.GTE(nextSV) {
						log.Printf("you're already running the latest version: v%v", semverVersion.String())
						return nil
					}
					log.Printf("you are running v%s", currentSV)
					log.Printf("a newer version v%s is available here:", nextSV)
					log.Printf("- url: %s", msg.NextArtifact.Url)
					log.Printf("- sha256: %s", msg.NextArtifact.Sha256)
					log.Printf("- sig: %s", msg.NextArtifact.Signature)
					log.Printf("run `apictl version update` to update")
					return nil
				},
			},
			{
				Name: "update",
				Flags: []cli.Flag{
					cli.IntFlag{Name: flagMachineId, Value: -1},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					updateClient := cliupdatev1connect.NewUpdateServiceClient(client, apiURL)
					machineId := cctx.Int64(flagMachineId)
					res, err := updateClient.GetNextUpdate(ctx, connect.NewRequest(&cliupdatepb.GetNextUpdateRequest{
						ProjectName:            "apictl",
						CurrentVersion:         version,
						MachineArchitecture:    runtime.GOARCH,
						MachineOperatingSystem: runtime.GOOS,
						Meta: &typesv1.ReqMeta{
							MachineId: machineId,
						},
					}))
					if err != nil {
						return err
					}
					msg := res.Msg
					meta := msg.Meta
					if machineId != meta.MachineId {
						log.Printf("a machine id was assigned: %d", meta.MachineId)
					}

					currentSV, err := version.AsSemver()
					if err != nil {
						return err
					}
					nextSV, err := msg.NextVersion.AsSemver()
					if err != nil {
						return err
					}
					if currentSV.GTE(nextSV) {
						log.Printf("you're already running the latest version: v%v", semverVersion.String())
						return nil
					}
					return selfupdate.UpgradeInPlace(ctx, "apictl", os.Stdout, os.Stderr, os.Stdin)
				},
			},
			{
				Name: "to-json",
				Flags: []cli.Flag{
					cli.StringFlag{Name: flagVersion},
					cli.IntFlag{Name: flagVersionMajor},
					cli.IntFlag{Name: flagVersionMinor},
					cli.IntFlag{Name: flagVersionPatch},
					cli.StringSliceFlag{Name: flagVersionPrereleases},
					cli.StringFlag{Name: flagVersionBuild},
				},
				Action: func(cctx *cli.Context) error {
					version, err := parseVersion(cctx)
					if err != nil {
						return fmt.Errorf("parsing version flag: %w", err)
					}
					if err := json.NewEncoder(os.Stdout).Encode(version); err != nil {
						return fmt.Errorf("encoding to stdout: %w", err)
					}
					return nil
				},
			},
			{
				Name: "from-json",
				Action: func(cctx *cli.Context) error {
					input := new(typesv1.Version)
					if err := json.NewDecoder(os.Stdin).Decode(input); err != nil {
						return fmt.Errorf("decoding version from stdin: %w", err)
					}
					sv, err := input.AsSemver()
					if err != nil {
						return fmt.Errorf("converting to semver: %v", err)
					}
					_, err = os.Stdout.WriteString(sv.String())
					return err
				},
			},
			{
				Name:  "math",
				Usage: "<lhs> <operator> [<rhs>]",
				Description: "Operate on versions as JSON on stdin.\n" +
					"`<lhs>` can be one of `major`, `minor`, `patch`\n" +
					"`<operator>` can be one of `add`, `sub` or `set`\n" +
					"`<rhs>` is an integer value, relevant depending on <operator>\n",
				Action: func(cctx *cli.Context) error {

					lhs := cctx.Args().Get(0)
					operator := cctx.Args().Get(1)
					rhs := cctx.Args().Get(2)
					parseInt32RHS := func() (int32, error) {
						if rhs == "" {
							return 0, fmt.Errorf("<rhs> can't be empty")
						}
						v, err := strconv.ParseInt(rhs, 10, 32)
						if err != nil {
							return 0, fmt.Errorf("invalid argument for <rhs>: %w", err)
						}
						return int32(v), nil
					}
					parseStringRHS := func() (string, error) {
						return rhs, nil
					}
					parseStringSliceRHS := func() ([]string, error) {
						return strings.Split(rhs, ","), nil
					}

					var (
						load              func(*typesv1.Version) any
						store             func(*typesv1.Version, any)
						operatorFn        func(any) (any, error)
						makeInt32Operator = func(fn func(int32) int32) func(any) (any, error) {
							return func(arg any) (any, error) {
								a, ok := arg.(int32)
								if !ok {
									return nil, fmt.Errorf("<lhs> is not an int")
								}
								return fn(a), nil
							}
						}
						makeStringSliceOperator = func(fn func([]string) []string) func(any) (any, error) {
							return func(arg any) (any, error) {
								a, ok := arg.([]string)
								if !ok {
									return nil, fmt.Errorf("<lhs> is not a slice of string")
								}
								return fn(a), nil
							}
						}
						makeStringOperator = func(fn func(string) string) func(any) (any, error) {
							return func(arg any) (any, error) {
								a, ok := arg.(string)
								if !ok {
									return nil, fmt.Errorf("<lhs> is not a slice of string")
								}
								return fn(a), nil
							}
						}
						operatorType string
					)
					switch lhs {
					default:
						log.Printf("unsupported <lhs>: %q", lhs)
						return cli.ShowSubcommandHelp(cctx)
					case "":
						log.Printf("no <lhs> specified")
						return cli.ShowSubcommandHelp(cctx)
					case "major":
						load = func(version *typesv1.Version) any { return version.Major }
						store = func(version *typesv1.Version, v any) { version.Major = v.(int32) }
						operatorType = "int32"
					case "minor":
						load = func(version *typesv1.Version) any { return version.Minor }
						store = func(version *typesv1.Version, v any) { version.Minor = v.(int32) }
						operatorType = "int32"
					case "patch":
						load = func(version *typesv1.Version) any { return version.Patch }
						store = func(version *typesv1.Version, v any) { version.Patch = v.(int32) }
						operatorType = "int32"
					case "pre":
						load = func(version *typesv1.Version) any { return version.Prereleases }
						store = func(version *typesv1.Version, v any) { version.Prereleases = v.([]string) }
						operatorType = "[]string"
					case "build":
						load = func(version *typesv1.Version) any { return version.Build }
						store = func(version *typesv1.Version, v any) { version.Build = v.(string) }
						operatorType = "string"
					}
					switch operator {
					default:
						log.Printf("unsupported <operator>: %q", operator)
						return cli.ShowSubcommandHelp(cctx)
					case "":
						log.Printf("no <operator> specified")
						return cli.ShowSubcommandHelp(cctx)
					case "add":
						switch operatorType {
						case "int32":
							rhsv, err := parseInt32RHS()
							if err != nil {
								log.Printf("invalid <rhs> for <operator> `add`: %v", err)
								return cli.ShowSubcommandHelp(cctx)
							}
							operatorFn = makeInt32Operator(func(lhsv int32) int32 {
								return lhsv + rhsv
							})
						case "[]string":
							rhsv, err := parseStringRHS()
							if err != nil {
								log.Printf("invalid <rhs> for <operator> `add`: %v", err)
								return cli.ShowSubcommandHelp(cctx)
							}
							operatorFn = makeStringSliceOperator(func(lhsv []string) []string {
								return append(lhsv, rhsv)
							})
						case "string":
							rhsv, err := parseStringRHS()
							if err != nil {
								log.Printf("invalid <rhs> for <operator> `add`: %v", err)
								return cli.ShowSubcommandHelp(cctx)
							}
							operatorFn = makeStringOperator(func(lhsv string) string {
								return lhsv + rhsv
							})
						}

					case "sub":
						switch operatorType {
						case "int32":
							rhsv, err := parseInt32RHS()
							if err != nil {
								log.Printf("invalid <rhs> for <operator> `sub`: %v", err)
								return cli.ShowSubcommandHelp(cctx)
							}
							operatorFn = makeInt32Operator(func(lhsv int32) int32 {
								return lhsv - rhsv
							})
						case "[]string":
							rhsv, err := parseStringRHS()
							if err != nil {
								log.Printf("invalid <rhs> for <operator> `sub`: %v", err)
								return cli.ShowSubcommandHelp(cctx)
							}
							operatorFn = makeStringSliceOperator(func(lhsv []string) []string {
								return slices.DeleteFunc(lhsv, func(v string) bool {
									return v == rhsv
								})
							})
						case "string":
							rhsv, err := parseStringRHS()
							if err != nil {
								log.Printf("invalid <rhs> for <operator> `sub`: %v", err)
								return cli.ShowSubcommandHelp(cctx)
							}
							operatorFn = makeStringOperator(func(lhsv string) string {
								return strings.ReplaceAll(lhsv, rhsv, "")
							})
						}
					case "set":
						switch operatorType {
						case "int32":
							rhsv, err := parseInt32RHS()
							if err != nil {
								log.Printf("invalid <rhs> for <operator> `set`: %v", err)
								return cli.ShowSubcommandHelp(cctx)
							}
							operatorFn = makeInt32Operator(func(_ int32) int32 {
								return rhsv
							})
						case "[]string":
							rhsv, err := parseStringSliceRHS()
							if err != nil {
								log.Printf("invalid <rhs> for <operator> `set`: %v", err)
								return cli.ShowSubcommandHelp(cctx)
							}
							operatorFn = makeStringSliceOperator(func(_ []string) []string {
								return rhsv
							})
						case "string":
							rhsv, err := parseStringRHS()
							if err != nil {
								log.Printf("invalid <rhs> for <operator> `set`: %v", err)
								return cli.ShowSubcommandHelp(cctx)
							}
							operatorFn = makeStringOperator(func(_ string) string {
								return rhsv
							})
						}
					}

					input := new(typesv1.Version)
					if err := json.NewDecoder(os.Stdin).Decode(input); err != nil {
						return fmt.Errorf("decoding version from stdin: %w", err)
					}

					out, err := operatorFn(load(input))
					if err != nil {
						return fmt.Errorf("performing operation: %v", err)
					}
					store(input, out)

					if err := json.NewEncoder(os.Stdout).Encode(input); err != nil {
						return fmt.Errorf("encoding result to stdout: %v", err)
					}
					return nil
				},
			},
		},
	})

	return app
}

func mustatoi(a string) int {
	i, err := strconv.Atoi(a)
	if err != nil {
		panic(err)
	}
	return i
}
