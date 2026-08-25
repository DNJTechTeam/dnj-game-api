package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultManifest         = "docs/handoff/dnj-v2-frontend-integration.json"
	defaultOperationsSource = "docs/openapi/dnj-v2.operations.yaml"
)

type operationsManifest struct {
	Operations []struct {
		OperationID string `yaml:"operationId"`
	} `yaml:"operations"`
}

type handoffFlow struct {
	ID             string   `json:"id"`
	Screen         string   `json:"screen"`
	Scope          string   `json:"scope"`
	Iteration      int      `json:"iteration"`
	Priority       string   `json:"priority"`
	State          string   `json:"state"`
	Dependencies   []string `json:"dependencies"`
	Endpoints      []string `json:"endpoints"`
	Owner          string   `json:"owner"`
	Blockers       []string `json:"blockers"`
	AcceptanceTest string   `json:"acceptanceTest"`
	Evidence       string   `json:"evidence"`
}

type handoffManifest struct {
	Version string        `json:"version"`
	Flows   []handoffFlow `json:"flows"`
}

var validStates = map[string]bool{"pending": true, "ready": true, "blocked": true, "done": true}
var validScopes = map[string]bool{"frontend": true, "admin-tooling": true, "enabler": true}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// validate checks that docs/handoff/dnj-v2-frontend-integration.json does not
// diverge from docs/openapi/dnj-v2.operations.yaml: every operationId
// referenced by a flow must exist in the operations manifest, every
// operationId in the operations manifest must be referenced by exactly one
// flow, and every flow must carry the fields a reader needs to act on it
// without reading source history.
func validate(manifestPath string, operationsPath string) error {
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read handoff manifest: %w", err)
	}
	var handoff handoffManifest
	if err := json.Unmarshal(manifestBytes, &handoff); err != nil {
		return fmt.Errorf("parse handoff manifest: %w", err)
	}
	if len(handoff.Flows) == 0 {
		return fmt.Errorf("handoff manifest must declare at least one flow")
	}

	operationsBytes, err := os.ReadFile(operationsPath)
	if err != nil {
		return fmt.Errorf("read operations manifest: %w", err)
	}
	var operations operationsManifest
	if err := yaml.Unmarshal(operationsBytes, &operations); err != nil {
		return fmt.Errorf("parse operations manifest: %w", err)
	}

	published := make(map[string]struct{}, len(operations.Operations))
	for _, op := range operations.Operations {
		if op.OperationID == "" {
			return fmt.Errorf("operations manifest has an entry with no operationId")
		}
		published[op.OperationID] = struct{}{}
	}

	referenced := make(map[string]string) // operationId -> flow id that claims it
	seenFlowIDs := make(map[string]struct{})
	for _, flow := range handoff.Flows {
		if flow.ID == "" || flow.Screen == "" || flow.Owner == "" || flow.AcceptanceTest == "" || flow.Evidence == "" {
			return fmt.Errorf("flow %q is missing a required field (id, screen, owner, acceptanceTest, evidence)", flow.ID)
		}
		if _, duplicate := seenFlowIDs[flow.ID]; duplicate {
			return fmt.Errorf("duplicate flow id %q", flow.ID)
		}
		seenFlowIDs[flow.ID] = struct{}{}
		if !validStates[flow.State] {
			return fmt.Errorf("flow %q has invalid state %q (want pending|ready|blocked|done)", flow.ID, flow.State)
		}
		if !validScopes[flow.Scope] {
			return fmt.Errorf("flow %q has invalid scope %q (want frontend|admin-tooling|enabler)", flow.ID, flow.Scope)
		}
		if flow.State == "blocked" && len(flow.Blockers) == 0 {
			return fmt.Errorf("flow %q is state=blocked but lists no blockers", flow.ID)
		}
		if len(flow.Endpoints) == 0 {
			return fmt.Errorf("flow %q references no endpoints", flow.ID)
		}
		for _, operationID := range flow.Endpoints {
			if _, exists := published[operationID]; !exists {
				return fmt.Errorf("flow %q references operationId %q, which is not in %s", flow.ID, operationID, operationsPath)
			}
			if previous, exists := referenced[operationID]; exists {
				return fmt.Errorf("operationId %q is referenced by both flow %q and flow %q — each operation must belong to exactly one flow", operationID, previous, flow.ID)
			}
			referenced[operationID] = flow.ID
		}
	}

	// Every dependency must reference a real flow id (order-independent —
	// dependencies may be declared later in the file than their target).
	for _, flow := range handoff.Flows {
		for _, dependency := range flow.Dependencies {
			if _, exists := seenFlowIDs[dependency]; !exists {
				return fmt.Errorf("flow %q depends on unknown flow id %q", flow.ID, dependency)
			}
		}
	}

	coveredIDs := make(map[string]struct{}, len(referenced))
	for operationID := range referenced {
		coveredIDs[operationID] = struct{}{}
	}
	if strings.Join(sortedKeys(published), "\n") != strings.Join(sortedKeys(coveredIDs), "\n") {
		var gap []string
		for _, id := range sortedKeys(published) {
			if _, ok := coveredIDs[id]; !ok {
				gap = append(gap, id)
			}
		}
		return fmt.Errorf("handoff manifest and operations manifest diverge — operationIds published but not covered by any flow: %v", gap)
	}

	return nil
}
