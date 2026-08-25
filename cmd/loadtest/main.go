// cmd/loadtest runs one reproducible load profile against a running server
// and fails (non-zero exit) when the observed p95 latency or error rate
// crosses the profile's budget. It is the executable counterpart to the load
// profile tables in docs/game-frontend-handoff.md and docs/load-testing.md —
// see docs/load-testing.md for the full list of profiles, how the CI smoke
// profile differs from the develop-only soak/spike profiles, and why only
// the CI smoke profile runs unattended today.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	baseURL := flag.String("url", "http://localhost:8081", "base URL of the running server")
	path := flag.String("path", "/healthcheck", "path to request")
	concurrency := flag.Int("concurrency", 10, "number of concurrent workers")
	rps := flag.Float64("rps", 10, "target aggregate requests per second")
	duration := flag.Duration("duration", 2*time.Minute, "how long to run the profile")
	p95BudgetMs := flag.Int64("p95-budget-ms", 5000, "fail if p95 latency exceeds this many milliseconds (0 disables)")
	errorBudgetPercent := flag.Float64(
		"error-budget-percent", 0.5,
		"fail if the aggregate error rate (4xx+5xx+transport failures) exceeds this percentage; "+
			"unlike -p95-budget-ms, 0 does NOT disable this check — it means zero errors tolerated. "+
			"Pass a negative value to disable it entirely",
	)
	maxServerErrors := flag.Int(
		"max-server-errors", -1,
		"fail if more than this many 5xx/transport-failure responses are observed, "+
			"regardless of -error-budget-percent (negative disables this stricter check)",
	)
	timeoutSeconds := flag.Int("request-timeout-seconds", 8, "per-request client timeout")
	flag.Parse()

	client := &http.Client{Timeout: time.Duration(*timeoutSeconds) * time.Second}
	cfg := Config{
		BaseURL:            *baseURL,
		Path:               *path,
		Concurrency:        *concurrency,
		RPS:                *rps,
		Duration:           *duration,
		P95BudgetMs:        *p95BudgetMs,
		ErrorBudgetPercent: *errorBudgetPercent,
	}

	report := run(context.Background(), client, cfg)

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(report)

	if err := report.Evaluate(cfg.P95BudgetMs, cfg.ErrorBudgetPercent, *maxServerErrors); err != nil {
		fmt.Fprintln(os.Stderr, "load profile failed:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "load profile passed")
}
