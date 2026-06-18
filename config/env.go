package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func getString(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}
	return value, nil
}

func getInt(key string) (int, error) {
	value, err := getString(key)
	if err != nil {
		return 0, err
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("environment variable %q must be an integer, got %q", key, value)
	}

	return parsed, nil
}

func getFloat(key string) (float64, error) {
	value, err := getString(key)
	if err != nil {
		return 0, err
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("environment variable %q must be a float, got %q", key, value)
	}

	return parsed, nil
}

func getOptionalString(key string) (string, bool) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", false
	}
	return value, true
}

func getBool(key string) (bool, error) {
	value, err := getString(key)
	if err != nil {
		return false, err
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("environment variable %q must be a boolean, got %q", key, value)
	}

	return parsed, nil
}
