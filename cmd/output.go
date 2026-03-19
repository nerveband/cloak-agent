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

	// Pretty-print as indented JSON.
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	return string(b)
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
