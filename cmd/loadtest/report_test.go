package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummarize(t *testing.T) {
	t.Run("empty input produces a zero report", func(t *testing.T) {
		// given: no samples

		// when
		report := summarize(nil)

		// then
		assert.Equal(t, 0, report.Requests)
		assert.Equal(t, 0, report.Errors)
	})

	t.Run("computes percentiles and error rate from ordered latencies", func(t *testing.T) {
		// given
		samples := []sample{
			{latency: 10 * time.Millisecond},
			{latency: 20 * time.Millisecond},
			{latency: 30 * time.Millisecond},
			{latency: 40 * time.Millisecond, failed: true, serverError: true},
			{latency: 100 * time.Millisecond},
		}

		// when
		report := summarize(samples)

		// then
		assert.Equal(t, 5, report.Requests)
		assert.Equal(t, 1, report.Errors)
		assert.Equal(t, 1, report.ServerErrors)
		assert.InDelta(t, 20.0, report.ErrorRate, 0.01)
		assert.Equal(t, 100*time.Millisecond, report.Max)
		assert.Equal(t, 30*time.Millisecond, report.P50)
	})

	t.Run("a single sample is every percentile", func(t *testing.T) {
		// given
		samples := []sample{{latency: 5 * time.Millisecond}}

		// when
		report := summarize(samples)

		// then
		assert.Equal(t, 5*time.Millisecond, report.P50)
		assert.Equal(t, 5*time.Millisecond, report.P95)
		assert.Equal(t, 5*time.Millisecond, report.P99)
	})
}

func TestReport_MarshalJSON(t *testing.T) {
	// given
	report := Report{
		Requests: 10, Errors: 1, ErrorRate: 10,
		P50: 12 * time.Millisecond, P95: 340 * time.Millisecond,
		P99: 900 * time.Millisecond, Max: 2 * time.Second,
	}

	// when
	encoded, err := json.Marshal(report)
	require.NoError(t, err)

	// then
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.InDelta(t, 12, decoded["p50Ms"], 0.01)
	assert.InDelta(t, 340, decoded["p95Ms"], 0.01)
	assert.InDelta(t, 900, decoded["p99Ms"], 0.01)
	assert.InDelta(t, 2000, decoded["maxMs"], 0.01)
}

func TestReport_Evaluate(t *testing.T) {
	t.Run("passes within budget", func(t *testing.T) {
		// given
		report := Report{Requests: 100, P95: 200 * time.Millisecond, ErrorRate: 0.1}

		// when / then
		assert.NoError(t, report.Evaluate(500, 1, -1))
	})

	t.Run("fails on zero requests", func(t *testing.T) {
		// given
		report := Report{}

		// when / then
		assert.Error(t, report.Evaluate(500, 1, -1))
	})

	t.Run("fails when p95 exceeds budget", func(t *testing.T) {
		// given
		report := Report{Requests: 10, P95: 900 * time.Millisecond}

		// when / then
		assert.Error(t, report.Evaluate(500, 100, -1))
	})

	t.Run("fails when error rate exceeds budget", func(t *testing.T) {
		// given
		report := Report{Requests: 10, Errors: 5, ErrorRate: 50, P95: 10 * time.Millisecond}

		// when / then
		assert.Error(t, report.Evaluate(500, 1, -1))
	})

	t.Run("a zero p95 budget disables the latency check", func(t *testing.T) {
		// given
		report := Report{Requests: 10, P95: 10 * time.Second, ErrorRate: 0}

		// when / then
		assert.NoError(t, report.Evaluate(0, 1, -1))
	})

	t.Run("a negative max-server-errors disables that check even with 5xx present", func(t *testing.T) {
		// given
		report := Report{Requests: 10, ServerErrors: 3, ErrorRate: 0, P95: 10 * time.Millisecond}

		// when / then
		assert.NoError(t, report.Evaluate(500, 1, -1))
	})

	t.Run("fails when server errors exceed the budget even within the aggregate error rate", func(t *testing.T) {
		// given
		report := Report{Requests: 1000, ServerErrors: 1, Errors: 1, ErrorRate: 0.1, P95: 10 * time.Millisecond}

		// when / then
		assert.Error(t, report.Evaluate(500, 1, 0))
	})

	t.Run("passes when server errors are within a positive budget", func(t *testing.T) {
		// given
		report := Report{Requests: 1000, ServerErrors: 2, Errors: 2, ErrorRate: 0.2, P95: 10 * time.Millisecond}

		// when / then
		assert.NoError(t, report.Evaluate(500, 1, 2))
	})
}
