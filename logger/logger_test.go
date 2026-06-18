package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitTextHandler(t *testing.T) {
	var buf bytes.Buffer
	if err := Init(Options{
		AppName: "test-app",
		Level:   "info",
		Format:  FormatText,
		Output:  &buf,
	}); err != nil {
		t.Fatalf("init logger: %v", err)
	}

	Info("server started", "port", 8080)

	out := buf.String()
	if !strings.Contains(out, "server started") {
		t.Fatalf("expected message in output, got %q", out)
	}
	if !strings.Contains(out, "port=8080") {
		t.Fatalf("expected structured field in output, got %q", out)
	}
	if !strings.Contains(out, "app=test-app") {
		t.Fatalf("expected app field in output, got %q", out)
	}
}

func TestInitJSONHandler(t *testing.T) {
	var buf bytes.Buffer
	if err := Init(Options{
		AppName: "test-app",
		Level:   "info",
		Format:  FormatJSON,
		Output:  &buf,
	}); err != nil {
		t.Fatalf("init logger: %v", err)
	}

	Info("event", "key", "value")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("invalid json output: %v\n%q", err, buf.String())
	}

	if record["msg"] != "event" {
		t.Fatalf("got msg %v, want event", record["msg"])
	}
	if record["key"] != "value" {
		t.Fatalf("got key %v, want value", record["key"])
	}
	if record["app"] != "test-app" {
		t.Fatalf("got app %v, want test-app", record["app"])
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	if err := Init(Options{
		Level:  "warn",
		Format: FormatText,
		Output: &buf,
	}); err != nil {
		t.Fatalf("init logger: %v", err)
	}

	Info("hidden")
	Warn("visible")

	out := buf.String()
	if strings.Contains(out, "hidden") {
		t.Fatalf("info log should be filtered, got %q", out)
	}
	if !strings.Contains(out, "visible") {
		t.Fatalf("warn log should appear, got %q", out)
	}
}

func TestComponent(t *testing.T) {
	var buf bytes.Buffer
	if err := Init(Options{
		Level:  "debug",
		Format: FormatText,
		Output: &buf,
	}); err != nil {
		t.Fatalf("init logger: %v", err)
	}

	Component("database").Info("connected")

	out := buf.String()
	if !strings.Contains(out, "component=database") {
		t.Fatalf("expected component field, got %q", out)
	}
	if !strings.Contains(out, "connected") {
		t.Fatalf("expected message, got %q", out)
	}
}

func TestInitLogFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "nested", "app.log")

	if err := Init(Options{
		AppName: "test-app",
		Level:   "info",
		Format:  FormatText,
		LogFile: logPath,
	}); err != nil {
		t.Fatalf("init logger: %v", err)
	}
	t.Cleanup(func() { _ = Close() })

	Info("persisted", "id", 1)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "persisted") {
		t.Fatalf("expected message in file, got %q", out)
	}
	if !strings.Contains(out, "id=1") {
		t.Fatalf("expected structured field in file, got %q", out)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "debug", want: "DEBUG"},
		{in: "info", want: "INFO"},
		{in: "warn", want: "WARN"},
		{in: "warning", want: "WARN"},
		{in: "error", want: "ERROR"},
		{in: "unknown", want: "INFO"},
	}

	for _, tt := range tests {
		got := parseLevel(tt.in)
		if got.String() != tt.want {
			t.Fatalf("parseLevel(%q) = %s, want %s", tt.in, got.String(), tt.want)
		}
	}
}
