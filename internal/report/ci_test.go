package report

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AlexMcHugh1/lookout/internal/config"
	"github.com/AlexMcHugh1/lookout/internal/runner"
	"github.com/AlexMcHugh1/lookout/internal/vision"
)

func sampleResults() []*runner.Result {
	return []*runner.Result{
		{TestID: "t1", Section: "smoke", Verdict: vision.Verdict{Result: "Pass", Note: "ok"}, Duration: 1 * time.Second, Attempts: 1},
		{TestID: "t2", Section: "auth", Verdict: vision.Verdict{Result: "Fail", Note: "no form"}, Duration: 2 * time.Second, Attempts: 2},
		{TestID: "t3", Section: "dash", Verdict: vision.Verdict{Result: "Blocked", Note: "did not load"}, Duration: 1 * time.Second, Attempts: 1},
		{TestID: "t4", Section: "misc", Verdict: vision.Verdict{Result: "Skipped", Note: "not present"}, Duration: 0, Attempts: 1},
	}
}

func sampleSpec() *config.Spec {
	return &config.Spec{
		App:   config.AppConfig{URL: "https://example.com"},
		Model: config.ModelConfig{Provider: "ollama", Name: "gemma3:12b"},
	}
}

func TestWriteJUnit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "junit.xml")

	if err := WriteJUnit(sampleResults(), sampleSpec(), 4*time.Second, path); err != nil {
		t.Fatalf("WriteJUnit: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	type testsuite struct {
		XMLName  xml.Name `xml:"testsuite"`
		Tests    int      `xml:"tests,attr"`
		Failures int      `xml:"failures,attr"`
		Errors   int      `xml:"errors,attr"`
		Skipped  int      `xml:"skipped,attr"`
	}
	var ts testsuite
	if err := xml.Unmarshal(data, &ts); err != nil {
		t.Fatalf("parse junit: %v", err)
	}
	if ts.Tests != 4 {
		t.Errorf("tests: got %d, want 4", ts.Tests)
	}
	if ts.Failures != 1 {
		t.Errorf("failures: got %d, want 1", ts.Failures)
	}
	if ts.Errors != 1 {
		t.Errorf("errors (Blocked): got %d, want 1", ts.Errors)
	}
	if ts.Skipped != 1 {
		t.Errorf("skipped: got %d, want 1", ts.Skipped)
	}
	if !strings.Contains(string(data), `classname="auth"`) {
		t.Errorf("expected classname for section, output:\n%s", data)
	}
}

func TestWriteJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	if err := WriteJSON(sampleResults(), sampleSpec(), 4*time.Second, "abc1234", path); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var out struct {
		Build   string `json:"build"`
		Summary struct {
			Pass, Fail, Blocked, Skipped, Total int
		} `json:"summary"`
		Results []struct {
			ID       string `json:"id"`
			Attempts int    `json:"attempts"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if out.Build != "abc1234" {
		t.Errorf("build: got %q, want abc1234", out.Build)
	}
	if out.Summary.Pass != 1 || out.Summary.Fail != 1 || out.Summary.Blocked != 1 || out.Summary.Skipped != 1 || out.Summary.Total != 4 {
		t.Errorf("summary: %+v", out.Summary)
	}
	if len(out.Results) != 4 {
		t.Errorf("results: got %d, want 4", len(out.Results))
	}
	if out.Results[1].Attempts != 2 {
		t.Errorf("attempts t2: got %d, want 2", out.Results[1].Attempts)
	}
}
