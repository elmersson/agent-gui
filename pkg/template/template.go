package template

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Template represents an agent template definition
type Template struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Model       string            `yaml:"model"`
	System      string            `yaml:"system"`
	Limits      Limits            `yaml:"limits"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

// Limits defines resource limits for an agent
type Limits struct {
	MaxTokens      int     `yaml:"max_tokens"`
	MaxCostUSD     float64 `yaml:"max_cost_usd"`
	MaxDurationSec int     `yaml:"max_duration_sec"`
}

// Loader handles loading and validating agent templates
type Loader struct {
	templatesDir string
	templates    map[string]*Template
}

// NewLoader creates a new template loader
func NewLoader(templatesDir string) *Loader {
	return &Loader{
		templatesDir: templatesDir,
		templates:    make(map[string]*Template),
	}
}

// Load loads all templates from the templates directory
func (l *Loader) Load() error {
	if err := os.MkdirAll(l.templatesDir, 0755); err != nil {
		return fmt.Errorf("failed to create templates directory: %w", err)
	}

	entries, err := os.ReadDir(l.templatesDir)
	if err != nil {
		return fmt.Errorf("failed to read templates directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".yaml" && filepath.Ext(entry.Name()) != ".yml" {
			continue
		}

		templatePath := filepath.Join(l.templatesDir, entry.Name())
		data, err := os.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template file %s: %w", entry.Name(), err)
		}

		var tmpl Template
		if err := yaml.Unmarshal(data, &tmpl); err != nil {
			return fmt.Errorf("failed to parse template file %s: %w", entry.Name(), err)
		}

		if err := l.validate(&tmpl); err != nil {
			return fmt.Errorf("invalid template %s: %w", entry.Name(), err)
		}

		l.templates[tmpl.Name] = &tmpl
	}

	return nil
}

// validate checks if a template is valid
func (l *Loader) validate(tmpl *Template) error {
	if tmpl.Name == "" {
		return fmt.Errorf("template name is required")
	}

	if tmpl.Model == "" {
		return fmt.Errorf("model is required")
	}

	if tmpl.System == "" {
		return fmt.Errorf("system prompt is required")
	}

	if tmpl.Limits.MaxTokens < 0 {
		return fmt.Errorf("max_tokens must be non-negative")
	}

	if tmpl.Limits.MaxCostUSD < 0 {
		return fmt.Errorf("max_cost_usd must be non-negative")
	}

	if tmpl.Limits.MaxDurationSec < 0 {
		return fmt.Errorf("max_duration_sec must be non-negative")
	}

	return nil
}

// GetTemplate returns a template by name
func (l *Loader) GetTemplate(name string) (*Template, error) {
	tmpl, ok := l.templates[name]
	if !ok {
		return nil, fmt.Errorf("template '%s' not found", name)
	}
	return tmpl, nil
}

// ListTemplates returns all available template names
func (l *Loader) ListTemplates() []string {
	names := make([]string, 0, len(l.templates))
	for name := range l.templates {
		names = append(names, name)
	}
	return names
}

// GetTemplateDetails returns details for all templates
func (l *Loader) GetTemplateDetails() []*Template {
	templates := make([]*Template, 0, len(l.templates))
	for _, tmpl := range l.templates {
		templates = append(templates, tmpl)
	}
	return templates
}
