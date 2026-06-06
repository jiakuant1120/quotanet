package tasks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
)

func TestDispatcherDispatch(t *testing.T) {
	reg := registry.New()
	if err := reg.Register(validSession("sess-1", 10)); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	sender := &stubSender{}
	if err := reg.AttachSender("sess-1", sender); err != nil {
		t.Fatalf("AttachSender() error = %v", err)
	}
	store := &stubStore{}
	dispatcher := NewDispatcher(store, reg)
	dispatcher.newTaskID = func() string { return "task-1" }
	dispatcher.newMessage = func() string { return "msg-1" }

	task, err := dispatcher.Dispatch(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if task.TaskID != "task-1" || task.Status != protocol.TaskStatusRunning {
		t.Fatalf("task = %+v", task)
	}
	if store.dispatchedTaskID != "task-1" || store.dispatchedCandidate.SessionID != "sess-1" {
		t.Fatalf("dispatch store state task=%q candidate=%+v", store.dispatchedTaskID, store.dispatchedCandidate)
	}
	if sender.sent.Event != protocol.EventTaskDispatch {
		t.Fatalf("sent event = %q, want task_dispatch", sender.sent.Event)
	}
	if len(store.events) != 1 || store.events[0].eventType != protocol.EventTaskDispatch {
		t.Fatalf("events = %+v", store.events)
	}
}

func TestDispatcherDispatchWithTaskID(t *testing.T) {
	reg := registry.New()
	if err := reg.Register(validSession("sess-1", 10)); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	sender := &stubSender{}
	if err := reg.AttachSender("sess-1", sender); err != nil {
		t.Fatalf("AttachSender() error = %v", err)
	}
	store := &stubStore{}
	dispatcher := NewDispatcher(store, reg)
	dispatcher.newTaskID = func() string { return "unused-task" }

	task, err := dispatcher.DispatchWithTaskID(context.Background(), validInput(), "pre-registered-task")
	if err != nil {
		t.Fatalf("DispatchWithTaskID() error = %v", err)
	}
	if task.TaskID != "pre-registered-task" || store.dispatchedTaskID != "pre-registered-task" {
		t.Fatalf("task=%+v dispatched=%q", task, store.dispatchedTaskID)
	}
}

func TestDispatcherDispatchToNodeID(t *testing.T) {
	reg := registry.New()
	if err := reg.Register(validSession("sess-1", 10)); err != nil {
		t.Fatalf("Register(sess-1) error = %v", err)
	}
	if err := reg.Register(validSession("sess-2", 20)); err != nil {
		t.Fatalf("Register(sess-2) error = %v", err)
	}
	if err := reg.AttachSender("sess-1", &stubSender{}); err != nil {
		t.Fatalf("AttachSender(sess-1) error = %v", err)
	}
	targetSender := &stubSender{}
	if err := reg.AttachSender("sess-2", targetSender); err != nil {
		t.Fatalf("AttachSender(sess-2) error = %v", err)
	}
	store := &stubStore{}
	dispatcher := NewDispatcher(store, reg)
	dispatcher.newTaskID = func() string { return "task-1" }
	dispatcher.newMessage = func() string { return "msg-1" }

	task, err := dispatcher.DispatchToNodeID(context.Background(), validInput(), 20)
	if err != nil {
		t.Fatalf("DispatchToNodeID() error = %v", err)
	}
	if task.NodeID == nil || *task.NodeID != 20 || store.dispatchedCandidate.SessionID != "sess-2" {
		t.Fatalf("task=%+v candidate=%+v", task, store.dispatchedCandidate)
	}
	if targetSender.sent.Event != protocol.EventTaskDispatch {
		t.Fatalf("target sender event = %q, want task_dispatch", targetSender.sent.Event)
	}
}

func TestDispatcherDispatchToUnavailableNodeID(t *testing.T) {
	reg := registry.New()
	if err := reg.Register(validSession("sess-1", 10)); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := reg.AttachSender("sess-1", &stubSender{}); err != nil {
		t.Fatalf("AttachSender() error = %v", err)
	}
	store := &stubStore{}
	dispatcher := NewDispatcher(store, reg)
	dispatcher.newTaskID = func() string { return "task-1" }

	_, err := dispatcher.DispatchToNodeID(context.Background(), validInput(), 99)
	if !errors.Is(err, ErrNoNodeAvailable) {
		t.Fatalf("DispatchToNodeID() error = %v, want ErrNoNodeAvailable", err)
	}
	if store.failedCode != "NO_NODE_AVAILABLE" {
		t.Fatalf("failed code = %q", store.failedCode)
	}
}

func TestDispatcherNoNodeAvailable(t *testing.T) {
	store := &stubStore{}
	dispatcher := NewDispatcher(store, registry.New())
	dispatcher.newTaskID = func() string { return "task-1" }

	_, err := dispatcher.Dispatch(context.Background(), validInput())
	if !errors.Is(err, ErrNoNodeAvailable) {
		t.Fatalf("Dispatch() error = %v, want ErrNoNodeAvailable", err)
	}
	if store.failedCode != "NO_NODE_AVAILABLE" {
		t.Fatalf("failed code = %q", store.failedCode)
	}
}

func TestDispatcherSendFailureMarksFailed(t *testing.T) {
	reg := registry.New()
	if err := reg.Register(validSession("sess-1", 10)); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := reg.AttachSender("sess-1", &stubSender{err: errors.New("send failed")}); err != nil {
		t.Fatalf("AttachSender() error = %v", err)
	}
	store := &stubStore{}
	dispatcher := NewDispatcher(store, reg)
	dispatcher.newTaskID = func() string { return "task-1" }

	_, err := dispatcher.Dispatch(context.Background(), validInput())
	if err == nil {
		t.Fatal("Dispatch() error = nil, want send failure")
	}
	if store.failedCode != "DISPATCH_SEND_FAILED" {
		t.Fatalf("failed code = %q", store.failedCode)
	}
}

func TestDispatcherRejectsInvalidInput(t *testing.T) {
	dispatcher := NewDispatcher(&stubStore{}, registry.New())
	if _, err := dispatcher.Dispatch(context.Background(), CreateTaskInput{}); !errors.Is(err, ErrInvalidTaskInput) {
		t.Fatalf("Dispatch(empty) error = %v, want ErrInvalidTaskInput", err)
	}
	if _, err := dispatcher.DispatchWithTaskID(context.Background(), validInput(), " "); !errors.Is(err, ErrInvalidTaskInput) {
		t.Fatalf("DispatchWithTaskID(empty task id) error = %v, want ErrInvalidTaskInput", err)
	}
}

func validInput() CreateTaskInput {
	return CreateTaskInput{
		RequestID:      "req-1",
		Platform:       "openai",
		Endpoint:       "/v1/chat/completions",
		Model:          "gpt-4.1",
		TimeoutSeconds: 60,
		Payload:        map[string]any{"messages": []any{}},
	}
}

func validSession(sessionID string, nodeID int64) registry.Session {
	return registry.Session{
		SessionID:      sessionID,
		NodeID:         nodeID,
		NodeKey:        "node-1",
		InstanceID:     "inst-1",
		WalletAddress:  "wallet-1",
		Status:         protocol.NodeStatusReady,
		MaxConcurrency: 2,
		Capabilities: []protocol.Capability{
			{Provider: "openai", Models: []string{"gpt-4.1"}, MaxConcurrency: 2},
		},
		LastHeartbeatAt: time.Now(),
	}
}

type stubSender struct {
	sent protocol.Envelope
	err  error
}

func (s *stubSender) Send(_ context.Context, envelope protocol.Envelope) error {
	if s.err != nil {
		return s.err
	}
	s.sent = envelope
	return nil
}

type stubStore struct {
	dispatchedTaskID    string
	dispatchedCandidate registry.Candidate
	failedCode          string
	events              []stubEvent
}

func (s *stubStore) CreateQueued(_ context.Context, input CreateTaskInput, taskID string) (*Task, error) {
	return &Task{
		ID:        1,
		TaskID:    taskID,
		RequestID: input.RequestID,
		Platform:  input.Platform,
		Endpoint:  input.Endpoint,
		Model:     input.Model,
		Stream:    input.Stream,
		Status:    protocol.TaskStatusQueued,
	}, nil
}

func (s *stubStore) MarkDispatched(_ context.Context, taskID string, candidate registry.Candidate, _ time.Time) error {
	s.dispatchedTaskID = taskID
	s.dispatchedCandidate = candidate
	return nil
}

func (s *stubStore) AppendEvent(_ context.Context, taskID, eventType string, sequence int64, payload map[string]any) error {
	s.events = append(s.events, stubEvent{taskID: taskID, eventType: eventType, sequence: sequence, payload: payload})
	return nil
}

func (s *stubStore) MarkFailed(_ context.Context, _ string, code, _ string, _ time.Time) error {
	s.failedCode = code
	return nil
}

type stubEvent struct {
	taskID    string
	eventType string
	sequence  int64
	payload   map[string]any
}
