package configurablehttpprovider // import "github.com/Dynatrace/dynatrace-otel-collector/confmap/provider/eecprovider/internal/configurablehttpprovider"

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestWatchForChanges(t *testing.T) {
	wg := &sync.WaitGroup{}
	count := &atomic.Uint32{}
	getConfigBytes := func() ([]byte, error) {
		count.Add(1)
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := watcher{
		providerCtx:     ctx,
		reqCtx:          context.Background(),
		getConfigBytes:  getConfigBytes,
		refreshInterval: time.Millisecond,
		watcherFunc: func(ce *confmap.ChangeEvent) {
			require.FailNow(t, "WatcherFunc should be called when config has not changed")
		},
		configHash: sha256.Sum256(nil),
	}

	wg.Add(1)
	go func() {
		w.watchForChanges()
		wg.Done()
	}()
	require.Eventually(t, func() bool {
		return count.Load() > 5
	}, time.Second*3, time.Millisecond*20)
	cancel()
	wg.Wait()
}

func TestStopsOnRequestDone(t *testing.T) {
	wg := &sync.WaitGroup{}
	getConfigBytes := func() ([]byte, error) {
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := watcher{
		providerCtx:     context.Background(),
		reqCtx:          ctx,
		getConfigBytes:  getConfigBytes,
		refreshInterval: time.Hour,
		watcherFunc:     func(ce *confmap.ChangeEvent) {},
		configHash:      sha256.Sum256(nil),
	}

	wg.Add(1)
	go func() {
		w.watchForChanges()
		wg.Done()
	}()
	cancel()
	wg.Wait()
}

func TestCallsWatcherFunc(t *testing.T) {
	wg := &sync.WaitGroup{}
	getConfigBytes := func() ([]byte, error) {
		return []byte("hello"), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := watcher{
		providerCtx:     ctx,
		reqCtx:          context.Background(),
		getConfigBytes:  getConfigBytes,
		refreshInterval: time.Millisecond,
		watcherFunc: func(ce *confmap.ChangeEvent) {
			wg.Done()
		},
		configHash: sha256.Sum256(nil),
	}

	wg.Add(2)
	go func() {
		w.watchForChanges()
		wg.Done()
	}()
	wg.Wait()
	cancel()
}

func TestHandlesGetBodyError(t *testing.T) {
	wg := &sync.WaitGroup{}
	count := &atomic.Uint32{}
	getConfigBytes := func() ([]byte, error) {
		count.Add(1)
		if count.Load()%2 == 1 {
			return nil, errors.New("odd-numbered failure")
		}
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := watcher{
		providerCtx:     ctx,
		reqCtx:          context.Background(),
		getConfigBytes:  getConfigBytes,
		refreshInterval: time.Millisecond * 5,
		watcherFunc: func(ce *confmap.ChangeEvent) {
			require.FailNow(t, "WatcherFunc should be called when config has not changed")
		},
		configHash: sha256.Sum256(nil),
	}

	wg.Add(1)
	go func() {
		w.watchForChanges()
		wg.Done()
	}()
	require.Eventually(t, func() bool {
		return count.Load() > 5
	}, time.Second*3, time.Millisecond*20)
	cancel()
	wg.Wait()
}
