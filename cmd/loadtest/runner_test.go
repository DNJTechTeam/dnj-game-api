package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	t.Run("collects successful samples from a healthy endpoint", func(t *testing.T) {
		// given
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		cfg := Config{
			BaseURL: server.URL, Path: "/healthcheck",
			Concurrency: 4, RPS: 50, Duration: 300 * time.Millisecond,
		}

		// when
		report := run(context.Background(), server.Client(), cfg)

		// then
		require.Greater(t, report.Requests, 0)
		assert.Equal(t, 0, report.Errors)
		require.NoError(t, report.Evaluate(5000, 1, 0))
	})

	t.Run("counts non-2xx responses as errors", func(t *testing.T) {
		// given
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()
		cfg := Config{
			BaseURL: server.URL, Path: "/readiness",
			Concurrency: 2, RPS: 40, Duration: 200 * time.Millisecond,
		}

		// when
		report := run(context.Background(), server.Client(), cfg)

		// then
		require.Greater(t, report.Requests, 0)
		assert.Equal(t, report.Requests, report.Errors)
		assert.Equal(t, report.Requests, report.ServerErrors)
		assert.Error(t, report.Evaluate(5000, 1, -1))
		assert.Error(t, report.Evaluate(5000, 100, 0))
	})

	t.Run("a request timeout counts as a failed sample, not a crash", func(t *testing.T) {
		// given
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		client := &http.Client{Timeout: 10 * time.Millisecond}
		cfg := Config{
			BaseURL: server.URL, Path: "/healthcheck",
			Concurrency: 2, RPS: 40, Duration: 150 * time.Millisecond,
		}

		// when
		report := run(context.Background(), client, cfg)

		// then
		require.Greater(t, report.Requests, 0)
		assert.Equal(t, report.Requests, report.Errors)
		assert.Equal(t, report.Requests, report.ServerErrors)
	})

	t.Run("catches up paced requests after a temporary full-concurrency saturation", func(t *testing.T) {
		// given: the first `concurrency` requests are slow enough to occupy
		// every worker at once for longer than several pacing intervals — the
		// exact condition under which a naive shared time.Ticker (whose
		// channel buffers only one pending tick) silently drops backlog
		// instead of ever issuing it.
		var served atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if served.Add(1) <= 3 {
				time.Sleep(50 * time.Millisecond)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		cfg := Config{
			BaseURL: server.URL, Path: "/healthcheck",
			Concurrency: 3, RPS: 200, Duration: 300 * time.Millisecond,
		}

		// when
		report := run(context.Background(), server.Client(), cfg)

		// then: at 200 RPS for 300ms the profile intends ~60 requests. A
		// pacer that drops ticks during the 50ms saturated window (10
		// intervals at 5ms each) would land well short of that; this pacer
		// recovers the backlog once workers free up.
		assert.GreaterOrEqual(t, report.Requests, 45)
	})

	t.Run("respects a parent context cancellation", func(t *testing.T) {
		// given
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cfg := Config{
			BaseURL: server.URL, Path: "/healthcheck",
			Concurrency: 2, RPS: 10, Duration: time.Second,
		}

		// when
		report := run(ctx, server.Client(), cfg)

		// then
		assert.Equal(t, 0, report.Requests)
	})
}

func TestDoRequest(t *testing.T) {
	t.Run("a malformed base URL fails the request instead of panicking", func(t *testing.T) {
		result := doRequest(context.Background(), http.DefaultClient, "://bad-url")
		assert.True(t, result.failed)
		assert.True(t, result.serverError)
	})
}
