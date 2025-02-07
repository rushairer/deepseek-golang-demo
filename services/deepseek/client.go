package deepseek

import (
	"bytes"
	"deepseek_golang_demo/models"
	"deepseek_golang_demo/prompts"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Client struct {
	apiKey  string
	baseURL string
}

type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type AnalysisResponse struct {
	Analysis    string          `json:"analysis"`
	Suggestions []string        `json:"suggestions"`
	Confidence  float64         `json:"confidence"`
	Actions     []models.Action `json:"actions"`
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: "https://api.deepseek.com/v1",
	}
}

func (c *Client) AnalyzeData(prompt string, data interface{}) (*AnalysisResponse, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("error marshaling data: %v", err)
	}
	content := fmt.Sprintf("%s\nData: %s", prompt, string(dataJSON))

	templateManager := prompts.NewTemplateManager()
	for _, template := range prompts.DefaultTemplates() {
		templateManager.RegisterTemplate(template)
	}

	systemPrompt, err := templateManager.GetPrompt("system", []string{string(dataJSON)})
	if err != nil {
		return nil, fmt.Errorf("error getting system prompt: %v", err)
	}

	request := ChatCompletionRequest{
		Model: "deepseek-chat",
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: content},
		},
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Printf("DeepSeek API Response: %s\n", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp ChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("error decoding API response: %v", err)
	}
	log.Println("DeepSeek API Response: ", apiResp)

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content = apiResp.Choices[0].Message.Content

	var analysisResp AnalysisResponse
	if err := json.Unmarshal([]byte(content), &analysisResp); err != nil {
		return nil, fmt.Errorf("error parsing analysis response from content '%s': %v", content, err)
	}

	return &analysisResp, nil
}
