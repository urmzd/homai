package schema

import (
	"encoding/json"
	"testing"
)

func lightSetSchema() json.RawMessage {
	return json.RawMessage(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"state": {"type": "string", "enum": ["ON", "OFF"]},
			"brightness": {"type": "number", "minimum": 0, "maximum": 254}
		},
		"additionalProperties": false
	}`)
}

func TestValidate_ValidPayload(t *testing.T) {
	v := NewValidator()
	schema := lightSetSchema()

	err := v.Validate(schema, map[string]any{
		"state":      "ON",
		"brightness": float64(200),
	})
	if err != nil {
		t.Errorf("expected valid payload, got: %v", err)
	}
}

func TestValidate_StateOnly(t *testing.T) {
	v := NewValidator()
	schema := lightSetSchema()

	err := v.Validate(schema, map[string]any{
		"state": "OFF",
	})
	if err != nil {
		t.Errorf("expected valid payload, got: %v", err)
	}
}

func TestValidate_InvalidEnum(t *testing.T) {
	v := NewValidator()
	schema := lightSetSchema()

	err := v.Validate(schema, map[string]any{
		"state": "INVALID",
	})
	if err == nil {
		t.Error("expected validation error for invalid enum value")
	}
}

func TestValidate_OutOfRange(t *testing.T) {
	v := NewValidator()
	schema := lightSetSchema()

	err := v.Validate(schema, map[string]any{
		"brightness": float64(300),
	})
	if err == nil {
		t.Error("expected validation error for out-of-range brightness")
	}
}

func TestValidate_UnknownProperty(t *testing.T) {
	v := NewValidator()
	schema := lightSetSchema()

	err := v.Validate(schema, map[string]any{
		"state":   "ON",
		"unknown": "value",
	})
	if err == nil {
		t.Error("expected validation error for unknown property")
	}
}

func TestValidate_EmptySchema(t *testing.T) {
	v := NewValidator()

	// Empty schema means no validation
	err := v.Validate(json.RawMessage(`{}`), map[string]any{
		"anything": "goes",
	})
	if err != nil {
		t.Errorf("empty schema should skip validation, got: %v", err)
	}
}

func TestValidate_NilSchema(t *testing.T) {
	v := NewValidator()

	err := v.Validate(nil, map[string]any{
		"anything": "goes",
	})
	if err != nil {
		t.Errorf("nil schema should skip validation, got: %v", err)
	}
}

func TestValidate_WrongType(t *testing.T) {
	v := NewValidator()
	schema := lightSetSchema()

	err := v.Validate(schema, map[string]any{
		"brightness": "not_a_number",
	})
	if err == nil {
		t.Error("expected validation error for wrong type")
	}
}

func TestValidate_CachesSchema(t *testing.T) {
	v := NewValidator()
	schema := lightSetSchema()

	// First call compiles
	err := v.Validate(schema, map[string]any{"state": "ON"})
	if err != nil {
		t.Fatal(err)
	}

	// Second call should use cache
	err = v.Validate(schema, map[string]any{"state": "OFF"})
	if err != nil {
		t.Fatal(err)
	}

	v.mu.RLock()
	cacheSize := len(v.cache)
	v.mu.RUnlock()
	if cacheSize != 1 {
		t.Errorf("expected 1 cached schema, got %d", cacheSize)
	}
}

func TestValidate_NegativeBrightness(t *testing.T) {
	v := NewValidator()
	schema := lightSetSchema()

	err := v.Validate(schema, map[string]any{
		"brightness": float64(-1),
	})
	if err == nil {
		t.Error("expected validation error for negative brightness")
	}
}
