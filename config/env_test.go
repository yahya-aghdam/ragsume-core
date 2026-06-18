package config

import (
	"os"
	"strings"
	"testing"
)

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	old, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %q: %v", key, err)
	}

	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, old)
			return
		}
		_ = os.Unsetenv(key)
	})
}

func TestGetString(t *testing.T) {
	t.Run("returns value when set", func(t *testing.T) {
		t.Setenv("TEST_STRING", "hello")

		got, err := getString("TEST_STRING")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "hello" {
			t.Fatalf("got %q, want %q", got, "hello")
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		t.Setenv("TEST_STRING", "  spaced  ")

		got, err := getString("TEST_STRING")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "spaced" {
			t.Fatalf("got %q, want %q", got, "spaced")
		}
	})

	t.Run("errors when missing", func(t *testing.T) {
		unsetEnv(t, "TEST_STRING")

		_, err := getString("TEST_STRING")
		if err == nil {
			t.Fatal("expected error for missing variable")
		}
		if !strings.Contains(err.Error(), `required environment variable "TEST_STRING" is not set`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("errors when empty", func(t *testing.T) {
		t.Setenv("TEST_STRING", "")

		_, err := getString("TEST_STRING")
		if err == nil {
			t.Fatal("expected error for empty variable")
		}
	})
}

func TestGetInt(t *testing.T) {
	t.Run("parses valid integer", func(t *testing.T) {
		t.Setenv("TEST_INT", "42")

		got, err := getInt("TEST_INT")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 42 {
			t.Fatalf("got %d, want %d", got, 42)
		}
	})

	t.Run("parses zero", func(t *testing.T) {
		t.Setenv("TEST_INT", "0")

		got, err := getInt("TEST_INT")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 0 {
			t.Fatalf("got %d, want %d", got, 0)
		}
	})

	t.Run("errors when missing", func(t *testing.T) {
		unsetEnv(t, "TEST_INT")

		_, err := getInt("TEST_INT")
		if err == nil {
			t.Fatal("expected error for missing variable")
		}
	})

	t.Run("errors on invalid integer", func(t *testing.T) {
		t.Setenv("TEST_INT", "not-a-number")

		_, err := getInt("TEST_INT")
		if err == nil {
			t.Fatal("expected error for invalid integer")
		}
		if !strings.Contains(err.Error(), `must be an integer`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestGetFloat(t *testing.T) {
	t.Run("parses valid float", func(t *testing.T) {
		t.Setenv("TEST_FLOAT", "3.14")

		got, err := getFloat("TEST_FLOAT")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 3.14 {
			t.Fatalf("got %v, want %v", got, 3.14)
		}
	})

	t.Run("errors when missing", func(t *testing.T) {
		unsetEnv(t, "TEST_FLOAT")

		_, err := getFloat("TEST_FLOAT")
		if err == nil {
			t.Fatal("expected error for missing variable")
		}
	})

	t.Run("errors on invalid float", func(t *testing.T) {
		t.Setenv("TEST_FLOAT", "abc")

		_, err := getFloat("TEST_FLOAT")
		if err == nil {
			t.Fatal("expected error for invalid float")
		}
		if !strings.Contains(err.Error(), `must be a float`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "true", value: "true", want: true},
		{name: "false", value: "false", want: false},
		{name: "1", value: "1", want: true},
		{name: "0", value: "0", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TEST_BOOL", tt.value)

			got, err := getBool("TEST_BOOL")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("errors when missing", func(t *testing.T) {
		unsetEnv(t, "TEST_BOOL")

		_, err := getBool("TEST_BOOL")
		if err == nil {
			t.Fatal("expected error for missing variable")
		}
	})

	t.Run("errors on invalid boolean", func(t *testing.T) {
		t.Setenv("TEST_BOOL", "maybe")

		_, err := getBool("TEST_BOOL")
		if err == nil {
			t.Fatal("expected error for invalid boolean")
		}
		if !strings.Contains(err.Error(), `must be a boolean`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
