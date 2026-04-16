package config

import (
	"strings"
	"testing"
)

func TestValidate_OK(t *testing.T) {
	s := &Spec{
		App: AppConfig{URL: "https://example.com"},
		Tests: []TestDef{
			{ID: "a", URL: "/", Question: "is the page loaded?"},
			{ID: "b", URL: "/login", Question: "is the login form visible?"},
		},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidate_MissingURL(t *testing.T) {
	s := &Spec{
		Tests: []TestDef{{ID: "a", URL: "/", Question: "q"}},
	}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "app.url is required") {
		t.Fatalf("expected app.url error, got %v", err)
	}
}

func TestValidate_NoTests(t *testing.T) {
	s := &Spec{App: AppConfig{URL: "https://example.com"}}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "no tests defined") {
		t.Fatalf("expected no-tests error, got %v", err)
	}
}

func TestValidate_MissingTestFields(t *testing.T) {
	s := &Spec{
		App: AppConfig{URL: "https://example.com"},
		Tests: []TestDef{
			{ID: "", URL: "", Question: ""},
		},
	}
	err := s.Validate()
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
	for _, want := range []string{"`id` is required", "`question` is required", "`url` is required"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error containing %q, got: %v", want, err)
		}
	}
}

func TestValidate_DuplicateIDs(t *testing.T) {
	s := &Spec{
		App: AppConfig{URL: "https://example.com"},
		Tests: []TestDef{
			{ID: "dup", URL: "/a", Question: "q"},
			{ID: "dup", URL: "/b", Question: "q"},
		},
	}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate id \"dup\"") {
		t.Fatalf("expected duplicate-id error, got %v", err)
	}
}

func TestAuthConfig_Defaults(t *testing.T) {
	a := AuthConfig{}
	a.SetDefaults()
	if a.Type != "email_password" {
		t.Errorf("Type default: got %q", a.Type)
	}
	if a.LoginPath != "/login" {
		t.Errorf("LoginPath default: got %q", a.LoginPath)
	}
	if a.SuccessURLExcludes != "/login" {
		t.Errorf("SuccessURLExcludes default: got %q", a.SuccessURLExcludes)
	}
}

func TestModelConfig_Defaults(t *testing.T) {
	m := ModelConfig{}
	m.SetDefaults()
	if m.Provider != "ollama" {
		t.Errorf("Provider default: got %q", m.Provider)
	}
	if m.Name != "gemma3:12b" {
		t.Errorf("Name default: got %q", m.Name)
	}
	if m.Host != "http://localhost:11434" {
		t.Errorf("Host default: got %q", m.Host)
	}
}
