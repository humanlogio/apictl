package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/aybabtme/hmachttp"
	"github.com/aybabtme/rgbterm"
	"github.com/bufbuild/connect-go"
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
			Name:     flagHMACKeyID,
			Value:    "",
			Required: true,
			EnvVar:   "HMAC_KEY_ID",
		},
		cli.StringFlag{
			Name:     flagHMACPrivateKey,
			Value:    "",
			Required: true,
			EnvVar:   "HMAC_PRIVATE_KEY",
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
	app.Commands = append(app.Commands, cli.Command{
		Name: "get",
		Subcommands: cli.Commands{
			{
				Name: "next-update",
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					updateClient := cliupdatev1connect.NewUpdateServiceClient(client, apiURL)
					_ = updateClient
					// TODO: do something
					return nil
				},
			},
		},
	})
	const (
		flagVersionMajor       = "v.major"
		flagVersionMinor       = "v.minor"
		flagVersionPatch       = "v.patch"
		flagVersionPrereleases = "v.pre"
		flagVersionBuild       = "v.build"
		flagUrl                = "url"
		flagSha256             = "sha256"
		flagSignature          = "sig"
		flagArchitecture       = "arch"
		flagOperatingSystem    = "os"
	)
	app.Commands = append(app.Commands, cli.Command{
		Name: "create",
		Subcommands: cli.Commands{
			{
				Name: "release",
				Flags: []cli.Flag{
					cli.IntFlag{Name: flagVersionMajor, Required: true},
					cli.IntFlag{Name: flagVersionMinor, Required: true},
					cli.IntFlag{Name: flagVersionPatch, Required: true},
					cli.StringSliceFlag{Name: flagVersionPrereleases},
					cli.StringFlag{Name: flagVersionBuild},
					cli.StringFlag{Name: flagUrl, Required: true},
					cli.StringFlag{Name: flagSha256, Required: true},
					cli.StringFlag{Name: flagSignature, Required: true},
					cli.StringFlag{Name: flagArchitecture, Required: true},
					cli.StringFlag{Name: flagOperatingSystem, Required: true},
				},
				Action: func(cctx *cli.Context) error {
					apiURL := cctx.GlobalString(flagAPIURL)
					releaseClient := releasev1connect.NewReleaseServiceClient(client, apiURL)
					req := &releasepb.CreateReleaseRequest{
						Version: &typesv1.Version{
							Major:       int32(cctx.Int(flagVersionMajor)),
							Minor:       int32(cctx.Int(flagVersionMinor)),
							Patch:       int32(cctx.Int(flagVersionPatch)),
							Prereleases: cctx.StringSlice(flagVersionPrereleases),
							Build:       cctx.String(flagVersionBuild),
						},
						Url:             cctx.String(flagUrl),
						Sha256:          cctx.String(flagSha256),
						Signature:       cctx.String(flagSignature),
						Architecture:    cctx.String(flagArchitecture),
						OperatingSystem: cctx.String(flagOperatingSystem),
					}
					res, err := releaseClient.CreateRelease(ctx, &connect.Request[releasepb.CreateReleaseRequest]{Msg: req})
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

	return app
}
