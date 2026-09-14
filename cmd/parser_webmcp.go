package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func parseWebMCPArgs(args []string) (map[string]interface{}, error) {
	if len(args) == 0 || (args[0] != "list" && args[0] != "call") {
		return nil, fmt.Errorf("webmcp requires list or call")
	}
	command := map[string]interface{}{"action": "webmcp_" + args[0]}
	positional := []string{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--frame", "--timeout":
			flag := args[i]
			if i+1 >= len(args) {
				return nil, fmt.Errorf("%s requires a value", flag)
			}
			i++
			if flag == "--frame" {
				command["frame"] = args[i]
			} else {
				value, err := strconv.Atoi(args[i])
				if err != nil || value < 1 || value > 60000 {
					return nil, fmt.Errorf("WebMCP timeout must be 1-60000 milliseconds")
				}
				command["timeout"] = value
			}
		default:
			if strings.HasPrefix(args[i], "--") {
				return nil, fmt.Errorf("unknown WebMCP option: %s", args[i])
			}
			positional = append(positional, args[i])
		}
	}
	if args[0] == "list" {
		if len(positional) != 0 {
			return nil, fmt.Errorf("webmcp list does not accept positional arguments")
		}
	} else {
		if len(positional) < 1 || len(positional) > 2 {
			return nil, fmt.Errorf("webmcp call requires a tool name and optional JSON object")
		}
		command["name"] = positional[0]
		if len(positional) == 2 {
			var input map[string]interface{}
			if err := json.Unmarshal([]byte(positional[1]), &input); err != nil || input == nil {
				return nil, fmt.Errorf("WebMCP arguments must be a JSON object")
			}
			command["arguments"] = input
		}
	}
	return command, nil
}
