// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package log

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	fatalTestEnv    = "NCMCTL_LOG_FATAL_TEST"
	fatalLogFileEnv = "NCMCTL_LOG_FATAL_LOG_FILE"
)

func newTestLogger(t *testing.T, level string) (*Logger, string) {
	t.Helper()

	filename := filepath.Join(t.TempDir(), "ncmctl.log")
	previous := Default
	logger := New(&Config{
		App:    "test",
		Format: "json",
		Level:  level,
		Rotate: lumberjack.Logger{Filename: filename},
	})
	Default = logger

	t.Cleanup(func() {
		Default = previous

		if err := logger.Close(); err != nil {
			t.Errorf("close test logger: %v", err)
		}
	})

	return logger, filename
}

func readTestLog(t *testing.T, logger *Logger, filename string) string {
	t.Helper()

	if err := logger.Close(); err != nil {
		t.Fatalf("close test logger: %v", err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read test log: %v", err)
	}
	return string(data)
}

func TestLogFunctionsRespectConfiguredLevel(t *testing.T) {
	logger, filename := newTestLogger(t, "info")

	Debugf("debug message")
	DebugW("debug structured", "kind", "debug")
	Infof("info message: %s", "chaunsin")
	InfoW("info structured", "kind", "info")
	Warnf("warn message")
	WarnW("warn structured", "kind", "warn")
	Errorf("error message")
	ErrorW("error structured", "kind", "error")

	output := readTestLog(t, logger, filename)
	for _, unexpected := range []string{"debug message", "debug structured"} {
		if strings.Contains(output, unexpected) {
			t.Fatalf("log output contains filtered message %q: %s", unexpected, output)
		}
	}

	for _, expected := range []string{
		"info message: chaunsin",
		"info structured",
		`"kind":"info"`,
		"warn message",
		"warn structured",
		`"kind":"warn"`,
		"error message",
		"error structured",
		`"kind":"error"`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("log output does not contain %q: %s", expected, output)
		}
	}
}

func TestSetLevel(t *testing.T) {
	logger, filename := newTestLogger(t, "warn")

	Infof("hidden before level update")
	logger.SetLevel(slog.LevelInfo)
	Infof("visible after level update")

	output := readTestLog(t, logger, filename)
	if strings.Contains(output, "hidden before level update") {
		t.Fatalf("log output contains a message below the configured level: %s", output)
	}

	if !strings.Contains(output, "visible after level update") {
		t.Fatalf("log output does not contain the message after SetLevel: %s", output)
	}
}

func TestFatalExits(t *testing.T) {
	if mode := os.Getenv(fatalTestEnv); mode != "" {
		filename := os.Getenv(fatalLogFileEnv)
		if filename == "" {
			os.Exit(3)
		}

		Default = New(&Config{
			App:    "test",
			Format: "json",
			Level:  "debug",
			Rotate: lumberjack.Logger{Filename: filename},
		})

		switch mode {
		case "formatted":
			Fatalf("fatal message: %s", mode)
		case "structured":
			FatalW("fatal message", "kind", mode)
		default:
			os.Exit(3)
		}

		os.Exit(2) // Reached only if Fatal unexpectedly returns.
	}

	for _, mode := range []string{"formatted", "structured"} {
		t.Run(mode, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "fatal.log")
			cmd := exec.Command(os.Args[0], "-test.run=^TestFatalExits$") //nolint:gosec // The test intentionally re-executes its own fixed binary path.

			cmd.Env = append(
				os.Environ(),
				fatalTestEnv+"="+mode,
				fatalLogFileEnv+"="+filename,
			)

			err := cmd.Run()

			exitErr := &exec.ExitError{}

			ok := errors.As(err, &exitErr)
			if !ok {
				t.Fatalf("Fatal exit error = %v, want exit code 1", err)
			}

			if code := exitErr.ExitCode(); code != 1 {
				t.Fatalf("Fatal exit code = %d, want 1", code)
			}

			output, err := os.ReadFile(filename)
			if err != nil {
				t.Fatalf("read Fatal log: %v", err)
			}

			if !strings.Contains(string(output), "fatal message") {
				t.Fatalf("Fatal did not write its message: %s", output)
			}

			if mode == "structured" && !strings.Contains(string(output), `"kind":"structured"`) {
				t.Fatalf("FatalW did not write its attributes: %s", output)
			}
		})
	}
}
