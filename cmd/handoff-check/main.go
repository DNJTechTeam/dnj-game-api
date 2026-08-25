// cmd/handoff-check verifies that docs/handoff/dnj-v2-frontend-integration.json
// (the machine-readable frontend handoff manifest) does not diverge from the
// published OpenAPI contract: every operationId referenced by a flow must
// exist in docs/openapi/dnj-v2.operations.yaml, every operationId in that
// manifest must be referenced by exactly one flow (no gaps, no duplicates),
// and every flow must carry the fields a reader needs to act on it without
// reading source history. This is the CI gate the Iteration 10 handoff
// requires: "a geração deve falhar no CI se a página ou o manifesto
// divergirem do OpenAPI publicado." The validation logic lives in
// validate.go, which carries its own dedicated coverage gate
// (scripts/check-iteration10-coverage.sh) — this file is only flag/exit
// wiring, excluded from that gate like every other cmd/* entrypoint.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	manifestPath := flag.String("manifest", defaultManifest, "frontend handoff JSON manifest")
	operationsPath := flag.String("operations", defaultOperationsSource, "OpenAPI operations manifest (YAML)")
	flag.Parse()

	if err := validate(*manifestPath, *operationsPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Frontend handoff manifest and OpenAPI operations manifest are consistent")
}
