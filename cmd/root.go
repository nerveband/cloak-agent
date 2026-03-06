package cmd

import "fmt"

func Execute(args []string) error {
	if len(args) == 0 {
		fmt.Println("cloak-agent - stealth browser automation CLI for AI agents")
		fmt.Println("Usage: cloak-agent <command> [args...]")
		return nil
	}
	return fmt.Errorf("unknown command: %s", args[0])
}
