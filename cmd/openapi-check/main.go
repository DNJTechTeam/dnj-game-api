package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultSpec     = "docs/openapi/dnj-v2.openapi.yaml"
	defaultManifest = "docs/openapi/dnj-v2.operations.yaml"
)

type document struct {
	OpenAPI string                          `yaml:"openapi"`
	Paths   map[string]map[string]yaml.Node `yaml:"paths"`
}

type operation struct {
	OperationID string                 `yaml:"operationId"`
	Responses   map[string]interface{} `yaml:"responses"`
}

type manifest struct {
	Operations []manifestOperation `yaml:"operations"`
}

type manifestOperation struct {
	OperationID    string   `yaml:"operationId"`
	Method         string   `yaml:"method"`
	Path           string   `yaml:"path"`
	Statuses       []string `yaml:"statuses"`
	AutomatedTests []string `yaml:"automatedTests"`
}

func readYAML(path string, target any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(content, target)
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func validate(specPath string, manifestPath string) error {
	var spec document
	if err := readYAML(specPath, &spec); err != nil {
		return fmt.Errorf("read OpenAPI document: %w", err)
	}
	if spec.OpenAPI != "3.0.3" {
		return fmt.Errorf("openapi must be 3.0.3, got %q", spec.OpenAPI)
	}
	if len(spec.Paths) == 0 {
		return fmt.Errorf("openapi must expose at least one path")
	}

	var coverage manifest
	if err := readYAML(manifestPath, &coverage); err != nil {
		return fmt.Errorf("read operation manifest: %w", err)
	}

	documented := make(map[string]struct{})
	locations := make(map[string]string)
	httpMethods := map[string]struct{}{
		"get": {}, "put": {}, "post": {}, "delete": {}, "options": {}, "head": {}, "patch": {}, "trace": {},
	}
	for path, methods := range spec.Paths {
		if !strings.HasPrefix(path, "/") {
			return fmt.Errorf("path %q must start with /", path)
		}
		for method, node := range methods {
			if _, isHTTPMethod := httpMethods[strings.ToLower(method)]; !isHTTPMethod {
				continue
			}
			var operation operation
			if err := node.Decode(&operation); err != nil {
				return fmt.Errorf("decode %s %s: %w", method, path, err)
			}
			method = strings.ToUpper(method)
			if operation.OperationID == "" {
				return fmt.Errorf("%s %s has no operationId", method, path)
			}
			if previous, exists := locations[operation.OperationID]; exists {
				return fmt.Errorf("duplicate operationId %q at %s and %s %s", operation.OperationID, previous, method, path)
			}
			locations[operation.OperationID] = method + " " + path
			documented[operation.OperationID] = struct{}{}
		}
	}

	covered := make(map[string]struct{})
	for _, entry := range coverage.Operations {
		if entry.OperationID == "" || entry.Method == "" || entry.Path == "" || len(entry.AutomatedTests) == 0 {
			return fmt.Errorf("every operation manifest entry requires operationId, method, path and automatedTests")
		}
		for _, automatedTest := range entry.AutomatedTests {
			if _, err := os.Stat(automatedTest); err != nil {
				return fmt.Errorf("operation %s test evidence: %w", entry.OperationID, err)
			}
		}
		methods, exists := spec.Paths[entry.Path]
		if !exists {
			return fmt.Errorf("operation %s references undocumented path %s", entry.OperationID, entry.Path)
		}
		node, exists := methods[strings.ToLower(entry.Method)]
		var op operation
		if !exists {
			return fmt.Errorf("operation %s does not match %s %s in OpenAPI", entry.OperationID, entry.Method, entry.Path)
		}
		if err := node.Decode(&op); err != nil {
			return fmt.Errorf("decode operation %s: %w", entry.OperationID, err)
		}
		if op.OperationID != entry.OperationID {
			return fmt.Errorf("operation %s does not match %s %s in OpenAPI", entry.OperationID, entry.Method, entry.Path)
		}
		for _, status := range entry.Statuses {
			if _, exists := op.Responses[status]; !exists {
				return fmt.Errorf("operation %s status %s is absent from OpenAPI", entry.OperationID, status)
			}
		}
		documentedStatuses := make(map[string]struct{}, len(op.Responses))
		for status := range op.Responses {
			documentedStatuses[status] = struct{}{}
		}
		manifestStatuses := make(map[string]struct{}, len(entry.Statuses))
		for _, status := range entry.Statuses {
			manifestStatuses[status] = struct{}{}
		}
		if strings.Join(sortedKeys(documentedStatuses), "\n") != strings.Join(sortedKeys(manifestStatuses), "\n") {
			return fmt.Errorf("operation %s response statuses and automated-test manifest differ: documented=%v covered=%v", entry.OperationID, sortedKeys(documentedStatuses), sortedKeys(manifestStatuses))
		}
		if _, duplicate := covered[entry.OperationID]; duplicate {
			return fmt.Errorf("duplicate manifest operation %s", entry.OperationID)
		}
		covered[entry.OperationID] = struct{}{}
	}

	if strings.Join(sortedKeys(documented), "\n") != strings.Join(sortedKeys(covered), "\n") {
		return fmt.Errorf("OpenAPI operations and automated-test manifest differ: documented=%v covered=%v", sortedKeys(documented), sortedKeys(covered))
	}
	return nil
}

func writeJSON(specPath string, outputPath string) error {
	var spec any
	if err := readYAML(specPath, &spec); err != nil {
		return err
	}
	content, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(outputPath, content, 0o644)
}

func main() {
	specPath := flag.String("spec", defaultSpec, "OpenAPI YAML document")
	manifestPath := flag.String("manifest", defaultManifest, "automated operation manifest")
	jsonPath := flag.String("write-json", "", "optional generated JSON path")
	flag.Parse()

	if err := validate(*specPath, *manifestPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *jsonPath != "" {
		if err := writeJSON(*specPath, *jsonPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Println("OpenAPI V2 contract and automated-test manifest are consistent")
}
