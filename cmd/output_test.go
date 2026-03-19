package cmd

import (
	"encoding/json"
	"testing"
)

func TestFormatResponseJSON(t *testing.T) {
	resp := Response{ID: "1", Success: true, Data: "hello"}
	flags := GlobalFlags{JSONOutput: true}
	result := FormatResponse(resp, flags)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got: %s", result)
	}
	if parsed["success"] != true {
		t.Fatalf("expected success=true in JSON output, got %#v", parsed)
	}
}

func TestFormatResponseJSONWithOKCompatibility(t *testing.T) {
	resp := Response{ID: "1", OK: true, Data: map[string]interface{}{"status": "running"}}
	flags := GlobalFlags{JSONOutput: true}
	result := FormatResponse(resp, flags)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got: %s", result)
	}
	if parsed["ok"] != true {
		t.Fatalf("expected ok=true in JSON output, got %#v", parsed)
	}
}

func TestFormatResponseString(t *testing.T) {
	resp := Response{ID: "1", Success: true, Data: "hello world"}
	flags := GlobalFlags{}
	result := FormatResponse(resp, flags)
	if result != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", result)
	}
}

func TestFormatResponseError(t *testing.T) {
	resp := Response{ID: "1", Success: false, Error: "bad thing"}
	flags := GlobalFlags{}
	result := FormatResponse(resp, flags)
	if result != "Error: bad thing" {
		t.Errorf("expected error message, got '%s'", result)
	}
}

func TestFilterFields(t *testing.T) {
	data := map[string]interface{}{
		"name":  "test",
		"url":   "https://x.com",
		"extra": "remove me",
	}
	result := FilterFields(data, "name,url")
	filtered, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map")
	}
	if _, exists := filtered["extra"]; exists {
		t.Error("extra field should be filtered out")
	}
	if filtered["name"] != "test" {
		t.Error("name should be present")
	}
}

func TestFormatResponseNilData(t *testing.T) {
	resp := Response{ID: "1", Success: true, Data: nil}
	flags := GlobalFlags{}
	result := FormatResponse(resp, flags)
	if result != "" {
		t.Errorf("expected empty string for nil data, got '%s'", result)
	}
}
