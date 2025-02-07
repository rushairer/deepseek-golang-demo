package prompts

import (
	"fmt"
	"strings"
)

// PromptTemplate 定义提示词模板
type PromptTemplate struct {
	Type        string
	Template    string
	Placeholder []string
}

// TemplateManager 管理提示词模板
type TemplateManager struct {
	templates map[string]*PromptTemplate
}

// NewTemplateManager 创建模板管理器
func NewTemplateManager() *TemplateManager {
	return &TemplateManager{
		templates: make(map[string]*PromptTemplate),
	}
}

// RegisterTemplate 注册提示词模板
func (tm *TemplateManager) RegisterTemplate(template *PromptTemplate) {
	tm.templates[template.Type] = template
}

// GetPrompt 根据数据类型和参数生成提示词
func (tm *TemplateManager) GetPrompt(dataType string, params []string) (string, error) {
	template, ok := tm.templates[dataType]
	if !ok {
		return "", fmt.Errorf("template not found for type: %s", dataType)
	}

	if len(params) != len(template.Placeholder) {
		return "", fmt.Errorf("invalid number of parameters")
	}

	prompt := template.Template
	for i, placeholder := range template.Placeholder {
		prompt = strings.Replace(prompt, placeholder, params[i], -1)
	}

	return prompt, nil
}

// DefaultTemplates 返回默认的提示词模板
func DefaultTemplates() []*PromptTemplate {
	return []*PromptTemplate{
		{
			Type: "system",
			Template: `你是一个数据分析助手。请分析输入的数据并返回指定格式的 JSON。返回的 JSON 必须严格遵循以下格式：
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
6. 每种操作类型都有其特定的参数要求，请严格按照示例格式提供`,
			Placeholder: []string{"%DATA%"},
		},
		{
			Type: "text",
			Template: `请分析以下文本内容：
%TEXT%

分析要求：
1. 提取关键信息和主题
2. 识别文本中的实体和关系
3. 评估情感倾向和紧急程度

请按以下JSON格式输出分析结果：
{
    "summary": "关键信息摘要",
    "entities": ["识别到的实体列表"],
    "sentiment": "positive/negative/neutral",
    "urgency": 1-5,
    "actions": [
        {
            "type": "数据库操作/通知/标记",
            "target": "操作对象",
            "params": {"参数名":"参数值"},
            "priority": 1-5
        }
    ]
}`,
			Placeholder: []string{"%TEXT%"},
		},
		{
			Type: "metrics",
			Template: `请分析以下指标数据：
%DATA%
关注指标：%METRICS%

分析要求：
1. 计算关键指标的统计值（平均值/中位数/标准差）
2. 检测异常值（超过±2个标准差）
3. 识别数据趋势（上升/下降/波动）
4. 对比历史基线判断异常

异常判定阈值：
- CPU使用率 > 80%
- 内存使用率 > 90%
- 响应时间 > 1000ms
- 错误率 > 1%

请按以下JSON格式输出分析结果：
{
    "stats": {
        "metric_name": {
            "avg": 数值,
            "median": 数值,
            "std": 数值
        }
    },
    "anomalies": [
        {
            "metric": "指标名",
            "value": 数值,
            "threshold": 数值,
            "severity": 1-5
        }
    ],
    "trend": "上升/下降/波动",
    "actions": [
        {
            "type": "扩容/降级/报警",
            "target": "操作对象",
            "params": {"参数名":"参数值"},
            "priority": 1-5
        }
    ]
}`,
			Placeholder: []string{"%DATA%", "%METRICS%"},
		},
		{
			Type: "log",
			Template: `请分析以下系统日志：
%LOG%

分析要求：
1. 识别错误和警告信息
2. 提取错误码和堆栈信息
3. 判断问题严重程度
4. 关联相似问题

错误等级定义：
- CRITICAL: 系统不可用
- ERROR: 功能故障
- WARNING: 潜在问题
- INFO: 普通信息

请按以下JSON格式输出分析结果：
{
    "level": "错误等级",
    "error_code": "错误码",
    "message": "错误描述",
    "stack_trace": "堆栈信息",
    "frequency": "出现频率",
    "impact": "影响范围",
    "actions": [
        {
            "type": "重启/回滚/清理",
            "target": "操作对象",
            "params": {"参数名":"参数值"},
            "priority": 1-5,
            "rollback": "回滚方案"
        }
    ]
}`,
			Placeholder: []string{"%LOG%"},
		},
	}
}
