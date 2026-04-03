package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jefrnc/sekd/internal/edgar"
)

type AIProvider string

const (
	ProviderOpenAI    AIProvider = "openai"
	ProviderAnthropic AIProvider = "anthropic"
)

type AIAnalysis struct {
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	Summary     string `json:"summary"`
	OfferingAmt string `json:"offering_amount,omitempty"`
	Warrants    string `json:"warrants,omitempty"`
	DilutionImpact string `json:"dilution_impact,omitempty"`
	RedFlags    []string `json:"red_flags,omitempty"`
	KeyTerms    []string `json:"key_terms,omitempty"`
}

func DetectProvider() (AIProvider, string) {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return ProviderOpenAI, key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return ProviderAnthropic, key
	}
	return "", ""
}

const filingPrompt = `You are a financial analyst specializing in US small-cap stocks and dilution risk.

Analyze this SEC filing and extract:

1. **Summary**: What is this filing about? (2-3 sentences)
2. **Offering Amount**: Total dollar amount of the offering (if applicable)
3. **Warrants**: Any warrant details — strike price, expiration, number of shares
4. **Dilution Impact**: Estimated dilution to existing shareholders (percentage if calculable)
5. **Red Flags**: List any concerning terms for shareholders (toxic terms, reset provisions, floor price removal, MFN clauses, death spiral convertibles)
6. **Key Terms**: List the most important business terms (interest rates, conversion prices, exercise prices, maturity dates)

Be concise and direct. Focus on what matters to a short seller or small-cap trader evaluating dilution risk.

Respond in this exact JSON format:
{
  "summary": "...",
  "offering_amount": "...",
  "warrants": "...",
  "dilution_impact": "...",
  "red_flags": ["...", "..."],
  "key_terms": ["...", "..."]
}

SEC Filing content:
`

func AnalyzeFiling(ctx context.Context, doc *edgar.FilingDocument) (*AIAnalysis, error) {
	provider, apiKey := DetectProvider()
	if provider == "" {
		return nil, fmt.Errorf("no AI provider configured. Set OPENAI_API_KEY or ANTHROPIC_API_KEY")
	}

	// Truncate to avoid token limits — first 15K chars covers the important parts
	text := doc.CleanText
	if len(text) > 15000 {
		text = text[:15000] + "\n\n[... truncated for analysis ...]"
	}

	prompt := filingPrompt + text

	var result *AIAnalysis
	var err error

	switch provider {
	case ProviderOpenAI:
		result, err = callOpenAI(ctx, apiKey, prompt)
	case ProviderAnthropic:
		result, err = callAnthropic(ctx, apiKey, prompt)
	}

	if err != nil {
		return nil, err
	}
	return result, nil
}

func callOpenAI(ctx context.Context, apiKey, prompt string) (*AIAnalysis, error) {
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature":     0.1,
		"response_format": map[string]string{"type": "json_object"},
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OpenAI returned %d: %s", resp.StatusCode, string(respBody))
	}

	var openaiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, err
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	return parseAIResponse(openaiResp.Choices[0].Message.Content, "openai", model)
}

func callAnthropic(ctx context.Context, apiKey, prompt string) (*AIAnalysis, error) {
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}

	body := map[string]interface{}{
		"model":      model,
		"max_tokens": 2048,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Anthropic returned %d: %s", resp.StatusCode, string(respBody))
	}

	var anthropicResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, err
	}

	if len(anthropicResp.Content) == 0 {
		return nil, fmt.Errorf("Anthropic returned no content")
	}

	return parseAIResponse(anthropicResp.Content[0].Text, "anthropic", model)
}

func parseAIResponse(content, provider, model string) (*AIAnalysis, error) {
	content = strings.TrimSpace(content)
	if idx := strings.Index(content, "{"); idx >= 0 {
		if end := strings.LastIndex(content, "}"); end > idx {
			content = content[idx : end+1]
		}
	}

	// Parse into raw map first to handle mixed types (string vs object)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return &AIAnalysis{
			Provider: provider,
			Model:    model,
			Summary:  content,
		}, nil
	}

	result := AIAnalysis{
		Provider: provider,
		Model:    model,
	}

	result.Summary = rawToString(raw["summary"])
	result.OfferingAmt = rawToString(raw["offering_amount"])
	result.Warrants = rawToString(raw["warrants"])
	result.DilutionImpact = rawToString(raw["dilution_impact"])

	if v, ok := raw["red_flags"]; ok {
		var flags []json.RawMessage
		if json.Unmarshal(v, &flags) == nil {
			for _, f := range flags {
				result.RedFlags = append(result.RedFlags, rawToString(f))
			}
		}
	}

	if v, ok := raw["key_terms"]; ok {
		var terms []json.RawMessage
		if json.Unmarshal(v, &terms) == nil {
			for _, t := range terms {
				result.KeyTerms = append(result.KeyTerms, rawToString(t))
			}
		}
	}

	return &result, nil
}

// rawToString converts a json.RawMessage to a clean string,
// whether the value is a string, object, number, or null.
func rawToString(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}

	// Try as string first
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}

	// Otherwise stringify the JSON value (object, number, etc.)
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "null" || trimmed == "\"N/A\"" || trimmed == "\"\"" {
		return ""
	}

	// For objects/arrays, make it readable
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var pretty map[string]interface{}
		if json.Unmarshal(raw, &pretty) == nil {
			var parts []string
			for k, v := range pretty {
				parts = append(parts, fmt.Sprintf("%s: %v", k, v))
			}
			return strings.Join(parts, ", ")
		}
	}

	return trimmed
}
