package deepseek

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	Analysis    string   `json:"analysis"`
	Suggestions []string `json:"suggestions"`
	Confidence  float64  `json:"confidence"`
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

	request := ChatCompletionRequest{
		Model: "deepseek-chat",
		Messages: []ChatMessage{
			{Role: "system", Content: `你是一个数据分析助手。请分析输入的数据并返回指定格式的 JSON。返回的 JSON 必须严格遵循以下格式：
{
    "analysis": "这里是分析结果文本",
    "suggestions": ["建议1", "建议2", "建议3"],
    "confidence": 0.95
}
请注意：
1. 不要添加任何额外的文本或 Markdown 标记
2. confidence 必须是 0-1 之间的浮点数
3. suggestions 必须是字符串数组`},
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

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content = apiResp.Choices[0].Message.Content
	fmt.Printf("DeepSeek Content Response: %s\n", content)

	var analysisResp AnalysisResponse
	if err := json.Unmarshal([]byte(content), &analysisResp); err != nil {
		return nil, fmt.Errorf("error parsing analysis response from content '%s': %v", content, err)
	}

	return &analysisResp, nil
}
