package cmd

// BuildSchemaCommand constructs the command map for the "schema" subcommand.
//
//   - schema (no args), schema --all, schema --list  -> {"action":"schema","all":true}
//   - schema <command>                                -> {"action":"schema","command":<command>}
func BuildSchemaCommand(args []string) map[string]interface{} {
	if len(args) == 0 {
		return map[string]interface{}{
			"action": "schema",
			"all":    true,
		}
	}

	// Check for flags.
	for _, a := range args {
		if a == "--all" || a == "--list" {
			return map[string]interface{}{
				"action": "schema",
				"all":    true,
			}
		}
	}

	// First non-flag argument is the command name.
	return map[string]interface{}{
		"action":  "schema",
		"command": args[0],
	}
}
