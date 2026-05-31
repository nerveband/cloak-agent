package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func parseLaunchArgs(rest []string) (map[string]interface{}, error) {
	m := map[string]interface{}{"action": "launch"}

	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--headed":
			m["headless"] = false
		case "--profile":
			if i+1 < len(rest) {
				m["profile"] = rest[i+1]
				i++
			}
		case "--proxy":
			if i+1 < len(rest) {
				m["proxy"] = rest[i+1]
				i++
			}
		case "--timezone":
			if i+1 < len(rest) {
				m["timezone"] = rest[i+1]
				i++
			}
		case "--locale":
			if i+1 < len(rest) {
				m["locale"] = rest[i+1]
				i++
			}
		case "--geoip":
			m["geoip"] = true
		case "--humanize":
			m["humanize"] = true
		case "--human-preset":
			if i+1 < len(rest) {
				m["humanPreset"] = rest[i+1]
				i++
			}
		case "--human-config":
			if i+1 < len(rest) {
				var cfg map[string]interface{}
				if err := json.Unmarshal([]byte(rest[i+1]), &cfg); err != nil {
					return nil, fmt.Errorf("--human-config requires a JSON object: %w", err)
				}
				m["humanConfig"] = cfg
				i++
			}
		case "--fingerprint-seed":
			if i+1 < len(rest) {
				if n, err := strconv.Atoi(rest[i+1]); err == nil {
					m["fingerprintSeed"] = n
				}
				i++
			}
		case "--platform":
			if i+1 < len(rest) {
				m["platform"] = rest[i+1]
				i++
			}
		case "--gpu-vendor":
			if i+1 < len(rest) {
				m["gpuVendor"] = rest[i+1]
				i++
			}
		case "--gpu-renderer":
			if i+1 < len(rest) {
				m["gpuRenderer"] = rest[i+1]
				i++
			}
		case "--user-agent":
			if i+1 < len(rest) {
				m["userAgent"] = rest[i+1]
				i++
			}
		case "--executable-path":
			if i+1 < len(rest) {
				m["executablePath"] = rest[i+1]
				i++
			}
		case "--storage-state":
			if i+1 < len(rest) {
				m["storageState"] = rest[i+1]
				i++
			}
		case "--ignore-https-errors":
			m["ignoreHTTPSErrors"] = true
		case "--context-options":
			if i+1 < len(rest) {
				var opts map[string]interface{}
				if err := json.Unmarshal([]byte(rest[i+1]), &opts); err != nil {
					return nil, fmt.Errorf("--context-options requires a JSON object: %w", err)
				}
				m["contextOptions"] = opts
				i++
			}
		case "--viewport":
			if i+1 < len(rest) {
				addViewport(m, rest[i+1])
				i++
			}
		case "--arg":
			if i+1 < len(rest) {
				addLaunchArg(m, rest[i+1])
				i++
			}
		default:
			if !strings.HasPrefix(rest[i], "--") {
				m["url"] = rest[i]
			}
		}
	}

	return m, nil
}

func addViewport(m map[string]interface{}, value string) {
	parts := strings.Split(strings.ToLower(value), "x")
	if len(parts) != 2 {
		return
	}
	w, wErr := strconv.Atoi(parts[0])
	h, hErr := strconv.Atoi(parts[1])
	if wErr == nil && hErr == nil {
		m["viewport"] = map[string]interface{}{"width": w, "height": h}
	}
}

func addLaunchArg(m map[string]interface{}, arg string) {
	if existing, ok := m["args"].([]string); ok {
		m["args"] = append(existing, arg)
		return
	}
	m["args"] = []string{arg}
}
