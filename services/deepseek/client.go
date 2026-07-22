package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	apiKey, baseURL, model string
	httpClient             *http.Client
}
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type Tool struct {
	Type     string       `json:"type"`
	Function FunctionTool `json:"function"`
}
type FunctionTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Strict      bool           `json:"strict,omitempty"`
}
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
}
type request struct {
	Model          string            `json:"model"`
	Messages       []ChatMessage     `json:"messages"`
	Tools          []Tool            `json:"tools,omitempty"`
	ToolChoice     string            `json:"tool_choice,omitempty"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	Temperature    float64           `json:"temperature"`
	Stream         bool              `json:"stream"`
}
type apiError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewClient(key, baseURL, model string, timeout time.Duration) *Client {
	return &Client{apiKey: key, baseURL: strings.TrimRight(baseURL, "/"), model: model, httpClient: &http.Client{Timeout: timeout}}
}
func (c *Client) Model() string { return c.model }
func (c *Client) Complete(ctx context.Context, messages []ChatMessage, tools []Tool, maxTokens int) (*ChatCompletionResponse, error) {
	body, err := json.Marshal(request{Model: c.model, Messages: messages, Tools: tools, ToolChoice: "auto", ResponseFormat: map[string]string{"type": "json_object"}, MaxTokens: maxTokens, Temperature: 0, Stream: false})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call DeepSeek API: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var ae apiError
		if json.Unmarshal(payload, &ae) == nil && ae.Error.Message != "" {
			return nil, fmt.Errorf("DeepSeek API returned %d: %s", resp.StatusCode, ae.Error.Message)
		}
		return nil, fmt.Errorf("DeepSeek API returned %d", resp.StatusCode)
	}
	var out ChatCompletionResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("decode DeepSeek response: %w", err)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("DeepSeek response contained no choices")
	}
	return &out, nil
}
