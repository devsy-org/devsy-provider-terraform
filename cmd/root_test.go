package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/devsy-org/devsy/pkg/log"
	"github.com/spf13/cobra"
)

const (
	testInfoLevel  = "info"
	testTraceLevel = "trace"
	testProbe      = "probe"
)

func TestProviderLoggingLevels(t *testing.T) {
	for _, tc := range []struct {
		level string
		want  string
		info  bool
		debug bool
	}{
		{"", testInfoLevel, true, false},
		{"error", "error", false, false},
		{"warn", "warn", false, false},
		{testInfoLevel, testInfoLevel, true, false},
		{"debug", "debug", true, true},
		{testTraceLevel, testTraceLevel, true, true},
		{"TrAcE", testTraceLevel, true, true},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Setenv("DEVSY_LOG_LEVEL", tc.level)
			t.Setenv("DEVSY_LOG_FORMAT", "auto")
			root := NewRootCmd()
			root.AddCommand(&cobra.Command{Use: testProbe, Run: func(*cobra.Command, []string) {
				log.Info("info message")
				log.Debug("debug message")
			}})
			output := readRootDiagnostics(t, root)
			if got := log.LevelString(); got != tc.want {
				t.Fatalf("level = %q, want %q", got, tc.want)
			}
			if bytes.Contains(output, []byte("info message")) != tc.info ||
				bytes.Contains(output, []byte("debug message")) != tc.debug {
				t.Fatalf("unexpected output: %s", output)
			}
			assertJSONDiagnostics(t, output)
		})
	}
}

func TestProviderLoggingFormats(t *testing.T) {
	for _, format := range []string{"", "auto", "json", "text", "plain", "logfmt", "AuTo", "JsOn", "PlAiN"} {
		t.Run("format="+format, func(t *testing.T) {
			t.Setenv("DEVSY_LOG_LEVEL", testInfoLevel)
			t.Setenv("DEVSY_LOG_FORMAT", format)
			t.Setenv("AWS_SECRET_ACCESS_KEY", "sensitive-canary-value")
			var protocol bytes.Buffer
			root := NewRootCmd()
			root.SetOut(&protocol)
			root.AddCommand(
				&cobra.Command{Use: testProbe, Run: func(cmd *cobra.Command, _ []string) {
					log.Info("diagnostic message sensitive-canary-value")
					_, _ = io.WriteString(cmd.OutOrStdout(), "raw protocol output\n")
				}},
			)
			output := readRootDiagnostics(t, root)
			assertProtocolSeparation(t, protocol.String(), output)
			normalized := strings.ToLower(format)
			if json.Valid(
				bytes.TrimSpace(output),
			) != (normalized == "" || normalized == "auto" || normalized == "json") {
				t.Fatalf("unexpected format %q: %s", format, output)
			}
		})
	}
}

func TestProviderLoggingRejectsInvalidConfiguration(t *testing.T) {
	for _, name := range []string{"DEVSY_LOG_LEVEL", "DEVSY_LOG_FORMAT"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("DEVSY_LOG_LEVEL", "")
			t.Setenv("DEVSY_LOG_FORMAT", "")
			t.Setenv(name, "secret-canary-invalid-value")
			ran := false
			root := NewRootCmd()
			root.AddCommand(
				&cobra.Command{Use: testProbe, Run: func(*cobra.Command, []string) { ran = true }},
			)
			root.SetArgs([]string{testProbe})
			err := root.Execute()
			if err == nil || ran || !strings.Contains(err.Error(), name) ||
				strings.Contains(err.Error(), "secret-canary") {
				t.Fatalf("error = %v, command ran = %v", err, ran)
			}
		})
	}
}

func readRootDiagnostics(t *testing.T, root *cobra.Command) []byte {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	original := os.Stderr
	os.Stderr = writer
	defer func() { os.Stderr = original }()

	root.SetArgs([]string{testProbe})
	err = root.Execute()
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	output, readErr := io.ReadAll(reader)
	if err != nil || readErr != nil {
		t.Fatalf("execute: %v, read: %v", err, readErr)
	}
	return output
}

func assertJSONDiagnostics(t *testing.T, output []byte) {
	t.Helper()
	for line := range bytes.SplitSeq(bytes.TrimSpace(output), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal(
			line,
			&record,
		); err != nil || record["ts"] == nil || record["level"] == nil ||
			record["msg"] == nil {
			t.Fatalf("invalid diagnostic %q: %v", line, err)
		}
	}
}

func assertProtocolSeparation(t *testing.T, protocol string, output []byte) {
	t.Helper()
	if protocol != "raw protocol output\n" ||
		!bytes.Contains(output, []byte("diagnostic message")) ||
		bytes.Contains(output, []byte("raw protocol output")) ||
		bytes.Contains(output, []byte("sensitive-canary-value")) {
		t.Fatalf("protocol = %q, diagnostics = %q", protocol, output)
	}
}
