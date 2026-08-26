// cmd/e2e-report renders the JSON transcripts written by the
// TestE2E_DefaultJourney / TestE2E_EventManagerJourney / TestE2E_AdminJourney
// tests (internal/app/services/e2e_*_journey_test.go) into a single
// human-readable Markdown report: docs/handoff/E2E-EVIDENCE-REPORT.md.
//
// Run `go test ./internal/app/services/... -run TestE2E` first to (re)generate
// the JSON evidence under docs/handoff/e2e-evidence/, then run this command
// to regenerate the report. The report is committed; re-run both whenever a
// journey test changes so the doc never drifts from what was actually
// exercised.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultEvidenceDir = "docs/handoff/e2e-evidence"
	defaultOutput      = "docs/handoff/E2E-EVIDENCE-REPORT.md"
)

// evidence mirrors e2eEvidence in internal/app/services/e2e_evidence_test.go.
// It's redefined here (rather than imported) because that type lives in an
// internal _test.go file, which is not part of the importable package.
type evidence struct {
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

// journeyOrder fixes the reading order of the report regardless of
// filesystem directory order: the natural narrative is player, then
// manager, then admin (the superset of both).
var journeyOrder = []string{"default", "event_manager", "admin"}

var journeyTitle = map[string]string{
	"default":       "Jornada: Default (jogador)",
	"event_manager": "Jornada: Event Manager",
	"admin":         "Jornada: Admin",
}

func main() {
	evidenceDir := flag.String("evidence-dir", defaultEvidenceDir, "directory with e2e-journey JSON transcripts")
	output := flag.String("out", defaultOutput, "path to write the rendered Markdown report")
	flag.Parse()

	if err := run(*evidenceDir, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("E2E evidence report generated at " + *output)
}

func run(evidenceDir, output string) error {
	byJourney := make(map[string][]evidence)
	for _, journey := range journeyOrder {
		path := filepath.Join(evidenceDir, journey+".json")
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		var entries []evidence
		if err := json.Unmarshal(raw, &entries); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		byJourney[journey] = entries
	}
	if len(byJourney) == 0 {
		return fmt.Errorf(
			"no evidence found in %s — run `go test ./internal/app/services/... -run TestE2E` first",
			evidenceDir,
		)
	}

	rendered := render(byJourney)
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	return os.WriteFile(output, []byte(rendered), 0o644)
}

func render(byJourney map[string][]evidence) string {
	var b strings.Builder
	b.WriteString("# Relatório de evidências — E2E por jornada de papel\n\n")
	b.WriteString(
		"Gerado a partir de `docs/handoff/e2e-evidence/*.json`, produzido pelos testes " +
			"`internal/app/services/e2e_default_journey_test.go`, `e2e_event_manager_journey_test.go` " +
			"e `e2e_admin_journey_test.go`. Cada linha é uma chamada HTTP real, contra Postgres real " +
			"(testcontainers), através do `gin.Engine` completo — não é uma simulação. " +
			"Regenere com `go test ./internal/app/services/... -run TestE2E && go run ./cmd/e2e-report` " +
			"sempre que uma jornada mudar.\n\n",
	)

	writeJurisdictionProof(&b, byJourney)

	for _, journey := range journeyOrder {
		entries, ok := byJourney[journey]
		if !ok {
			continue
		}
		title := journeyTitle[journey]
		if title == "" {
			title = journey
		}
		b.WriteString("## " + title + "\n\n")
		fmt.Fprintf(&b, "%d chamadas registradas. Fonte: `docs/handoff/e2e-evidence/%s.json`.\n\n", len(entries), journey)
		b.WriteString("| # | Passo | Papel | Ator | Método | Path | Status | O que prova |\n")
		b.WriteString("|---|---|---|---|---|---|---|---|\n")
		for _, e := range entries {
			fmt.Fprintf(
				&b, "| %d | %s | %s | %s | %s | `%s` | %d | %s |\n",
				e.Step, escapeCell(e.Label), e.Role, escapeCell(e.Actor), e.Method, e.Path, e.Status, escapeCell(e.Proves),
			)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// writeJurisdictionProof looks for the two calls that most directly prove
// the permissions audit's central claim (an Event Manager is scoped to its
// own activities; an Admin sees everything, including a run it was never
// assigned to) and puts them side by side, if both journeys ran.
func writeJurisdictionProof(b *strings.Builder, byJourney map[string][]evidence) {
	managerDenied := findByStatus(byJourney["event_manager"], http404, "run do gestor B")
	adminAllowed := findByStatus(byJourney["admin"], http200, "run do gestor Y sem estar atribuído")
	if managerDenied == nil || adminAllowed == nil {
		return
	}
	b.WriteString("## Prova de jurisdição: o mesmo endpoint, dois vereditos\n\n")
	b.WriteString(
		"`GET /v2/manager/runs/:id` aplicado ao run de **outro** gestor: um Event Manager " +
			"par leva 404 (fora do seu escopo); um Admin, que também nunca foi atribuído àquela " +
			"activity, recebe 200 (alçada global). Esta é a prova executável de que \"alçadas de " +
			"nível Admin sempre veem tudo\" não é só uma leitura de código — é comportamento " +
			"testado.\n\n",
	)
	b.WriteString("| Jornada | Ator | Chamada | Status | Prova |\n")
	b.WriteString("|---|---|---|---|---|\n")
	fmt.Fprintf(
		b, "| Event Manager | %s | `%s %s` | %d | %s |\n",
		managerDenied.Actor, managerDenied.Method, managerDenied.Path, managerDenied.Status, escapeCell(managerDenied.Proves),
	)
	fmt.Fprintf(
		b, "| Admin | %s | `%s %s` | %d | %s |\n\n",
		adminAllowed.Actor, adminAllowed.Method, adminAllowed.Path, adminAllowed.Status, escapeCell(adminAllowed.Proves),
	)
}

const (
	http200 = 200
	http404 = 404
)

func findByStatus(entries []evidence, status int, labelContains string) *evidence {
	for index := range entries {
		e := &entries[index]
		if e.Status == status && strings.Contains(e.Label, labelContains) {
			return e
		}
	}
	return nil
}

func escapeCell(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}
