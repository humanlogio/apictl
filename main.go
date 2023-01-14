package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"

	"github.com/aybabtme/hmachttp"
	"github.com/aybabtme/rgbterm"
	"github.com/blang/semver"
	"github.com/bufbuild/connect-go"
	cliupdatepb "github.com/humanlogio/api/go/svc/cliupdate/v1"
	"github.com/humanlogio/api/go/svc/cliupdate/v1/cliupdatev1connect"
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
			prerelease = append(prerelease, versionPrerelease)
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
		flagAccountId               = "account.id"
		flagMachineId               = "machine.id"
	)

	parseVersion := func(cctx *cli.Context) (*typesv1.Version, error) {
		if v := cctx.String(flagVersion); v != "" {
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
		return &typesv1.Version{
			Major:       int32(cctx.Int(flagVersionMajor)),
			Minor:       int32(cctx.Int(flagVersionMinor)),
			Patch:       int32(cctx.Int(flagVersionPatch)),
			Prereleases: cctx.StringSlice(flagVersionPrereleases),
			Build:       cctx.String(flagVersionBuild),
		}, nil
	}

	app.Commands = append(app.Commands, cli.Command{
		Name: "get",
		Subcommands: cli.Commands{
			{
				Name: "next-update",
				Flags: []cli.Flag{
					cli.StringFlag{Name: flagProjectName, Required: true},
					cli.IntFlag{Name: flagAccountId, Required: true},
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
					accountId := cctx.Int64(flagAccountId)
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
							AccountId: accountId,
							MachineId: machineId,
						},
					}))
					if err != nil {
						return err
					}
					msg := res.Msg
					meta := msg.Meta

					if accountId != meta.AccountId {
						log.Printf("an account id was assigned: %d", meta.AccountId)
					}
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
		},
	})

	app.Commands = append(app.Commands, cli.Command{
		Name: "version",
		Subcommands: cli.Commands{
			{
				Name: "check",
				Flags: []cli.Flag{
					cli.IntFlag{Name: flagAccountId, Value: -1},
					cli.IntFlag{Name: flagMachineId, Value: -1},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					updateClient := cliupdatev1connect.NewUpdateServiceClient(client, apiURL)
					accountId := cctx.Int64(flagAccountId)
					machineId := cctx.Int64(flagMachineId)
					res, err := updateClient.GetNextUpdate(ctx, connect.NewRequest(&cliupdatepb.GetNextUpdateRequest{
						ProjectName:            "apictl",
						CurrentVersion:         version,
						MachineArchitecture:    runtime.GOARCH,
						MachineOperatingSystem: runtime.GOOS,
						Meta: &typesv1.ReqMeta{
							AccountId: accountId,
							MachineId: machineId,
						},
					}))
					if err != nil {
						return err
					}
					msg := res.Msg
					meta := msg.Meta
					if accountId != meta.AccountId {
						log.Printf("an account id was assigned: %d", meta.AccountId)
					}
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
					cli.IntFlag{Name: flagAccountId, Value: -1},
					cli.IntFlag{Name: flagMachineId, Value: -1},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					updateClient := cliupdatev1connect.NewUpdateServiceClient(client, apiURL)
					accountId := cctx.Int64(flagAccountId)
					machineId := cctx.Int64(flagMachineId)
					res, err := updateClient.GetNextUpdate(ctx, connect.NewRequest(&cliupdatepb.GetNextUpdateRequest{
						ProjectName:            "apictl",
						CurrentVersion:         version,
						MachineArchitecture:    runtime.GOARCH,
						MachineOperatingSystem: runtime.GOOS,
						Meta: &typesv1.ReqMeta{
							AccountId: accountId,
							MachineId: machineId,
						},
					}))
					if err != nil {
						return err
					}
					msg := res.Msg
					meta := msg.Meta
					if accountId != meta.AccountId {
						log.Printf("an account id was assigned: %d", meta.AccountId)
					}
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
