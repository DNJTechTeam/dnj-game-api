package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// e2eEvidence is one recorded HTTP call inside a role journey: enough to
// reconstruct, for a human reviewer, what was called, as whom, and what it
// proved -- without leaking secrets (tokens are redacted before recording).
type e2eEvidence struct {
	Journey  string          `json:"journey"`
	Step     int             `json:"step"`
	Label    string          `json:"label"`
	Role     string          `json:"role"`
	Actor    string          `json:"actor"`
	Method   string          `json:"method"`
	Path     string          `json:"path"`
	Status   int             `json:"status"`
	Request  json.RawMessage `json:"request,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
	Proves   string          `json:"proves"`
}

// e2eRecorder wraps adminHTTPRequest, capturing every call made during a
// journey into a JSON transcript under docs/handoff/e2e-evidence/. That
// directory is the source of truth cmd/e2e-report reads to render
// docs/handoff/E2E-EVIDENCE-REPORT.md.
type e2eRecorder struct {
	t       *testing.T
	journey string
	engine  http.Handler
	entries []e2eEvidence
}

func newE2ERecorder(t *testing.T, journey string) *e2eRecorder {
	t.Helper()
	rec := &e2eRecorder{t: t, journey: journey}
	t.Cleanup(rec.dump)
	return rec
}

var bearerTokenPattern = regexp.MustCompile(`"(accessToken|refreshToken|csrfToken)":"[^"]*"`)

func redactTokens(body string) json.RawMessage {
	if body == "" {
		return nil
	}
	redacted := bearerTokenPattern.ReplaceAllString(body, `"$1":"[redacted]"`)
	return json.RawMessage(redacted)
}

// call performs the HTTP request via the shared adminHTTPRequest helper and
// records it. label is a short step name ("admin creates space"), role/actor
// identify who is calling, and proves is the one-sentence claim this call
// backs up (used verbatim in the generated evidence report).
func (r *e2eRecorder) call(label, role, actor, method, path, token, key, body, proves string) *httptest.ResponseRecorder {
	r.t.Helper()
	response := adminHTTPRequest(r.rigEngine(), method, path, body, token, key)
	r.entries = append(r.entries, e2eEvidence{
		Journey:  r.journey,
		Step:     len(r.entries) + 1,
		Label:    label,
		Role:     role,
		Actor:    actor,
		Method:   method,
		Path:     path,
		Status:   response.Code,
		Request:  redactTokens(body),
		Response: redactTokens(response.Body.String()),
		Proves:   proves,
	})
	return response
}

// rigEngine is set by withEngine before the first call; kept separate from
// the constructor so newE2ERecorder(t, journey) stays a one-liner at the top
// of each journey test, matching the rest of the package's setup helpers.
func (r *e2eRecorder) rigEngine() http.Handler {
	if r.engine == nil {
		r.t.Fatal("e2eRecorder used before withEngine(engine) was called")
	}
	return r.engine
}

func (r *e2eRecorder) withEngine(engine http.Handler) *e2eRecorder {
	r.engine = engine
	return r
}

func (r *e2eRecorder) dump() {
	dir := filepath.Join(repoRootForE2EEvidence(), "docs", "handoff", "e2e-evidence")
	require.NoError(r.t, os.MkdirAll(dir, 0o755))
	payload, err := json.MarshalIndent(r.entries, "", "  ")
	require.NoError(r.t, err)
	require.NoError(r.t, os.WriteFile(filepath.Join(dir, r.journey+".json"), payload, 0o644))
}

// repoRootForE2EEvidence resolves the module root regardless of which
// directory `go test` was invoked from, so the evidence always lands in
// docs/handoff/e2e-evidence at the repo root, not inside a temp -test dir.
func repoRootForE2EEvidence() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
