package configurablehttpprovider // import "github.com/Dynatrace/dynatrace-otel-collector/confmap/provider/eecprovider/internal/configurablehttpprovider"

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/confmap"
)

type watcher struct {
	providerCtx context.Context
	reqCtx      context.Context

	getConfigBytes func() ([]byte, error)

	refreshInterval time.Duration
	watcherFunc     confmap.WatcherFunc
	configHash      [32]byte
}

func (w *watcher) watchForChanges() {
	ticker := time.NewTicker(w.refreshInterval)

	for {
		select {
		case <-ticker.C:
			body, err := w.getConfigBytes()
			if err != nil {
				fmt.Printf("Error while polling for new configuration: %s\n", err)
				break
			}
			if w.configHash != sha256.Sum256(body) {
				// If we find that there is new config, notify
				// the Collector and stop watching. A new watcher
				// will be created once the provider's Retrieve
				// method is called again.
				w.watcherFunc(nil)
				return
			}
		case <-w.providerCtx.Done():
			return
		case <-w.reqCtx.Done():
			return
		}
	}
}
