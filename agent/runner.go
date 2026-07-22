package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"deepseek_golang_demo/models"
	"deepseek_golang_demo/prompts"
	"deepseek_golang_demo/services/actions"
	"deepseek_golang_demo/services/deepseek"
)

type Runner struct {
	client          *deepseek.Client
	executor        *actions.Executor
	store           *models.Store
	maxSteps        int
	maxOutputTokens int
}

type Result struct {
	Run        *models.AgentRun      `json:"run"`
	Analysis   models.AnalysisResult `json:"analysis"`
	Executions []actions.Result      `json:"executions"`
}

type finalResponse struct {
	Analysis    string   `json:"analysis"`
	Suggestions []string `json:"suggestions"`
	Confidence  float64  `json:"confidence"`
}

func NewRunner(client *deepseek.Client, executor *actions.Executor, store *models.Store, maxSteps, maxOutputTokens int) *Runner {
	return &Runner{client: client, executor: executor, store: store, maxSteps: maxSteps, maxOutputTokens: maxOutputTokens}
}

func (r *Runner) Run(ctx context.Context, record *models.DataRecord) (_ *Result, runErr error) {
	run, err := r.store.CreateAgentRun(ctx, record.ID, r.client.Model(), prompts.Version, r.maxSteps)
	if err != nil {
		return nil, err
	}
	stepsUsed, inputTokens, outputTokens := 0, 0, 0
	defer func() {
		if runErr != nil {
			_ = r.store.FinishAgentRun(context.WithoutCancel(ctx), run.ID, "failed", stepsUsed, inputTokens, outputTokens, runErr.Error())
		}
	}()

	userPrompt, err := prompts.User(record)
	if err != nil {
		return nil, err
	}
	messages := []deepseek.ChatMessage{
		{Role: "system", Content: prompts.System},
		{Role: "user", Content: userPrompt},
	}
	executions := make([]actions.Result, 0)

	for step := 1; step <= r.maxSteps; step++ {
		stepsUsed = step
		response, err := r.client.Complete(ctx, messages, r.executor.Tools(), r.maxOutputTokens)
		if err != nil {
			return nil, err
		}
		inputTokens += response.Usage.PromptTokens
		outputTokens += response.Usage.CompletionTokens
		choice := response.Choices[0]
		modelSnapshot, _ := json.Marshal(choice.Message)
		if err := r.store.AddAgentStep(ctx, models.AgentStep{
			RunID: run.ID, StepNumber: step, Kind: "model", Result: string(modelSnapshot), Status: "succeeded",
		}); err != nil {
			return nil, err
		}

		if len(choice.Message.ToolCalls) == 0 {
			final, err := parseFinal(choice.Message.Content)
			if err != nil {
				return nil, err
			}
			analysisResult := models.AnalysisResult{
				RecordID: record.ID, Analysis: final.Analysis, Suggestions: final.Suggestions, Confidence: final.Confidence,
			}
			if err := r.store.SaveAnalysisResult(ctx, &analysisResult); err != nil {
				return nil, err
			}
			if err := r.store.FinishAgentRun(ctx, run.ID, "completed", stepsUsed, inputTokens, outputTokens, ""); err != nil {
				return nil, err
			}
			run.Status, run.StepsUsed, run.InputTokens, run.OutputTokens = "completed", stepsUsed, inputTokens, outputTokens
			return &Result{Run: run, Analysis: analysisResult, Executions: executions}, nil
		}

		messages = append(messages, choice.Message)
		for _, call := range choice.Message.ToolCalls {
			execution := r.executor.Execute(ctx, actions.ExecutionContext{RunID: run.ID, RecordID: record.ID}, call)
			executions = append(executions, execution)
			observation, _ := json.Marshal(execution)
			stepStatus := execution.Status
			if stepStatus == "pending_approval" {
				stepStatus = "pending"
			}
			if err := r.store.AddAgentStep(ctx, models.AgentStep{
				RunID: run.ID, StepNumber: step, Kind: "tool", ToolCallID: call.ID,
				ToolName: call.Function.Name, Arguments: call.Function.Arguments,
				Result: string(observation), Status: stepStatus, ErrorMessage: execution.Error,
			}); err != nil {
				return nil, err
			}
			messages = append(messages, deepseek.ChatMessage{Role: "tool", ToolCallID: call.ID, Content: string(observation)})
		}
	}
	return nil, fmt.Errorf("agent exceeded maximum of %d steps", r.maxSteps)
}

func parseFinal(content string) (*finalResponse, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("model returned empty final content")
	}
	var result finalResponse
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode final analysis JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("decode final analysis JSON: trailing data")
	}
	result.Analysis = strings.TrimSpace(result.Analysis)
	if result.Analysis == "" {
		return nil, fmt.Errorf("final analysis is empty")
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		return nil, fmt.Errorf("final confidence must be between 0 and 1")
	}
	if result.Suggestions == nil {
		result.Suggestions = []string{}
	}
	return &result, nil
}
