package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// sample is one completed request observation. serverError is the stricter
// subset of failed: an HTTP 5xx status or a transport-level failure (the
// server did not answer at all), as opposed to a 4xx client error.
type sample struct {
	latency     time.Duration
	failed      bool
	serverError bool
}

// Report summarizes a completed load profile run against a single endpoint.
// See MarshalJSON: every latency field here is a time.Duration for Evaluate's
// convenience, but is rendered in milliseconds on the wire.
type Report struct {
	Requests     int
	Errors       int
	ServerErrors int
	ErrorRate    float64
	P50          time.Duration
	P95          time.Duration
	P99          time.Duration
	Max          time.Duration
}

// summarize computes percentiles and the error rate from raw samples. It is
// deterministic and side-effect free so it can be unit tested without a
// running server.
func summarize(samples []sample) Report {
	report := Report{Requests: len(samples)}
	if len(samples) == 0 {
		return report
	}

	latencies := make([]time.Duration, 0, len(samples))
	for _, s := range samples {
		latencies = append(latencies, s.latency)
		if s.failed {
			report.Errors++
		}
		if s.serverError {
			report.ServerErrors++
		}
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	percentile := func(p float64) time.Duration {
		if len(latencies) == 1 {
			return latencies[0]
		}
		index := int(p * float64(len(latencies)-1))
		return latencies[index]
	}

	report.P50 = percentile(0.50)
	report.P95 = percentile(0.95)
	report.P99 = percentile(0.99)
	report.Max = latencies[len(latencies)-1]
	report.ErrorRate = 100 * float64(report.Errors) / float64(report.Requests)
	return report
}

// MarshalJSON renders every latency field in milliseconds. time.Duration's
// own JSON encoding is raw nanoseconds, which would silently contradict the
// "Ms"-suffixed field names in every report this tool prints.
func (r Report) MarshalJSON() ([]byte, error) {
	type wire struct {
		Requests     int     `json:"requests"`
		Errors       int     `json:"errors"`
		ServerErrors int     `json:"serverErrors"`
		ErrorRate    float64 `json:"errorRatePercent"`
		P50Ms        int64   `json:"p50Ms"`
		P95Ms        int64   `json:"p95Ms"`
		P99Ms        int64   `json:"p99Ms"`
		MaxMs        int64   `json:"maxMs"`
	}
	return json.Marshal(wire{
		Requests:     r.Requests,
		Errors:       r.Errors,
		ServerErrors: r.ServerErrors,
		ErrorRate:    r.ErrorRate,
		P50Ms:        r.P50.Milliseconds(),
		P95Ms:        r.P95.Milliseconds(),
		P99Ms:        r.P99.Milliseconds(),
		MaxMs:        r.Max.Milliseconds(),
	})
}

// Evaluate fails the profile when the observed p95 latency, aggregate error
// rate, or server-error count crosses the budget declared for the profile —
// this is what makes a load profile a CI gate instead of a report nobody
// reads. maxServerErrors < 0 disables that specific check (it is a stricter,
// opt-in complement to errorBudgetPercent — see the "CI smoke reproduzível"
// row in docs/game-frontend-handoff.md, which additionally requires zero 5xx
// on top of its 0.5% aggregate error budget).
func (r Report) Evaluate(p95BudgetMs int64, errorBudgetPercent float64, maxServerErrors int) error {
	if r.Requests == 0 {
		return fmt.Errorf("load profile produced zero requests")
	}
	p95Ms := r.P95.Milliseconds()
	if p95BudgetMs > 0 && p95Ms > p95BudgetMs {
		return fmt.Errorf("p95 latency %dms exceeds budget %dms", p95Ms, p95BudgetMs)
	}
	if errorBudgetPercent >= 0 && r.ErrorRate > errorBudgetPercent {
		return fmt.Errorf("error rate %.2f%% exceeds budget %.2f%%", r.ErrorRate, errorBudgetPercent)
	}
	if maxServerErrors >= 0 && r.ServerErrors > maxServerErrors {
		return fmt.Errorf("server errors %d exceed budget %d", r.ServerErrors, maxServerErrors)
	}
	return nil
}
