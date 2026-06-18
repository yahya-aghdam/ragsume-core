package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Profile is the resume summary loaded into the system prompt.
type Profile struct {
	Name     string            `yaml:"name"`
	Headline string            `yaml:"headline"`
	Summary  string            `yaml:"summary"`
	Skills   []string          `yaml:"skills"`
	Contact  map[string]string `yaml:"contact"`
}

// LoadProfile reads and parses the profile YAML file.
func LoadProfile(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read profile: %w", err)
	}

	var profile Profile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return Profile{}, fmt.Errorf("parse profile: %w", err)
	}
	if strings.TrimSpace(profile.Name) == "" {
		return Profile{}, fmt.Errorf("profile name is required")
	}
	return profile, nil
}

// RenderProfileYAML returns the profile as YAML text for the system prompt.
func RenderProfileYAML(profile Profile) (string, error) {
	data, err := yaml.Marshal(profile)
	if err != nil {
		return "", fmt.Errorf("marshal profile: %w", err)
	}
	return string(data), nil
}
