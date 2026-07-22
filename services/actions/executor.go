package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"deepseek_golang_demo/models"
	"deepseek_golang_demo/services/deepseek"
)

const ( ToolUpdateStatus="update_record_status"; ToolAddTag="add_record_tag"; ToolSendNotification="send_notification" )
type Store interface { UpdateStatus(context.Context,int64,string)error; AddTag(context.Context,int64,string)error; CreateNotification(context.Context,models.Notification)(int64,bool,error); UpdateNotificationStatus(context.Context,int64,string)error; CreateApproval(context.Context,models.ApprovalRequest)(int64,error) }
type Notifier interface { Send(context.Context,string,string,string)error; HasTarget(string,string)bool }
type Executor struct{store Store; notifier Notifier; requireApproval bool}
type ExecutionContext struct{RunID,RecordID int64}
type Result struct{ToolCallID string `json:"toolCallId"`; ToolName string `json:"toolName"`; Status string `json:"status"`; Data map[string]any `json:"data,omitempty"`; Error string `json:"error,omitempty"`}
type updateStatusArgs struct{Status string `json:"status"`}; type addTagArgs struct{Tag string `json:"tag"`}; type sendArgs struct{Channel string `json:"channel"`;Target string `json:"target"`;Message string `json:"message"`}
func NewExecutor(s Store,n Notifier,approval bool)*Executor{return &Executor{store:s,notifier:n,requireApproval:approval}}
func(e *Executor)Tools()[]deepseek.Tool{return []deepseek.Tool{
 {Type:"function",Function:deepseek.FunctionTool{Name:ToolUpdateStatus,Description:"Update the status of the current record. The server binds the record ID.",Strict:true,Parameters:schema(map[string]any{"status":map[string]any{"type":"string","enum":[]string{"new","processing","needs_review","resolved","ignored"}}},[]string{"status"})}},
 {Type:"function",Function:deepseek.FunctionTool{Name:ToolAddTag,Description:"Attach a concise tag to the current record.",Strict:true,Parameters:schema(map[string]any{"tag":map[string]any{"type":"string","minLength":1,"maxLength":64}},[]string{"tag"})}},
 {Type:"function",Function:deepseek.FunctionTool{Name:ToolSendNotification,Description:"Request a notification through a configured target alias.",Strict:true,Parameters:schema(map[string]any{"channel":map[string]any{"type":"string","enum":[]string{"email","webhook"}},"target":map[string]any{"type":"string","minLength":1,"maxLength":64},"message":map[string]any{"type":"string","minLength":1,"maxLength":2000}},[]string{"channel","target","message"})}},}}
func(e *Executor)Execute(ctx context.Context,x ExecutionContext,c deepseek.ToolCall)Result{r:=Result{ToolCallID:c.ID,ToolName:c.Function.Name,Status:"failed"}; switch c.Function.Name{
case ToolUpdateStatus: var a updateStatusArgs; if err:=decode(c.Function.Arguments,&a);err!=nil{r.Error=err.Error();return r}; allowed:=map[string]bool{"new":true,"processing":true,"needs_review":true,"resolved":true,"ignored":true}; if !allowed[a.Status]{r.Error="invalid status";return r}; if err:=e.store.UpdateStatus(ctx,x.RecordID,a.Status);err!=nil{r.Error=err.Error();return r}; r.Status="succeeded"; r.Data=map[string]any{"recordId":x.RecordID,"status":a.Status}; return r
case ToolAddTag: var a addTagArgs; if err:=decode(c.Function.Arguments,&a);err!=nil{r.Error=err.Error();return r}; a.Tag=strings.TrimSpace(a.Tag); if a.Tag==""||len([]rune(a.Tag))>64{r.Error="tag must contain 1 to 64 characters";return r}; if err:=e.store.AddTag(ctx,x.RecordID,a.Tag);err!=nil{r.Error=err.Error();return r}; r.Status="succeeded"; r.Data=map[string]any{"recordId":x.RecordID,"tag":a.Tag}; return r
case ToolSendNotification: var a sendArgs; if err:=decode(c.Function.Arguments,&a);err!=nil{r.Error=err.Error();return r}; a.Channel=strings.ToLower(strings.TrimSpace(a.Channel)); a.Target=strings.TrimSpace(a.Target); a.Message=strings.TrimSpace(a.Message); if !e.notifier.HasTarget(a.Channel,a.Target){r.Error="notification target is not configured";return r}; if e.requireApproval{b,_:=json.Marshal(a); id,err:=e.store.CreateApproval(ctx,models.ApprovalRequest{RunID:x.RunID,RecordID:x.RecordID,ToolCallID:c.ID,ToolName:c.Function.Name,Arguments:string(b)}); if err!=nil{r.Error=err.Error();return r}; r.Status="pending_approval"; r.Data=map[string]any{"approvalId":id}; return r}; return e.send(ctx,x,c.ID,a)
default:r.Error="unknown tool";return r}}
func(e *Executor)ExecuteApprovedNotification(ctx context.Context,a *models.ApprovalRequest)Result{var args sendArgs; if err:=decode(a.Arguments,&args);err!=nil{return Result{ToolCallID:a.ToolCallID,ToolName:a.ToolName,Status:"failed",Error:err.Error()}}; return e.send(ctx,ExecutionContext{RunID:a.RunID,RecordID:a.RecordID},a.ToolCallID,args)}
func(e *Executor)send(ctx context.Context,x ExecutionContext,id string,a sendArgs)Result{r:=Result{ToolCallID:id,ToolName:ToolSendNotification,Status:"failed"}; key:=fmt.Sprintf("run:%d:tool:%s",x.RunID,id); nid,created,err:=e.store.CreateNotification(ctx,models.Notification{RecordID:x.RecordID,Channel:a.Channel,Target:a.Target,Message:a.Message,IdempotencyKey:key}); if err!=nil{r.Error=err.Error();return r}; if !created{r.Status="succeeded";r.Data=map[string]any{"notificationId":nid,"duplicate":true};return r}; if err:=e.notifier.Send(ctx,a.Channel,a.Target,a.Message);err!=nil{_ = e.store.UpdateNotificationStatus(ctx,nid,"failed");r.Error=err.Error();return r}; if err:=e.store.UpdateNotificationStatus(ctx,nid,"sent");err!=nil{r.Error=err.Error();return r};r.Status="succeeded";r.Data=map[string]any{"notificationId":nid};return r}
func schema(p map[string]any,r []string)map[string]any{return map[string]any{"type":"object","properties":p,"required":r,"additionalProperties":false}}
func decode(raw string,v any)error{d:=json.NewDecoder(bytes.NewBufferString(raw));d.DisallowUnknownFields();if err:=d.Decode(v);err!=nil{return fmt.Errorf("invalid tool arguments: %w",err)};if err:=d.Decode(&struct{}{});err!=io.EOF{return fmt.Errorf("invalid tool arguments: trailing JSON data")};return nil}
