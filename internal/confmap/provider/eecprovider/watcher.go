// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package eecprovider // import "github.com/Dynatrace/dynatrace-otel-collector/confmap/provider/eecprovider"

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

type watcher struct {
	shutdown chan struct{}

	getConfigBytes func(context.Context) ([]byte, error)

	refreshInterval time.Duration
	watcherFunc     func([]byte)
	configHash      [32]byte
}

func (w *watcher) watchForChanges(ctx context.Context) {
	ticker := time.NewTicker(w.refreshInterval)

	// Setting these ensures the previous request is canceled once we make a new one.
	var reqCtx context.Context
	// Set cancel to an empty function so we don't have to do a nil check every tick.
	var cancel context.CancelFunc = func() {}

	for {
		select {
		case <-ticker.C:
			cancel()
			reqCtx, cancel = context.WithTimeoutCause(ctx, 3*time.Second, errors.New("request to EEC timed out"))
			body, err := w.getConfigBytes(reqCtx)
			if err != nil {
				fmt.Printf("Error while polling for new configuration: %s\n", err)
				break
			}
			if w.configHash != sha256.Sum256(body) {
				// If we find that there is new config, notify
				// the Collector and stop watching. A new watcher
				// will be created once the provider's Retrieve
				// method is called again.
				w.watcherFunc(body)
				return
			}
		case <-w.shutdown:
			return
		case <-ctx.Done():
			return
		}
	}
}
