package actions

import (
 "context"
 "testing"
 "deepseek_golang_demo/models"
 "deepseek_golang_demo/services/deepseek"
)
type fakeStore struct{recordID int64}
func(f *fakeStore)UpdateStatus(_ context.Context,id int64,_ string)error{f.recordID=id;return nil};func(f *fakeStore)AddTag(context.Context,int64,string)error{return nil};func(f *fakeStore)CreateNotification(context.Context,models.Notification)(int64,bool,error){return 1,true,nil};func(f *fakeStore)UpdateNotificationStatus(context.Context,int64,string)error{return nil};func(f *fakeStore)CreateApproval(context.Context,models.ApprovalRequest)(int64,error){return 1,nil}
type fakeNotifier struct{};func(fakeNotifier)Send(context.Context,string,string,string)error{return nil};func(fakeNotifier)HasTarget(string,string)bool{return true}
func TestServerBindsRecordID(t *testing.T){s:=&fakeStore{};e:=NewExecutor(s,fakeNotifier{},false);r:=e.Execute(context.Background(),ExecutionContext{RunID:1,RecordID:42},deepseek.ToolCall{ID:"x",Function:deepseek.FunctionCall{Name:ToolUpdateStatus,Arguments:`{"status":"resolved"}`}});if r.Status!="succeeded"||s.recordID!=42{t.Fatalf("unexpected result: %+v id=%d",r,s.recordID)}}
func TestInjectedRecordIDRejected(t *testing.T){e:=NewExecutor(&fakeStore{},fakeNotifier{},false);r:=e.Execute(context.Background(),ExecutionContext{RunID:1,RecordID:42},deepseek.ToolCall{ID:"x",Function:deepseek.FunctionCall{Name:ToolUpdateStatus,Arguments:`{"status":"resolved","record_id":1}`}});if r.Status!="failed"{t.Fatalf("expected failure: %+v",r)}}
