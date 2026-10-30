package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

// mcpRegistryLabelProblems returns why the MCP Registry would refuse
// server.json's image (D56): it lists the server only when every image carries
// the label io.modelcontextprotocol.server.name equal to server.json's name,
// and the images are .goreleaser.yaml's dockers: entries. Empty means listable.
func mcpRegistryLabelProblems(serverJSON, goreleaser []byte) []string {
	var server struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(serverJSON, &server); err != nil {
		return []string{fmt.Sprintf("server.json: %v", err)}
	}
	if server.Name == "" {
		return []string{"server.json: no name"}
	}

	var release struct {
		Dockers []struct {
			ImageTemplates     []string `yaml:"image_templates"`
			BuildFlagTemplates []string `yaml:"build_flag_templates"`
		} `yaml:"dockers"`
	}
	if err := yaml.Unmarshal(goreleaser, &release); err != nil {
		return []string{fmt.Sprintf(".goreleaser.yaml: %v", err)}
	}
	if len(release.Dockers) == 0 {
		return []string{".goreleaser.yaml: no dockers: entry"}
	}

	label := "--label=io.modelcontextprotocol.server.name=" + server.Name
	var problems []string
	for i, d := range release.Dockers {
		if !slices.Contains(d.BuildFlagTemplates, label) {
			problems = append(problems, fmt.Sprintf(".goreleaser.yaml dockers[%d] %v lacks %s", i, d.ImageTemplates, label))
		}
	}
	return problems
}

// TestMCPRegistry_ImageLabelMatchesServerJSON: every image the release builds
// carries server.json's name, or the registry publish fails after the release
// is out.
func TestMCPRegistry_ImageLabelMatchesServerJSON(t *testing.T) {
	t.Parallel()
	serverJSON, err := os.ReadFile("../../server.json")
	if err != nil {
		t.Fatal(err)
	}
	goreleaser, err := os.ReadFile("../../.goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range mcpRegistryLabelProblems(serverJSON, goreleaser) {
		t.Error(p)
	}
}

func TestMCPRegistryLabelProblems(t *testing.T) {
	t.Parallel()

	const name = `{"name": "io.github.example/doc"}`
	const labelled = `"--label=io.modelcontextprotocol.server.name=io.github.example/doc"`
	tests := []struct {
		name, serverJSON, goreleaser string
		want                         int
	}{
		{"every entry labelled", name, "dockers:\n  - build_flag_templates: [" + labelled + "]\n  - build_flag_templates: [" + labelled + "]\n", 0},
		{"invalid server.json", `{"name": `, "dockers: []\n", 1},
		{"no name", `{}`, "dockers:\n  - build_flag_templates: [" + labelled + "]\n", 1},
		{"no dockers entry", name, "builds: []\n", 1},
		{"one entry unlabelled", name, "dockers:\n  - build_flag_templates: [" + labelled + "]\n  - build_flag_templates: [\"--pull\"]\n", 1},
		{"another name", `{"name": "io.github.example/other"}`, "dockers:\n  - build_flag_templates: [" + labelled + "]\n", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := mcpRegistryLabelProblems([]byte(tt.serverJSON), []byte(tt.goreleaser)); len(got) != tt.want {
				t.Errorf("problems = %q, want %d", got, tt.want)
			}
		})
	}
}
