// Copyright The OpenTelemetry Authors
// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package ctxspan // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxspan"

import "go.opentelemetry.io/collector/pdata/ptrace"

const (
	Name   = "span"
	DocRef = "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlspan"
)

type Context interface {
	GetSpan() ptrace.Span
}
