package main

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"
)

// Config describes one load profile: a fixed concurrency of workers issuing
// requests paced at a target rate for a fixed duration. It intentionally has
// no knobs for request bodies or authentication — this tool exercises public,
// read-only, non-mutating endpoints (healthcheck/readiness today), which is
// enough to validate the server survives sustained concurrent load without
// needing to seed accounts or data in whatever environment it targets.
type Config struct {
	BaseURL            string
	Path               string
	Concurrency        int
	RPS                float64
	Duration           time.Duration
	P95BudgetMs        int64
	ErrorBudgetPercent float64
}

// run executes the profile against client and returns the aggregate report.
// It paces requests against Config.RPS and fans them out to
// Config.Concurrency workers.
//
// Pacing deliberately does NOT use a shared time.Ticker read directly by the
// workers: time.Ticker's channel has a buffer of exactly one, so a tick that
// arrives while every worker is still busy on a slow request is silently
// dropped instead of queued — under sustained saturation this would let the
// offered rate quietly fall below the configured RPS, hiding precisely the
// kind of saturation this tool exists to catch. Instead, a single pacer
// goroutine computes each request's absolute target timestamp
// (start + i*interval) up front and sleeps until it; if a send falls behind
// because workers are momentarily saturated, time.Until for the next target
// is already <= 0, so the pacer issues the backlog immediately once a worker
// frees up rather than losing it — the achieved request count over the full
// duration is never silently reduced by channel buffering.
func run(ctx context.Context, client *http.Client, cfg Config) Report {
	interval := time.Second
	if cfg.RPS > 0 {
		interval = time.Duration(float64(time.Second) / cfg.RPS)
	}

	runCtx, cancel := context.WithTimeout(ctx, cfg.Duration)
	defer cancel()

	concurrency := max(cfg.Concurrency, 1)
	url := cfg.BaseURL + cfg.Path
	jobs := make(chan struct{})
	results := make(chan sample, concurrency*4)
	var workers sync.WaitGroup

	for range concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range jobs {
				// Deliberately use ctx (the caller's context), not runCtx: once a
				// request has been issued within the profile's window, it runs to
				// completion instead of being cut off by the duration timer. A
				// tool that cancels in-flight requests at the exact deadline would
				// misreport an endpoint's real error rate as the timer edge is hit.
				results <- doRequest(ctx, client, url)
			}
		}()
	}

	go func() {
		defer close(jobs)
		start := time.Now()
		for i := 1; ; i++ {
			// i starts at 1, not 0: the first request fires after one interval
			// has elapsed (matching time.Ticker's own semantics, whose channel
			// isn't ready until the first tick), not immediately at i=0 — that
			// keeps an already-cancelled parent context reliably winning the
			// very first select below instead of racing an immediately-ready
			// timer.
			target := start.Add(time.Duration(i) * interval)
			select {
			case <-runCtx.Done():
				return
			case <-time.After(time.Until(target)):
			}
			select {
			case jobs <- struct{}{}:
			case <-runCtx.Done():
				return
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()

	var samples []sample
	collecting := true
	for collecting {
		select {
		case s := <-results:
			samples = append(samples, s)
		case <-done:
			collecting = false
		}
	}
	// Drain whatever landed in the buffer between the last done-check and close.
	for {
		select {
		case s := <-results:
			samples = append(samples, s)
		default:
			return summarize(samples)
		}
	}
}

func doRequest(ctx context.Context, client *http.Client, url string) sample {
	started := time.Now()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return sample{latency: time.Since(started), failed: true, serverError: true}
	}
	response, err := client.Do(request)
	if err != nil {
		// A transport-level failure (connection refused/reset, client timeout)
		// is at least as severe as an HTTP 5xx for the "no server error"
		// budget: the server failed to answer at all.
		return sample{latency: time.Since(started), failed: true, serverError: true}
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	latency := time.Since(started)
	return sample{
		latency:     latency,
		failed:      response.StatusCode >= 400,
		serverError: response.StatusCode >= 500,
	}
}
