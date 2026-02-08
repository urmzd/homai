package schema

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator validates JSON payloads against JSON Schema documents.
// It caches compiled schemas keyed by their raw bytes.
type Validator struct {
	mu    sync.RWMutex
	cache map[string]*jsonschema.Schema
}

// NewValidator creates a new Validator with an empty cache.
func NewValidator() *Validator {
	return &Validator{
		cache: make(map[string]*jsonschema.Schema),
	}
}

// Validate validates payload against the given JSON Schema document.
// Returns nil if valid, or an error describing the validation failures.
func (v *Validator) Validate(schemaDoc json.RawMessage, payload map[string]any) error {
	if len(schemaDoc) == 0 || string(schemaDoc) == "{}" || string(schemaDoc) == "null" {
		return nil // No schema = no validation
	}

	compiled, err := v.compile(schemaDoc)
	if err != nil {
		return fmt.Errorf("failed to compile schema: %w", err)
	}

	return compiled.Validate(payload)
}

func (v *Validator) compile(schemaDoc json.RawMessage) (*jsonschema.Schema, error) {
	key := string(schemaDoc)

	v.mu.RLock()
	if s, ok := v.cache[key]; ok {
		v.mu.RUnlock()
		return s, nil
	}
	v.mu.RUnlock()

	v.mu.Lock()
	defer v.mu.Unlock()

	// Double-check after acquiring write lock
	if s, ok := v.cache[key]; ok {
		return s, nil
	}

	var schemaMap any
	if err := json.Unmarshal(schemaDoc, &schemaMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", schemaMap); err != nil {
		return nil, fmt.Errorf("failed to add resource: %w", err)
	}
	compiled, err := c.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to compile: %w", err)
	}

	v.cache[key] = compiled
	return compiled, nil
}
