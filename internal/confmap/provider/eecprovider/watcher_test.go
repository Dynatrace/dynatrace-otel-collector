// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package eecprovider // import "github.com/Dynatrace/dynatrace-otel-collector/confmap/provider/eecprovider/eecprovider"

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatchForChanges(t *testing.T) {
	wg := &sync.WaitGroup{}
	count := &atomic.Uint32{}
	getConfigBytes := func(context.Context) ([]byte, error) {
		count.Add(1)
		return nil, nil
	}
	shutdown := make(chan struct{})
	w := watcher{
		shutdown:        shutdown,
		getConfigBytes:  getConfigBytes,
		refreshInterval: time.Millisecond,
		watcherFunc: func([]byte) {
			require.FailNow(t, "WatcherFunc should be called when config has not changed")
		},
		configHash: sha256.Sum256(nil),
	}

	wg.Add(1)
	go func() {
		w.watchForChanges(context.Background())
		wg.Done()
	}()
	require.Eventually(t, func() bool {
		return count.Load() > 5
	}, time.Second*3, time.Millisecond*20)
	close(shutdown)
	wg.Wait()
}

func TestStopsOnRequestDone(t *testing.T) {
	wg := &sync.WaitGroup{}
	getConfigBytes := func(context.Context) ([]byte, error) {
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := watcher{
		getConfigBytes:  getConfigBytes,
		refreshInterval: time.Hour,
		watcherFunc:     func([]byte) {},
		configHash:      sha256.Sum256(nil),
	}

	wg.Add(1)
	go func() {
		w.watchForChanges(ctx)
		wg.Done()
	}()
	cancel()
	wg.Wait()
}

func TestCallsWatcherFunc(t *testing.T) {
	wg := &sync.WaitGroup{}
	body := []byte("hello")
	getConfigBytes := func(context.Context) ([]byte, error) {
		return []byte("hello"), nil
	}
	shutdown := make(chan struct{})
	w := watcher{
		shutdown:        shutdown,
		getConfigBytes:  getConfigBytes,
		refreshInterval: time.Millisecond,
		watcherFunc: func(b []byte) {
			assert.Equal(t, body, b)
			wg.Done()
		},
		configHash: sha256.Sum256(nil),
	}

	wg.Add(2)
	go func() {
		w.watchForChanges(context.Background())
		wg.Done()
	}()
	wg.Wait()
	close(shutdown)
}

func TestHandlesGetBodyError(t *testing.T) {
	wg := &sync.WaitGroup{}
	count := &atomic.Uint32{}
	getConfigBytes := func(context.Context) ([]byte, error) {
		count.Add(1)
		if count.Load()%2 == 1 {
			return nil, errors.New("odd-numbered failure")
		}
		return nil, nil
	}
	shutdown := make(chan struct{})
	w := watcher{
		shutdown:        shutdown,
		getConfigBytes:  getConfigBytes,
		refreshInterval: time.Millisecond * 5,
		watcherFunc: func([]byte) {
			require.FailNow(t, "WatcherFunc should be called when config has not changed")
		},
		configHash: sha256.Sum256(nil),
	}

	wg.Add(1)
	go func() {
		w.watchForChanges(context.Background())
		wg.Done()
	}()
	require.Eventually(t, func() bool {
		return count.Load() > 5
	}, time.Second*3, time.Millisecond*20)
	close(shutdown)
	wg.Wait()
}
