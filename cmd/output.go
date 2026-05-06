package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// FormatResponse converts a daemon Response into a human-readable (or
// machine-readable) string based on the active GlobalFlags.
func FormatResponse(resp Response, flags GlobalFlags) string {
	// Raw JSON mode: marshal the entire response.
	if flags.JSONOutput {
		if !resp.IsSuccess() {
			payload := map[string]interface{}{
				"id":      resp.ID,
				"ok":      false,
				"success": false,
				"error": map[string]interface{}{
					"code":      "command_failed",
					"message":   resp.Error,
					"hint":      "Inspect the command response, re-snapshot stale refs, or run with --dry-run before mutating actions.",
					"retryable": false,
				},
			}
			b, err := json.Marshal(payload)
			if err != nil {
				return fmt.Sprintf(`{"error":{"code":"marshal_error","message":"%s"}}`, err)
			}
			return string(b)
		}
		if flags.Fields != "" || flags.Limit > 0 || flags.CountOnly || flags.IDOnly {
			filtered := resp
			data := filtered.Data
			if flags.Fields != "" {
				data = FilterFields(data, flags.Fields)
			}
			filtered.Data = ApplyContextControls(data, flags)
			b, err := json.Marshal(filtered)
			if err != nil {
				return fmt.Sprintf(`{"error":{"code":"marshal_error","message":"%s"}}`, err)
			}
			return string(b)
		}
		b, err := json.Marshal(resp)
		if err != nil {
			return fmt.Sprintf(`{"error":"failed to marshal response: %s"}`, err)
		}
		return string(b)
	}

	// Error response.
	if !resp.IsSuccess() {
		return "Error: " + resp.Error
	}

	// Nil data — success with no output.
	if resp.Data == nil {
		return ""
	}

	// String data — return directly.
	if s, ok := resp.Data.(string); ok {
		return s
	}

	// Field filtering.
	data := resp.Data
	if flags.Fields != "" {
		data = FilterFields(data, flags.Fields)
	}
	data = ApplyContextControls(data, flags)

	// Pretty-print as indented JSON.
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	return string(b)
}

func ApplyContextControls(data interface{}, flags GlobalFlags) interface{} {
	if flags.CountOnly {
		switch v := data.(type) {
		case []interface{}:
			return map[string]interface{}{"count": len(v)}
		case map[string]interface{}:
			for _, key := range []string{"sessions", "profiles", "requests", "cookies"} {
				if arr, ok := v[key].([]interface{}); ok {
					return map[string]interface{}{"count": len(arr)}
				}
			}
		}
	}
	if flags.Limit <= 0 && !flags.IDOnly {
		return data
	}
	if arr, ok := data.([]interface{}); ok {
		return trimArray(minimalArray(arr, flags.IDOnly), flags.Limit)
	}
	if m, ok := data.(map[string]interface{}); ok {
		copy := make(map[string]interface{}, len(m))
		for k, v := range m {
			if arr, ok := v.([]interface{}); ok {
				copy[k] = trimArray(minimalArray(arr, flags.IDOnly), flags.Limit)
				continue
			}
			copy[k] = v
		}
		return copy
	}
	return data
}

func minimalArray(arr []interface{}, idOnly bool) []interface{} {
	if !idOnly {
		return arr
	}
	out := make([]interface{}, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			for _, key := range []string{"id", "ref", "name", "session", "url"} {
				if v, exists := m[key]; exists {
					out = append(out, v)
					goto next
				}
			}
		}
		out = append(out, item)
	next:
	}
	return out
}

func trimArray(arr []interface{}, limit int) []interface{} {
	if limit <= 0 || len(arr) <= limit {
		return arr
	}
	return arr[:limit]
}

// FilterFields reduces a map to only the comma-separated field names listed in
// the fields string. If data is not a map[string]interface{}, it is returned
// as-is.
func FilterFields(data interface{}, fields string) interface{} {
	m, ok := data.(map[string]interface{})
	if !ok {
		return data
	}

	names := strings.Split(fields, ",")
	wanted := make(map[string]bool, len(names))
	for _, n := range names {
		wanted[strings.TrimSpace(n)] = true
	}

	filtered := make(map[string]interface{})
	for k, v := range m {
		if wanted[k] {
			filtered[k] = v
		}
	}
	return filtered
}

// PrintResponse writes the formatted response to stdout (on success) or
// stderr (on error).
func PrintResponse(resp Response, flags GlobalFlags) {
	out := FormatResponse(resp, flags)
	if out == "" {
		return
	}

	if !resp.IsSuccess() && !flags.JSONOutput {
		fmt.Fprintln(os.Stderr, out)
	} else {
		fmt.Println(out)
	}
}
