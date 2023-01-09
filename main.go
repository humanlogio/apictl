package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/aybabtme/hmachttp"
	"github.com/aybabtme/rgbterm"
	"github.com/bufbuild/connect-go"
	cliupdatepb "github.com/humanlogio/api/go/svc/cliupdate/v1"
	"github.com/humanlogio/api/go/svc/cliupdate/v1/cliupdatev1connect"
	releasepb "github.com/humanlogio/api/go/svc/release/v1"
	"github.com/humanlogio/api/go/svc/release/v1/releasev1connect"
	typesv1 "github.com/humanlogio/api/go/types/v1"
	"github.com/mattn/go-colorable"
	"github.com/urfave/cli"
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

	app.Commands = append(app.Commands, cli.Command{
		Name: "get",
		Subcommands: cli.Commands{
			{
				Name: "next-update",
				Flags: []cli.Flag{
					cli.StringFlag{Name: flagProjectName, Required: true},
					cli.IntFlag{Name: flagAccountId, Required: true},
					cli.IntFlag{Name: flagMachineId, Required: true},
					cli.IntFlag{Name: flagVersionMajor, Required: true},
					cli.IntFlag{Name: flagVersionMinor, Required: true},
					cli.IntFlag{Name: flagVersionPatch, Required: true},
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
					res, err := updateClient.GetNextUpdate(ctx, connect.NewRequest(&cliupdatepb.GetNextUpdateRequest{
						ProjectName: cctx.String(flagProjectName),
						CurrentVersion: &typesv1.Version{
							Major:       int32(cctx.Int(flagVersionMajor)),
							Minor:       int32(cctx.Int(flagVersionMinor)),
							Patch:       int32(cctx.Int(flagVersionPatch)),
							Prereleases: cctx.StringSlice(flagVersionPrereleases),
							Build:       cctx.String(flagVersionBuild),
						},
						AccountId:              accountId,
						MachineId:              machineId,
						MachineArchitecture:    cctx.String(flagArtifactArchitecture),
						MachineOperatingSystem: cctx.String(flagArtifactOperatingSystem),
					}))
					if err != nil {
						return err
					}
					msg := res.Msg

					if msg.Account != nil && accountId != msg.Account.Id {
						log.Printf("an account id was assigned: %d", msg.Account.Id)
					}
					if msg.Machine != nil && machineId != msg.Machine.Id {
						log.Printf("a machine id was assigned: %d", msg.Machine.Id)
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
					log.Printf("- url: %s", msg.Url)
					log.Printf("- sha256: %s", msg.Sha256)
					log.Printf("- sig: %s", msg.Signature)
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
					cli.IntFlag{Name: flagVersionMajor, Required: true},
					cli.IntFlag{Name: flagVersionMinor, Required: true},
					cli.IntFlag{Name: flagVersionPatch, Required: true},
					cli.StringSliceFlag{Name: flagVersionPrereleases},
					cli.StringFlag{Name: flagVersionBuild},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					releaseClient := releasev1connect.NewReleaseServiceClient(client, apiURL)
					req := &releasepb.PublishVersionRequest{
						ProjectName:        cctx.String(flagProjectName),
						ReleaseChannelName: cctx.String(flagChannelName),
						Version: &typesv1.Version{
							Major:       int32(cctx.Int(flagVersionMajor)),
							Minor:       int32(cctx.Int(flagVersionMinor)),
							Patch:       int32(cctx.Int(flagVersionPatch)),
							Prereleases: cctx.StringSlice(flagVersionPrereleases),
							Build:       cctx.String(flagVersionBuild),
						},
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
					cli.IntFlag{Name: flagVersionMajor, Required: true},
					cli.IntFlag{Name: flagVersionMinor, Required: true},
					cli.IntFlag{Name: flagVersionPatch, Required: true},
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
					req := &releasepb.CreateVersionArtifactRequest{
						ProjectName: cctx.String(flagProjectName),
						Version: &typesv1.Version{
							Major:       int32(cctx.Int(flagVersionMajor)),
							Minor:       int32(cctx.Int(flagVersionMinor)),
							Patch:       int32(cctx.Int(flagVersionPatch)),
							Prereleases: cctx.StringSlice(flagVersionPrereleases),
							Build:       cctx.String(flagVersionBuild),
						},
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
					cli.IntFlag{Name: flagVersionMajor, Required: true},
					cli.IntFlag{Name: flagVersionMinor, Required: true},
					cli.IntFlag{Name: flagVersionPatch, Required: true},
					cli.StringSliceFlag{Name: flagVersionPrereleases},
					cli.StringFlag{Name: flagVersionBuild},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					releaseClient := releasev1connect.NewReleaseServiceClient(client, apiURL)
					req := &releasepb.UnpublishVersionRequest{
						ProjectName:        cctx.String(flagProjectName),
						ReleaseChannelName: cctx.String(flagChannelName),
						Version: &typesv1.Version{
							Major:       int32(cctx.Int(flagVersionMajor)),
							Minor:       int32(cctx.Int(flagVersionMinor)),
							Patch:       int32(cctx.Int(flagVersionPatch)),
							Prereleases: cctx.StringSlice(flagVersionPrereleases),
							Build:       cctx.String(flagVersionBuild),
						},
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
					cli.IntFlag{Name: flagVersionMajor, Required: true},
					cli.IntFlag{Name: flagVersionMinor, Required: true},
					cli.IntFlag{Name: flagVersionPatch, Required: true},
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
					req := &releasepb.DeleteVersionArtifactRequest{
						ProjectName: cctx.String(flagProjectName),
						Version: &typesv1.Version{
							Major:       int32(cctx.Int(flagVersionMajor)),
							Minor:       int32(cctx.Int(flagVersionMinor)),
							Patch:       int32(cctx.Int(flagVersionPatch)),
							Prereleases: cctx.StringSlice(flagVersionPrereleases),
							Build:       cctx.String(flagVersionBuild),
						},
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

	return app
}
