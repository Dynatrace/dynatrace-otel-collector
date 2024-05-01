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
				fmt.Printf("Error while polling for new configuration: %s", err)
				break
			}
			if w.configHash != sha256.Sum256(body) {
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
