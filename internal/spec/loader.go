// Package spec loads lookout specs from YAML or PDF documents.
package spec

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alexmchughdev/lookout/internal/config"
)

// Load auto-detects the file format and loads a Spec.
// url overrides the app.url from the spec (useful when parsing PDFs).
func Load(path string, model *config.ModelConfig, url string) (*config.Spec, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".yaml", ".yml":
		spec, err := config.LoadYAML(path)
		if err != nil {
			return nil, err
		}
		if url != "" {
			spec.App.URL = strings.TrimRight(url, "/")
		}
		return spec, nil

	case ".pdf":
		return loadPDF(path, model, url)

	default:
		return nil, fmt.Errorf("unsupported spec format %q — use .yaml or .pdf", ext)
	}
}

// loadPDF extracts test cases from a PDF using the Ollama vision API.
func loadPDF(path string, model *config.ModelConfig, url string) (*config.Spec, error) {
	if model == nil {
		m := config.ModelConfig{}
		m.SetDefaults()
		model = &m
	}

	if url == "" {
		return nil, fmt.Errorf("--url is required when loading a PDF spec")
	}

	pages, err := renderPDF(path)
	if err != nil {
		return nil, fmt.Errorf("rendering PDF pages: %w\n"+
			"Install poppler:\n"+
			"  Ubuntu/Debian:  sudo apt install poppler-utils\n"+
			"  Arch:           sudo pacman -S poppler\n"+
			"  Fedora:         sudo dnf install poppler-utils\n"+
			"  macOS:          brew install poppler", err)
	}

	tests, err := extractTests(pages, model)
	if err != nil {
		return nil, fmt.Errorf("extracting tests from PDF: %w", err)
	}

	spec := &config.Spec{
		App: config.AppConfig{
			URL: strings.TrimRight(url, "/"),
		},
		Model: *model,
		Tests: tests,
	}
	spec.App.Auth.SetDefaults()

	return spec, nil
}

// renderPDF renders each page of a PDF to a base64 PNG using pdftoppm or ImageMagick convert.
func renderPDF(path string) ([]string, error) {
	tmpDir, err := os.MkdirTemp("", "lookout_pdf_")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	outPrefix := filepath.Join(tmpDir, "page")

	if err := exec.Command("pdftoppm", "-r", "150", "-png", path, outPrefix).Run(); err != nil {
		if err2 := exec.Command("convert", "-density", "150", path, outPrefix+"-%03d.png").Run(); err2 != nil {
			return nil, fmt.Errorf("pdftoppm failed (%v); convert also failed (%v)", err, err2)
		}
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, err
	}

	var pages []string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".png") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(tmpDir, e.Name()))
		if err != nil {
			continue
		}
		pages = append(pages, base64.StdEncoding.EncodeToString(data))
		if len(pages) >= 10 {
			break
		}
	}

	if len(pages) == 0 {
		return nil, fmt.Errorf("no PNG pages generated from PDF")
	}

	return pages, nil
}

const extractPrompt = `You are reading a QA test specification document.
Extract ALL test cases from these pages.

For each test case output a JSON object with these exact fields:
- "id": the test case ID (e.g. "G-01", "N-03", "W-02")
- "section": section name lowercase (e.g. "navigation", "notes", "workflows")
- "url": the URL path to test (e.g. "/login", "/notes", "/settings")
- "question": a yes/no question answerable by looking at a screenshot

Output ONLY a JSON array. No explanation, no markdown, no code fences.
Example: [{"id":"G-01","section":"navigation","url":"/login","question":"Is a login form visible?"}]`

type ollamaReq struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	Images []string `json:"images"`
	Stream bool     `json:"stream"`
}

type ollamaResp struct {
	Response string `json:"response"`
}

func extractTests(pages []string, model *config.ModelConfig) ([]config.TestDef, error) {
	body, _ := json.Marshal(ollamaReq{
		Model:  model.Name,
		Prompt: extractPrompt,
		Images: pages,
		Stream: false,
	})

	resp, err := http.Post(model.Host+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("calling Ollama: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var or ollamaResp
	if err := json.Unmarshal(data, &or); err != nil {
		return nil, fmt.Errorf("parsing Ollama response: %w", err)
	}

	text := regexp.MustCompile("```(?:json)?|```").ReplaceAllString(or.Response, "")
	text = strings.TrimSpace(text)

	var raw []map[string]string
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf(
			"could not parse test cases from model output.\n"+
				"Raw response:\n%s\n\nTip: use a YAML spec for more reliable results",
			text[:min(len(text), 500)],
		)
	}

	tests := make([]config.TestDef, 0, len(raw))
	for _, r := range raw {
		if r["id"] == "" || r["question"] == "" {
			continue
		}
		u := r["url"]
		if u == "" {
			u = "/"
		}
		tests = append(tests, config.TestDef{
			ID:       r["id"],
			Section:  r["section"],
			URL:      u,
			Question: r["question"],
		})
	}

	return tests, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
