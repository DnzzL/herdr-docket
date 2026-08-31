// Package fleet reads the fleet directory: where the platform backlog lives
// and who the agents are. Agents are folders of AGENT.md files — a YAML
// frontmatter for the run parameters, a markdown body for the persona.
package fleet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Agent is one named worker: the parameters a run needs plus the persona the
// prompt opens with.
type Agent struct {
	Name           string
	Model          string   `yaml:"model"`
	Workdir        string   `yaml:"workdir"`
	Workspace      string   `yaml:"workspace"` // root | worktree
	Kind           string   `yaml:"agent"`     // herdr agent kind, default claude
	MCPConfig      string   `yaml:"mcp_config"`
	AgentArgs      []string `yaml:"agent_args"`
	TimeoutMinutes int      `yaml:"timeout_minutes"`
	Persona        string   `yaml:"-"`
}

// Diagnostic is one AGENT.md that did not load, and why. The rest of the
// fleet still works: a typo in one agent must not ground the others.
type Diagnostic struct {
	Agent   string
	Message string
}

func (d Diagnostic) String() string { return d.Agent + ": " + d.Message }

// LoadAgents reads every agents/<name>/AGENT.md under dir. A missing agents
// directory is an empty fleet, not an error.
func LoadAgents(dir string) (map[string]Agent, []Diagnostic) {
	entries, err := os.ReadDir(filepath.Join(dir, "agents"))
	if err != nil {
		return nil, nil
	}
	agents := map[string]Agent{}
	var diags []Diagnostic
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		a, err := loadAgent(filepath.Join(dir, "agents", name, "AGENT.md"), name)
		if err != nil {
			diags = append(diags, Diagnostic{Agent: name, Message: err.Error()})
			continue
		}
		agents[name] = a
	}
	return agents, diags
}

func loadAgent(path, name string) (Agent, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Agent{}, err
	}
	front, body, err := splitFrontmatter(string(raw))
	if err != nil {
		return Agent{}, err
	}
	a := Agent{Name: name}
	if err := yaml.Unmarshal([]byte(front), &a); err != nil {
		return Agent{}, fmt.Errorf("frontmatter: %v", err)
	}
	a.Persona = strings.TrimSpace(body)
	if a.Workdir == "" {
		return Agent{}, fmt.Errorf("frontmatter has no workdir")
	}
	a.Workdir = expandHome(a.Workdir)
	if a.Kind == "" {
		a.Kind = "claude"
	}
	if a.Workspace == "" {
		a.Workspace = "worktree"
	}
	if a.Workspace != "root" && a.Workspace != "worktree" {
		return Agent{}, fmt.Errorf("unknown workspace mode %q", a.Workspace)
	}
	if a.TimeoutMinutes <= 0 {
		a.TimeoutMinutes = 60
	}
	return a, nil
}

// splitFrontmatter separates the YAML header from the markdown body.
func splitFrontmatter(s string) (front, body string, err error) {
	if !strings.HasPrefix(s, "---\n") {
		return "", "", fmt.Errorf("no frontmatter (file must start with ---)")
	}
	rest := s[4:]
	i := strings.Index(rest, "\n---")
	if i < 0 {
		return "", "", fmt.Errorf("unterminated frontmatter")
	}
	return rest[:i], strings.TrimPrefix(rest[i+4:], "\n"), nil
}

// expandHome resolves a leading ~ against the user's home directory.
func expandHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
