package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aksara/internal/content"
)

const deepSeekBaseURL = "https://api.deepseek.com"

type Translator struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

type TranslatorConfig struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
}

func NewTranslator(cfg TranslatorConfig) *Translator {
	if cfg.BaseURL == "" {
		cfg.BaseURL = deepSeekBaseURL
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 120 * time.Second
	}
	return &Translator{
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		client:  &http.Client{Timeout: cfg.Timeout},
	}
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type blockResponse struct {
	Blocks []content.Block `json:"blocks"`
}

const novelSystemPrompt = `You are a literary translator translating English novels into natural Indonesian.
Goal: help an Indonesian reader comfortably follow the story without translating sentence by sentence.
Rules:
1. Preserve plot, tone, character voice, dialogue, and emotional nuance.
2. Do not summarize, omit, add events, or explain your translation.
3. Keep character names, place names, invented terms, and proper nouns consistent.
4. Translate idioms by meaning, not word-for-word.
5. Preserve code blocks, poems, letters, and special text as their own blocks when present.
6. Return ONLY valid JSON with this shape:
{"blocks":[{"type":"paragraph","text":"..."},{"type":"heading","text":"..."},{"type":"code","text":"..."}]}
Allowed block types: heading, paragraph, code, list_item.`

func (t *Translator) TranslateBlocks(ctx context.Context, text string) ([]content.Block, error) {
	body, err := json.Marshal(chatRequest{
		Model: t.model,
		Messages: []chatMessage{
			{Role: "system", Content: novelSystemPrompt},
			{Role: "user", Content: text},
		},
		ResponseFormat: &responseFormat{Type: "json_object"},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal translation request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create translation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transient translation http error: %w", err)
	}
	defer resp.Body.Close()

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode translation response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, classifyAPIError(resp.StatusCode, result.Error)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("permanent translation api error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("transient translation api error: empty choices")
	}

	var parsed blockResponse
	if err := json.Unmarshal([]byte(result.Choices[0].Message.Content), &parsed); err != nil {
		return nil, fmt.Errorf("invalid translation json: %w", err)
	}
	if len(parsed.Blocks) == 0 {
		return nil, fmt.Errorf("invalid translation json: no blocks")
	}
	for i, block := range parsed.Blocks {
		if block.Text == "" {
			return nil, fmt.Errorf("invalid translation json: block %d has empty text", i)
		}
		if block.Type == "" {
			parsed.Blocks[i].Type = content.BlockParagraph
			continue
		}
		if !validBlockType(block.Type) {
			return nil, fmt.Errorf("invalid translation json: block %d has unsupported type %q", i, block.Type)
		}
	}
	return parsed.Blocks, nil
}

func classifyAPIError(status int, apiErr *struct {
	Message string `json:"message"`
}) error {
	message := http.StatusText(status)
	if apiErr != nil && apiErr.Message != "" {
		message = apiErr.Message
	}
	switch {
	case status == http.StatusTooManyRequests || status >= 500:
		return fmt.Errorf("transient translation api error: status %d: %s", status, message)
	case status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusBadRequest:
		return fmt.Errorf("permanent translation api error: status %d: %s", status, message)
	default:
		return fmt.Errorf("permanent translation api error: status %d: %s", status, message)
	}
}

func validBlockType(blockType content.BlockType) bool {
	switch blockType {
	case content.BlockHeading, content.BlockParagraph, content.BlockCode, content.BlockListItem:
		return true
	default:
		return false
	}
}
