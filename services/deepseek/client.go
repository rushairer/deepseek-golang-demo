package deepseek

import (
	"bytes"
	"deepseek_golang_demo/models"
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

	request := ChatCompletionRequest{
		Model: "deepseek-chat",
		Messages: []ChatMessage{
			{Role: "system", Content: `你是一个数据分析助手。请分析输入的数据并返回指定格式的 JSON。返回的 JSON 必须严格遵循以下格式：
{
    "analysis": "这里是分析结果文本",
    "suggestions": ["建议1", "建议2", "建议3"],
    "confidence": 0.95,
    "actions": [
        {
            "type": "database",
            "target": "update_status 或 add_tag",
            "params": {
                "record_id": 123,
                "status": "新状态" // 当 target 为 update_status 时
                "tag": "标签名称" // 当 target 为 add_tag 时
            },
            "priority": 1
        },
        {
            "type": "notification",
            "target": "发送通知",
            "params": {
                "record_id": 123,
                "message": "通知内容",
                "channel": "通知渠道"
            },
            "priority": 2
        },
        {
            "type": "tag",
            "target": "添加标签",
            "params": {
                "record_id": 123,
                "tag": "标签名称"
            },
            "priority": 3
        }
    ]
}
请注意：
1. 不要添加任何额外的文本或 Markdown 标记
2. confidence 必须是 0-1 之间的浮点数
3. suggestions 必须是字符串数组
4. actions 中的每个操作都必须包含 type、target、params、priority 字段
5. record_id 必须是数值类型
6. 每种操作类型都有其特定的参数要求，请严格按照示例格式提供`},
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
