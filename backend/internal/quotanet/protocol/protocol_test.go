package protocol

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNewEnvelopeRoundTrip(t *testing.T) {
	hello := ClientHello{
		ClientID:        "client-1",
		ClientVersion:   "v0.1.0",
		WalletAddress:   "wallet-1",
		ProtocolVersion: Version,
		Capabilities: []Capability{
			{Provider: "openai_compatible", Models: []string{"gpt-4o"}, MaxConcurrency: 2},
		},
	}

	envelope, err := NewEnvelope(EventClientHello, "msg-1", hello)
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	if err := envelope.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded Envelope
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	var got ClientHello
	if err := decoded.DecodeData(&got); err != nil {
		t.Fatalf("DecodeData() error = %v", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("ClientHello.Validate() error = %v", err)
	}
	if got.ClientID != hello.ClientID || got.WalletAddress != hello.WalletAddress {
		t.Fatalf("decoded hello = %+v, want %+v", got, hello)
	}
}

func TestEnvelopeValidateRejectsUnsupportedVersion(t *testing.T) {
	envelope := Envelope{
		Version: "old",
		Event:   EventClientHeartbeat,
		MsgID:   "msg-1",
		Data:    []byte(`{}`),
	}
	if err := envelope.Validate(); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("Validate() error = %v, want ErrUnsupportedVersion", err)
	}
}

func TestClientHelloValidate(t *testing.T) {
	valid := ClientHello{
		ClientID:        "client-1",
		WalletAddress:   "wallet-1",
		ProtocolVersion: Version,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalid := valid
	invalid.ProtocolVersion = "old"
	if err := invalid.Validate(); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("Validate() error = %v, want ErrUnsupportedVersion", err)
	}
}

func TestTaskDispatchValidate(t *testing.T) {
	task := TaskDispatch{
		TaskID:         "task-1",
		Provider:       "openai_compatible",
		Model:          "gpt-4o",
		Endpoint:       "/v1/chat/completions",
		TimeoutSeconds: 120,
		Payload:        map[string]any{"messages": []any{}},
	}
	if err := task.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	task.TaskID = ""
	if err := task.Validate(); err == nil {
		t.Fatal("Validate() expected task_id error")
	}
}

func TestTaskResponseValidate(t *testing.T) {
	response := TaskResponse{
		TaskID: "task-1",
		Status: TaskStatusSuccess,
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
		DurationMS:   1000,
		FirstTokenMS: 200,
	}
	if err := response.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	response.Status = "unknown"
	if err := response.Validate(); err == nil {
		t.Fatal("Validate() expected invalid status error")
	}
}

func TestSettlementNoticeValidateAndJSONNames(t *testing.T) {
	notice := SettlementNotice{
		ID:        "settlement-1",
		AmountCXS: "1.2500",
		TokenFlow: 44000,
		TxHash:    "tx-1",
		Status:    SettlementStatusPending,
		CreatedAt: "2026-06-04T14:18:45Z",
		UpdatedAt: "2026-06-04T14:18:45Z",
	}
	if err := notice.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	encoded, err := json.Marshal(notice)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	for _, key := range []string{"amountCxs", "tokenFlow", "txHash", "createdAt", "updatedAt"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("encoded settlement missing key %q: %s", key, string(encoded))
		}
	}
}
