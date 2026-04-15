// Package config defines the lookout spec schema and loads it from YAML.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ModelConfig holds vision model settings.
type ModelConfig struct {
	Provider string `yaml:"provider"` // ollama | anthropic | openai
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`    // Ollama host
	APIKey   string `yaml:"api_key"` // anthropic / openai
}

func (m *ModelConfig) SetDefaults() {
	if m.Provider == "" {
		m.Provider = "ollama"
	}
	if m.Name == "" {
		m.Name = "gemma3:12b"
	}
	if m.Host == "" {
		m.Host = "http://localhost:11434"
	}
	if m.APIKey == "" {
		m.APIKey = os.Getenv("LOOKOUT_API_KEY")
	}
}

// AuthConfig holds login flow settings.
type AuthConfig struct {
	Type               string `yaml:"type"`                 // email_password
	LoginPath          string `yaml:"login_path"`           // path to login page, default "/login"
	EmailField         string `yaml:"email_field"`
	ContinueButton     string `yaml:"continue_button"`      // optional two-step
	PasswordField      string `yaml:"password_field"`
	SubmitButton       string `yaml:"submit_button"`
	SuccessURLExcludes string `yaml:"success_url_excludes"` // e.g. "/login"
	Email              string `yaml:"email"`
	Password           string `yaml:"password"`
}

func (a *AuthConfig) SetDefaults() {
	if a.Type == "" {
		a.Type = "email_password"
	}
	if a.LoginPath == "" {
		a.LoginPath = "/login"
	}
	if a.EmailField == "" {
		a.EmailField = `input[type="email"]`
	}
	if a.PasswordField == "" {
		a.PasswordField = `input[type="password"]`
	}
	if a.SubmitButton == "" {
		a.SubmitButton = `button[type="submit"]:not(:text("Google")):not(:text("GitHub"))`
	}
	if a.SuccessURLExcludes == "" {
		a.SuccessURLExcludes = "/login"
	}
	if a.Email == "" {
		a.Email = os.Getenv("LOOKOUT_EMAIL")
	}
	if a.Password == "" {
		a.Password = os.Getenv("LOOKOUT_PASSWORD")
	}
}

// AppConfig holds the target application settings.
type AppConfig struct {
	URL  string     `yaml:"url"`
	Auth AuthConfig `yaml:"auth"`
}

// PreAction defines an optional interaction to run before screenshotting.
type PreAction struct {
	Type           string `yaml:"type"`
	Selector       string `yaml:"selector"`
	ClickSelector  string `yaml:"click_selector"`
	EditorSelector string `yaml:"editor_selector"`
	FallbackButton string `yaml:"fallback_button"`
	Source         string `yaml:"source"`
	Target         string `yaml:"target"`
	Text           string `yaml:"text"`
	HoldMs         int    `yaml:"hold_ms"`
	ReloadAfter    bool   `yaml:"reload_after"`
	WaitMs         int    `yaml:"wait_ms"`
	Ms             int    `yaml:"ms"`
}

// TestDef is a single test case.
type TestDef struct {
	ID        string     `yaml:"id"`
	Section   string     `yaml:"section"`
	URL       string     `yaml:"url"` // relative path
	Question  string     `yaml:"question"`
	WaitMs    int        `yaml:"wait_ms,omitempty"`   // extra wait after navigation / pre-action
	WaitFor   string     `yaml:"wait_for,omitempty"`  // CSS selector to wait for before screenshot
	FullPage  *bool      `yaml:"full_page,omitempty"` // capture full scrollable page (default true)
	PreAction *PreAction `yaml:"pre_action,omitempty"`
}

// Spec is the top-level lookout.yaml structure.
type Spec struct {
	App   AppConfig   `yaml:"app"`
	Model ModelConfig `yaml:"model"`
	Tests []TestDef   `yaml:"tests"`
}

// LoadYAML reads and parses a lookout.yaml file.
func LoadYAML(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}

	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	spec.Model.SetDefaults()
	spec.App.Auth.SetDefaults()

	return &spec, nil
}
