package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFixture(t *testing.T, name string, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

const validOperations = `operations:
  - operationId: getHealth
    method: GET
    path: /health
  - operationId: createWidget
    method: POST
    path: /widgets
`

func validFlow(id string, endpoints string) string {
	return `{"id":"` + id + `","screen":"s","scope":"frontend","priority":"P0","state":"ready","dependencies":[],"endpoints":[` + endpoints + `],"owner":"o","blockers":[],"acceptanceTest":"a","evidence":"e"}`
}

func TestValidate(t *testing.T) {
	t.Run("accepts exact bidirectional operation coverage", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[`+
			validFlow("health", `"getHealth"`)+","+
			validFlow("widgets", `"createWidget"`)+`]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.NoError(t, err)
	})

	t.Run("rejects a flow referencing an operationId absent from the operations manifest", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[`+
			validFlow("health", `"getHealth"`)+","+
			validFlow("widgets", `"createWidget","deleteGhost"`)+`]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "deleteGhost")
	})

	t.Run("rejects an operationId published but not covered by any flow", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[`+validFlow("health", `"getHealth"`)+`]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "createWidget")
	})

	t.Run("rejects the same operationId claimed by two flows", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[`+
			validFlow("health", `"getHealth"`)+","+
			validFlow("also-health", `"getHealth"`)+","+
			validFlow("widgets", `"createWidget"`)+`]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "getHealth")
	})

	t.Run("rejects a duplicate flow id", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[`+
			validFlow("health", `"getHealth"`)+","+
			validFlow("health", `"createWidget"`)+`]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate flow id")
	})

	t.Run("rejects an invalid state", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[
			{"id":"health","screen":"s","scope":"frontend","priority":"P0","state":"maybe","dependencies":[],"endpoints":["getHealth"],"owner":"o","blockers":[],"acceptanceTest":"a","evidence":"e"},
			`+validFlow("widgets", `"createWidget"`)+`]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid state")
	})

	t.Run("rejects an invalid scope", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[
			{"id":"health","screen":"s","scope":"backend","priority":"P0","state":"ready","dependencies":[],"endpoints":["getHealth"],"owner":"o","blockers":[],"acceptanceTest":"a","evidence":"e"},
			`+validFlow("widgets", `"createWidget"`)+`]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid scope")
	})

	t.Run("rejects a blocked flow with no blockers listed", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[
			{"id":"health","screen":"s","scope":"frontend","priority":"P0","state":"blocked","dependencies":[],"endpoints":["getHealth"],"owner":"o","blockers":[],"acceptanceTest":"a","evidence":"e"},
			`+validFlow("widgets", `"createWidget"`)+`]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "blocked")
	})

	t.Run("rejects a flow with a missing required field", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[
			{"id":"health","screen":"","scope":"frontend","priority":"P0","state":"ready","dependencies":[],"endpoints":["getHealth"],"owner":"o","blockers":[],"acceptanceTest":"a","evidence":"e"},
			`+validFlow("widgets", `"createWidget"`)+`]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing a required field")
	})

	t.Run("rejects a flow with no endpoints", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[
			{"id":"health","screen":"s","scope":"frontend","priority":"P0","state":"ready","dependencies":[],"endpoints":[],"owner":"o","blockers":[],"acceptanceTest":"a","evidence":"e"},
			`+validFlow("widgets", `"createWidget"`)+`]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "references no endpoints")
	})

	t.Run("rejects a dependency on an unknown flow id", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[
			{"id":"health","screen":"s","scope":"frontend","priority":"P0","state":"ready","dependencies":["ghost"],"endpoints":["getHealth"],"owner":"o","blockers":[],"acceptanceTest":"a","evidence":"e"},
			`+validFlow("widgets", `"createWidget"`)+`]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown flow id")
	})

	t.Run("accepts a forward-referenced dependency declared later in the file", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[
			{"id":"widgets","screen":"s","scope":"frontend","priority":"P0","state":"ready","dependencies":["health"],"endpoints":["createWidget"],"owner":"o","blockers":[],"acceptanceTest":"a","evidence":"e"},
			`+validFlow("health", `"getHealth"`)+`]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.NoError(t, err)
	})

	t.Run("rejects an empty flow list", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one flow")
	})

	t.Run("rejects a missing manifest file", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)

		// when
		err := validate(filepath.Join(t.TempDir(), "missing.json"), operations)

		// then
		require.Error(t, err)
	})

	t.Run("rejects a missing operations file", func(t *testing.T) {
		// given
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[`+validFlow("health", `"getHealth"`)+`]}`)

		// when
		err := validate(manifest, filepath.Join(t.TempDir(), "missing.yaml"))

		// then
		require.Error(t, err)
	})

	t.Run("rejects malformed JSON", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", validOperations)
		manifest := writeFixture(t, "handoff.json", `not json`)

		// when
		err := validate(manifest, operations)

		// then
		require.Error(t, err)
	})

	t.Run("rejects an operations manifest entry with no operationId", func(t *testing.T) {
		// given
		operations := writeFixture(t, "operations.yaml", `operations:
  - method: GET
    path: /health
`)
		manifest := writeFixture(t, "handoff.json", `{"version":"1","flows":[`+validFlow("health", `"getHealth"`)+`]}`)

		// when
		err := validate(manifest, operations)

		// then
		require.Error(t, err)
	})

	t.Run("validates the real repository manifest against the real operations manifest", func(t *testing.T) {
		// given: the actual files this tool guards in CI

		// when
		err := validate("../../docs/handoff/dnj-v2-frontend-integration.json", "../../docs/openapi/dnj-v2.operations.yaml")

		// then
		require.NoError(t, err)
	})
}
