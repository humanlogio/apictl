module github.com/humanlogio/apictl

go 1.22.0

toolchain go1.23.2

require (
	connectrpc.com/connect v1.16.2
	github.com/99designs/keyring v1.2.2
	github.com/aws/aws-sdk-go-v2 v1.32.4
	github.com/aws/aws-sdk-go-v2/credentials v1.17.44
	github.com/aws/aws-sdk-go-v2/service/s3 v1.66.3
	github.com/aybabtme/hmachttp v0.0.0-20221112075348-2e1763138894
	github.com/aybabtme/rgbterm v0.0.0-20170906152045-cc83f3b3ce59
	github.com/blang/semver v3.5.1+incompatible
	github.com/cli/safeexec v1.0.1
	github.com/humanlogio/api/go v0.0.0-20241111064752-147218a45746
	github.com/humanlogio/humanlog v0.7.8
	github.com/mattn/go-colorable v0.1.13
	github.com/urfave/cli v1.22.14
)

require (
	github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.4.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.4 // indirect
	github.com/aws/smithy-go v1.22.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/danieljoos/wincred v1.2.0 // indirect
	github.com/dvsekhvalnov/jose2go v1.6.0 // indirect
	github.com/godbus/dbus v0.0.0-20190726142602-4481cbc300e2 // indirect
	github.com/gsterjov/go-libsecret v0.0.0-20161001094733-a6f4afe4910c // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mtibben/percent v0.2.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/term v0.18.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
)

// replace github.com/humanlogio/api/go => ../api/go/
