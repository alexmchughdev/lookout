// Package vision sends screenshots to a vision model and returns verdicts.
package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/AlexMcHugh1/lookout/internal/config"
)

// Verdict is the result of a single test assessment.
type Verdict struct {
	Result string // Pass | Fail | Blocked | Skipped
	Note   string // one-sentence explanation
}

const promptTemplate = `You are a QA tester reviewing a screenshot of a web application.

Question: %s

Reply with ONLY these two lines — nothing else:
RESULT: <Pass or Fail or Blocked or Skipped>
NOTE: <one sentence describing what you see>

Rules:
- Pass if the condition described in the question is met
- Fail if the condition is clearly not met
- Blocked if you cannot determine the answer (e.g. page not loaded)
- Skipped if the feature described is not present in the app`

var (
	resultRe = regexp.MustCompile(`(?i)RESULT:\s*(Pass|Fail|Blocked|Skipped)`)
	noteRe   = regexp.MustCompile(`(?i)NOTE:\s*(.+)`)
)

// Judge assesses a screenshot against a question using the configured model.
func Judge(screenshot []byte, question string, model config.ModelConfig) (Verdict, error) {
	b64 := base64.StdEncoding.EncodeToString(screenshot)
	prompt := fmt.Sprintf(promptTemplate, question)

	switch model.Provider {
	case "ollama", "":
		return judgeOllama(b64, prompt, model)
	case "anthropic":
		return judgeAnthropic(b64, prompt, model)
	case "openai":
		return judgeOpenAI(b64, prompt, model)
	default:
		return Verdict{}, fmt.Errorf("unknown provider %q (use ollama, anthropic, or openai)", model.Provider)
	}
}

func parseResponse(text string) Verdict {
	v := Verdict{
		Result: "Blocked",
		Note:   strings.TrimSpace(text),
	}
	if m := resultRe.FindStringSubmatch(text); len(m) > 1 {
		lower := strings.ToLower(m[1])
		if len(lower) > 0 {
			v.Result = strings.ToUpper(lower[:1]) + lower[1:]
		}
	}
	if m := noteRe.FindStringSubmatch(text); len(m) > 1 {
		v.Note = strings.TrimSpace(m[1])
		if len(v.Note) > 200 {
			v.Note = v.Note[:200]
		}
	}
	return v
}

// ── Ollama ────────────────────────────────────────────────────────────────────

type ollamaRequest struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	Images []string `json:"images"`
	Stream bool     `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

func judgeOllama(b64, prompt string, model config.ModelConfig) (Verdict, error) {
	body, err := json.Marshal(ollamaRequest{
		Model:  model.Name,
		Prompt: prompt,
		Images: []string{b64},
		Stream: false,
	})
	if err != nil {
		return Verdict{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		model.Host+"/api/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Verdict{Result: "Blocked", Note: fmt.Sprintf("Ollama error: %v", err)}, nil
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var ollamaResp ollamaResponse
	if err := json.Unmarshal(data, &ollamaResp); err != nil {
		return Verdict{Result: "Blocked", Note: "failed to parse Ollama response"}, nil
	}

	return parseResponse(ollamaResp.Response), nil
}

// ── Anthropic ─────────────────────────────────────────────────────────────────

func judgeAnthropic(b64, prompt string, model config.ModelConfig) (Verdict, error) {
	if model.APIKey == "" {
		return Verdict{}, fmt.Errorf("anthropic provider requires api_key in spec or LOOKOUT_API_KEY env var")
	}

	payload := map[string]any{
		"model":      model.Name,
		"max_tokens": 256,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "image",
						"source": map[string]string{
							"type":       "base64",
							"media_type": "image/png",
							"data":       b64,
						},
					},
					{"type": "text", "text": prompt},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", model.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Verdict{Result: "Blocked", Note: fmt.Sprintf("Anthropic error: %v", err)}, nil
	}
	defer resp.Body.Close()

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Content) == 0 {
		return Verdict{Result: "Blocked", Note: "failed to parse Anthropic response"}, nil
	}

	return parseResponse(result.Content[0].Text), nil
}

// ── OpenAI ────────────────────────────────────────────────────────────────────

func judgeOpenAI(b64, prompt string, model config.ModelConfig) (Verdict, error) {
	if model.APIKey == "" {
		return Verdict{}, fmt.Errorf("openai provider requires api_key in spec or LOOKOUT_API_KEY env var")
	}

	modelName := model.Name
	if modelName == "" {
		modelName = "gpt-4o"
	}

	payload := map[string]any{
		"model":      modelName,
		"max_tokens": 256,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type":      "image_url",
						"image_url": map[string]string{"url": "data:image/png;base64," + b64},
					},
					{"type": "text", "text": prompt},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+model.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Verdict{Result: "Blocked", Note: fmt.Sprintf("OpenAI error: %v", err)}, nil
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Choices) == 0 {
		return Verdict{Result: "Blocked", Note: "failed to parse OpenAI response"}, nil
	}

	return parseResponse(result.Choices[0].Message.Content), nil
}
