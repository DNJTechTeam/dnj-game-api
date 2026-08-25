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

func TestValidate(t *testing.T) {
	t.Run("accepts exact operation and response coverage", func(t *testing.T) {
		// given
		evidence := writeFixture(t, "handler_test.go", "package fixture")
		spec := writeFixture(t, "openapi.yaml", `openapi: 3.0.3
paths:
  /health:
    parameters: []
    get:
      operationId: getHealth
      responses:
        '200': {description: ok}
`)
		manifest := writeFixture(t, "manifest.yaml", `operations:
  - operationId: getHealth
    method: GET
    path: /health
    statuses: ['200']
    automatedTests: [`+evidence+`]
`)

		// when
		err := validate(spec, manifest)

		// then
		require.NoError(t, err)
	})

	t.Run("rejects a documented status without automated evidence", func(t *testing.T) {
		// given
		evidence := writeFixture(t, "handler_test.go", "package fixture")
		spec := writeFixture(t, "openapi.yaml", `openapi: 3.0.3
paths:
  /health:
    get:
      operationId: getHealth
      responses:
        '200': {description: ok}
        '503': {description: unavailable}
`)
		manifest := writeFixture(t, "manifest.yaml", `operations:
  - operationId: getHealth
    method: GET
    path: /health
    statuses: ['200']
    automatedTests: [`+evidence+`]
`)

		// when
		err := validate(spec, manifest)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "statuses")
	})

	t.Run("rejects an operation without a real test evidence file", func(t *testing.T) {
		// given
		spec := writeFixture(t, "openapi.yaml", `openapi: 3.0.3
paths:
  /health:
    get:
      operationId: getHealth
      responses:
        '200': {description: ok}
`)
		manifest := writeFixture(t, "manifest.yaml", `operations:
  - operationId: getHealth
    method: GET
    path: /health
    statuses: ['200']
    automatedTests: [/does/not/exist_test.go]
`)

		// when
		err := validate(spec, manifest)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "test evidence")
	})
}
