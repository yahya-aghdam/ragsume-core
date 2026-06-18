package main

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type Project struct {
	ProjectName string   `yaml:"project_name"`
	Category    string   `yaml:"category"`
	DateRange   string   `yaml:"date_range"`
	TechStack   []string `yaml:"tech_stack"`
	Problem     string   `yaml:"problem"`
	Decisions   string   `yaml:"decisions"`
	Tradeoffs   string   `yaml:"tradeoffs"`
	Outcome     string   `yaml:"outcome"`
}

type projectChunk struct {
	Section string
	Text    string
}

func parseProject(data []byte) (Project, error) {
	var p Project
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Project{}, fmt.Errorf("parse project yaml: %w", err)
	}
	if strings.TrimSpace(p.ProjectName) == "" {
		return Project{}, fmt.Errorf("project_name is required")
	}
	return p, nil
}

func chunkProject(p Project) []projectChunk {
	sections := []struct {
		name string
		text string
	}{
		{"problem", p.Problem},
		{"decisions", p.Decisions},
		{"outcome", p.Outcome},
	}

	out := make([]projectChunk, 0, 3)
	for _, s := range sections {
		text := strings.TrimSpace(s.text)
		if text == "" {
			continue
		}
		out = append(out, projectChunk{Section: s.name, Text: text})
	}
	return out
}

func normalizeTechStack(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		v := strings.ToLower(strings.TrimSpace(item))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func pointID(projectName, section string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(projectName+":"+section)).String()
}
