//go:build tools

package tools

import (
	_ "github.com/hairyhenderson/gomplate/v5/cmd/gomplate"
	_ "github.com/jstemmer/go-junit-report/v2"
	_ "github.com/sigstore/cosign/v3/cmd/cosign"
	_ "go.opentelemetry.io/build-tools/chloggen"
	_ "go.opentelemetry.io/collector/cmd/builder"
)
