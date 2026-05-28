package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/vanducng/miu-db/internal/config"
)

const apiVersion = "miudb.cli/v1"

type Envelope struct {
	OK         bool           `json:"ok"`
	APIVersion string         `json:"api_version"`
	Kind       string         `json:"kind"`
	Command    string         `json:"command"`
	RequestID  string         `json:"request_id"`
	Summary    map[string]any `json:"summary,omitempty"`
	Data       any            `json:"data,omitempty"`
	Page       map[string]any `json:"page,omitempty"`
	Stats      map[string]any `json:"stats,omitempty"`
	Artifacts  []any          `json:"artifacts"`
	Warnings   []any          `json:"warnings"`
	Error      *ErrorInfo     `json:"error,omitempty"`
}

type ErrorInfo struct {
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	Hint        string         `json:"hint,omitempty"`
	Retryable   bool           `json:"retryable"`
	SafeToRetry bool           `json:"safe_to_retry,omitempty"`
	Details     map[string]any `json:"details,omitempty"`
}

type CLIError struct {
	Code      string
	Message   string
	Hint      string
	Exit      int
	Details   map[string]any
	Retry     bool
	SafeRetry bool
}

func (e *CLIError) Error() string { return e.Message }

func newRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

func writeJSON(w io.Writer, env Envelope) error {
	env.APIVersion = apiVersion
	if env.RequestID == "" {
		env.RequestID = newRequestID()
	}
	if env.Artifacts == nil {
		env.Artifacts = []any{}
	}
	if env.Warnings == nil {
		env.Warnings = []any{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(env)
}

func writeSuccess(w io.Writer, command, kind string, data any, summary map[string]any) error {
	return writeJSON(w, Envelope{
		OK:        true,
		Kind:      kind,
		Command:   command,
		Summary:   summary,
		Data:      data,
		Artifacts: []any{},
		Warnings:  []any{},
	})
}

func writeError(w io.Writer, command string, err error) error {
	info := ErrorInfo{
		Code:      "internal.error",
		Message:   config.RedactString(err.Error()),
		Retryable: false,
	}
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		info.Code = cliErr.Code
		info.Message = config.RedactString(cliErr.Message)
		info.Hint = config.RedactString(cliErr.Hint)
		info.Details = redactDetails(cliErr.Details)
		info.Retryable = cliErr.Retry
		info.SafeToRetry = cliErr.SafeRetry
	}
	return writeJSON(w, Envelope{
		OK:        false,
		Kind:      "error",
		Command:   command,
		Error:     &info,
		Artifacts: []any{},
		Warnings:  []any{},
	})
}

func redactDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return details
	}
	out := make(map[string]any, len(details))
	for key, value := range details {
		switch typed := value.(type) {
		case string:
			out[key] = config.RedactString(typed)
		default:
			out[key] = value
		}
	}
	return out
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var cliErr *CLIError
	if errors.As(err, &cliErr) && cliErr.Exit != 0 {
		return cliErr.Exit
	}
	return 1
}
