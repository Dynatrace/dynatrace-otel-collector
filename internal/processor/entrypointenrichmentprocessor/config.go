// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import (
	"fmt"
	"regexp"
	"time"
)

// Config holds all configuration for the entrypoint enrichment processor.
type Config struct {
	// WaitDuration is the per-subtree wait timer (starts when local root arrives).
	WaitDuration time.Duration `mapstructure:"wait_duration"`
	// FallbackDuration is a trace-level fallback for spans whose local root never arrives.
	FallbackDuration time.Duration `mapstructure:"fallback_duration"`
	// NumTraces is the upper bound on in-flight traces buffered.
	NumTraces int `mapstructure:"num_traces"`
	// LocalRootDetection controls how local roots are identified.
	LocalRootDetection LocalRootDetectionMode `mapstructure:"local_root_detection"`
	// AttributesToPromote is the list of regex patterns for attribute keys to promote.
	AttributesToPromote []string `mapstructure:"attributes_to_promote"`
	// LocalRootMarkerAttribute, if non-empty, is stamped as `true` on local root spans.
	LocalRootMarkerAttribute string `mapstructure:"local_root_marker_attribute"`

	// compiledPatterns holds the compiled regex patterns from AttributesToPromote.
	compiledPatterns []*regexp.Regexp
}

// Validate checks the config and compiles regex patterns.
func (c *Config) Validate() error {
	if c.WaitDuration < 0 {
		return fmt.Errorf("wait_duration must be >= 0, got %v", c.WaitDuration)
	}
	if c.FallbackDuration < c.WaitDuration {
		return fmt.Errorf("fallback_duration (%v) must be >= wait_duration (%v)", c.FallbackDuration, c.WaitDuration)
	}
	if c.NumTraces <= 0 {
		return fmt.Errorf("num_traces must be > 0, got %d", c.NumTraces)
	}
	switch c.LocalRootDetection {
	case ModeFlagsWithKindFallback, ModeFlagsOnly, ModeKindOnly:
	default:
		return fmt.Errorf("local_root_detection must be one of %q, %q, %q; got %q",
			ModeFlagsWithKindFallback, ModeFlagsOnly, ModeKindOnly, c.LocalRootDetection)
	}
	c.compiledPatterns = make([]*regexp.Regexp, 0, len(c.AttributesToPromote))
	for _, pat := range c.AttributesToPromote {
		re, err := regexp.Compile(pat)
		if err != nil {
			return fmt.Errorf("attributes_to_promote: invalid regex %q: %w", pat, err)
		}
		c.compiledPatterns = append(c.compiledPatterns, re)
	}
	return nil
}
