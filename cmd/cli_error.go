package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	ExitValidation = 64
	ExitDaemon     = 69
	ExitTimeout    = 70
	ExitInternal   = 1
)

type CLIError struct {
	Code     string      `json:"code"`
	Message  string      `json:"message"`
	Hint     string      `json:"hint,omitempty"`
	Retry    bool        `json:"retryable"`
	Details  interface{} `json:"details,omitempty"`
	ExitCode int         `json:"-"`
}

func (e *CLIError) Error() string {
	if e.Hint == "" {
		return e.Message
	}
	return e.Message + " Hint: " + e.Hint
}

func NewCLIError(code, message, hint string, retry bool, exitCode int) *CLIError {
	return &CLIError{Code: code, Message: message, Hint: hint, Retry: retry, ExitCode: exitCode}
}

func WrapCLIError(err error) *CLIError {
	if err == nil {
		return nil
	}
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return cliErr
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no command provided"),
		strings.Contains(msg, "requires"),
		strings.Contains(msg, "invalid JSON"),
		strings.Contains(msg, "unknown command"):
		return NewCLIError("validation_error", msg, "Run 'cloak-agent --help' or 'cloak-agent schema' for valid command shapes.", false, ExitValidation)
	case strings.Contains(msg, "timed out"),
		strings.Contains(msg, "timeout"):
		return NewCLIError("timeout", msg, "Retry with --timeout <ms>, or run 'cloak-agent daemon restart' if the daemon is stuck.", true, ExitTimeout)
	case strings.Contains(msg, "failed to send command"),
		strings.Contains(msg, "daemon"):
		return NewCLIError("daemon_error", msg, "Run 'cloak-agent daemon status' or 'cloak-agent daemon restart'.", true, ExitDaemon)
	default:
		return NewCLIError("internal_error", msg, "Run with --output json for structured error details.", false, ExitInternal)
	}
}

func WantsJSON(args []string) bool {
	for i, arg := range args {
		if arg == "--json" {
			return true
		}
		if arg == "--output" && i+1 < len(args) && args[i+1] == "json" {
			return true
		}
		if arg == "--output" && i+1 < len(args) && args[i+1] == "human" {
			return false
		}
	}
	info, err := os.Stdout.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) == 0
}

func PrintCLIError(err error, args []string) int {
	cliErr := WrapCLIError(err)
	if cliErr == nil {
		return 0
	}
	if WantsJSON(args) {
		payload := map[string]interface{}{
			"ok":      false,
			"success": false,
			"error":   cliErr,
		}
		b, marshalErr := json.Marshal(payload)
		if marshalErr == nil {
			fmt.Fprintln(os.Stderr, string(b))
			return cliErr.ExitCode
		}
	}
	fmt.Fprintf(os.Stderr, "Error [%s]: %s\n", cliErr.Code, cliErr.Message)
	if cliErr.Hint != "" {
		fmt.Fprintf(os.Stderr, "Hint: %s\n", cliErr.Hint)
	}
	return cliErr.ExitCode
}
