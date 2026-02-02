package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_Load(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	t.Run("empty directory", func(t *testing.T) {
		err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		templates := loader.ListTemplates()
		if len(templates) != 0 {
			t.Errorf("expected 0 templates, got %d", len(templates))
		}
	})

	t.Run("valid template", func(t *testing.T) {
		templateContent := `name: test-agent
description: A test agent
model: claude-sonnet-4-20250514
system: You are a helpful assistant.
limits:
  max_tokens: 1000
  max_cost_usd: 1.0
  max_duration_sec: 60
`
		templatePath := filepath.Join(tmpDir, "test-agent.yaml")
		if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		if err := loader.Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		templates := loader.ListTemplates()
		if len(templates) != 1 {
			t.Errorf("expected 1 template, got %d", len(templates))
		}

		if templates[0] != "test-agent" {
			t.Errorf("expected template name 'test-agent', got '%s'", templates[0])
		}
	})

	t.Run("invalid template - missing name", func(t *testing.T) {
		templateContent := `description: A test agent
model: claude-sonnet-4-20250514
system: You are a helpful assistant.
limits:
  max_tokens: 1000
  max_cost_usd: 1.0
  max_duration_sec: 60
`
		templatePath := filepath.Join(tmpDir, "invalid.yaml")
		if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		err := loader.Load()
		if err == nil {
			t.Error("expected error for invalid template, got nil")
		}
	})

	t.Run("invalid template - negative limits", func(t *testing.T) {
		templateContent := `name: test-agent-negative
description: A test agent
model: claude-sonnet-4-20250514
system: You are a helpful assistant.
limits:
  max_tokens: -100
  max_cost_usd: 1.0
  max_duration_sec: 60
`
		templatePath := filepath.Join(tmpDir, "negative.yaml")
		if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		err := loader.Load()
		if err == nil {
			t.Error("expected error for negative limits, got nil")
		}
	})
}

func TestLoader_GetTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	templateContent := `name: test-agent
description: A test agent
model: claude-sonnet-4-20250514
system: You are a helpful assistant.
limits:
  max_tokens: 1000
  max_cost_usd: 1.0
  max_duration_sec: 60
metadata:
  category: test
`
	templatePath := filepath.Join(tmpDir, "test-agent.yaml")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	t.Run("existing template", func(t *testing.T) {
		tmpl, err := loader.GetTemplate("test-agent")
		if err != nil {
			t.Fatalf("GetTemplate() error = %v", err)
		}

		if tmpl.Name != "test-agent" {
			t.Errorf("expected name 'test-agent', got '%s'", tmpl.Name)
		}

		if tmpl.Model != "claude-sonnet-4-20250514" {
			t.Errorf("expected model 'claude-sonnet-4-20250514', got '%s'", tmpl.Model)
		}

		if tmpl.Limits.MaxTokens != 1000 {
			t.Errorf("expected MaxTokens 1000, got %d", tmpl.Limits.MaxTokens)
		}

		if tmpl.Metadata["category"] != "test" {
			t.Errorf("expected metadata category 'test', got '%s'", tmpl.Metadata["category"])
		}
	})

	t.Run("non-existent template", func(t *testing.T) {
		_, err := loader.GetTemplate("non-existent")
		if err == nil {
			t.Error("expected error for non-existent template, got nil")
		}
	})
}

func TestLoader_GetTemplateDetails(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	templateContent := `name: test-agent
description: A test agent
model: claude-sonnet-4-20250514
system: You are a helpful assistant.
limits:
  max_tokens: 1000
  max_cost_usd: 1.0
  max_duration_sec: 60
`
	templatePath := filepath.Join(tmpDir, "test-agent.yaml")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	details := loader.GetTemplateDetails()
	if len(details) != 1 {
		t.Errorf("expected 1 template detail, got %d", len(details))
	}

	if details[0].Name != "test-agent" {
		t.Errorf("expected name 'test-agent', got '%s'", details[0].Name)
	}
}
