// Package protocol defines the wire messages exchanged by quotanet-server and
// QuotaNet Client nodes.
package protocol

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const Version = "2026-06-qt1"

const (
	EventClientHello     = "client_hello"
	EventClientHeartbeat = "client_heartbeat"
	EventServerAck       = "server_ack"
	EventTaskDispatch    = "task_dispatch"
	EventTaskDelta       = "task_delta"
	EventTaskResponse    = "task_response"
	EventTaskCancel      = "task_cancel"
	EventSettlementNotice = "settlement_notice"
	EventRouterShutdown   = "router_shutdown"
)

const (
	TaskStatusQueued    = "queued"
	TaskStatusRunning   = "running"
	TaskStatusSuccess   = "success"
	TaskStatusFailed    = "failed"
	TaskStatusTimeout   = "timeout"
	TaskStatusCancelled = "cancelled"
)

const (
	NodeStatusReady      = "ready"
	NodeStatusBusy       = "busy"
	NodeStatusCooldown   = "cooldown"
	NodeStatusDegraded   = "degraded"
	NodeStatusOffline    = "offline"
	NodeStatusError      = "error"
	NodeStatusStopped    = "stopped"
	NodeStatusConnecting = "connecting"
)

const (
	SettlementStatusPending   = "pending"
	SettlementStatusFinalized = "finalized"
	SettlementStatusFailed    = "failed"
)

var (
	ErrUnsupportedVersion = errors.New("unsupported quotanet protocol version")
	ErrMissingEvent       = errors.New("quotanet envelope event is required")
	ErrMissingMessageID   = errors.New("quotanet envelope msg_id is required")
)

type Envelope struct {
	Version   string          `json:"version"`
	Event     string          `json:"event"`
	MsgID     string          `json:"msg_id"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

func NewEnvelope(event, msgID string, data any) (Envelope, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		Version:   Version,
		Event:     strings.TrimSpace(event),
		MsgID:     strings.TrimSpace(msgID),
		Timestamp: time.Now().Unix(),
		Data:      payload,
	}, nil
}

func (e Envelope) Validate() error {
	if strings.TrimSpace(e.Version) != Version {
		return ErrUnsupportedVersion
	}
	if strings.TrimSpace(e.Event) == "" {
		return ErrMissingEvent
	}
	if strings.TrimSpace(e.MsgID) == "" {
		return ErrMissingMessageID
	}
	return nil
}

func (e Envelope) DecodeData(v any) error {
	if err := e.Validate(); err != nil {
		return err
	}
	return json.Unmarshal(e.Data, v)
}

type Ack struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ClientHello struct {
	ClientID        string       `json:"client_id"`
	ClientVersion   string       `json:"client_version"`
	WalletAddress   string       `json:"wallet_address"`
	ProtocolVersion string       `json:"protocol_version"`
	Capabilities    []Capability `json:"capabilities,omitempty"`
}

func (p ClientHello) Validate() error {
	if strings.TrimSpace(p.ClientID) == "" {
		return errors.New("client_id is required")
	}
	if strings.TrimSpace(p.WalletAddress) == "" {
		return errors.New("wallet_address is required")
	}
	if strings.TrimSpace(p.ProtocolVersion) != Version {
		return ErrUnsupportedVersion
	}
	return nil
}

type Capability struct {
	Provider       string   `json:"provider"`
	Models         []string `json:"models,omitempty"`
	MaxConcurrency int      `json:"max_concurrency,omitempty"`
}

type ClientHeartbeat struct {
	WalletAddress      string             `json:"wallet_address"`
	Status             string             `json:"status"`
	CurrentConcurrency int                `json:"current_concurrency"`
	MaxConcurrency     int                `json:"max_concurrency"`
	QueueSize          int                `json:"queue_size,omitempty"`
	MaxQueueSize       int                `json:"max_queue_size,omitempty"`
	Accounts           []AccountHeartbeat `json:"accounts,omitempty"`
}

func (p ClientHeartbeat) Validate() error {
	if strings.TrimSpace(p.WalletAddress) == "" {
		return errors.New("wallet_address is required")
	}
	if strings.TrimSpace(p.Status) == "" {
		return errors.New("status is required")
	}
	if p.CurrentConcurrency < 0 {
		return errors.New("current_concurrency must be non-negative")
	}
	if p.MaxConcurrency < 0 {
		return errors.New("max_concurrency must be non-negative")
	}
	if p.QueueSize < 0 {
		return errors.New("queue_size must be non-negative")
	}
	if p.MaxQueueSize < 0 {
		return errors.New("max_queue_size must be non-negative")
	}
	return nil
}

type AccountHeartbeat struct {
	Provider           string   `json:"provider"`
	Status             string   `json:"status"`
	CurrentConcurrency int      `json:"current_concurrency"`
	MaxConcurrency     int      `json:"max_concurrency"`
	Models             []string `json:"models,omitempty"`
}

type TaskDispatch struct {
	TaskID         string         `json:"task_id"`
	Provider       string         `json:"provider"`
	Model          string         `json:"model"`
	Endpoint       string         `json:"endpoint,omitempty"`
	Stream         bool           `json:"stream,omitempty"`
	TimeoutSeconds int            `json:"timeout_seconds,omitempty"`
	Payload        map[string]any `json:"payload"`
}

func (p TaskDispatch) Validate() error {
	if strings.TrimSpace(p.TaskID) == "" {
		return errors.New("task_id is required")
	}
	if strings.TrimSpace(p.Provider) == "" {
		return errors.New("provider is required")
	}
	if strings.TrimSpace(p.Model) == "" {
		return errors.New("model is required")
	}
	if p.TimeoutSeconds < 0 {
		return errors.New("timeout_seconds must be non-negative")
	}
	return nil
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (u Usage) Validate() error {
	if u.PromptTokens < 0 || u.CompletionTokens < 0 || u.TotalTokens < 0 {
		return errors.New("usage tokens must be non-negative")
	}
	return nil
}

type TaskResponse struct {
	TaskID       string         `json:"task_id"`
	Status       string         `json:"status"`
	ErrorCode    string         `json:"error_code,omitempty"`
	ErrorMessage string         `json:"error_msg,omitempty"`
	Usage        Usage          `json:"usage"`
	Payload      map[string]any `json:"payload,omitempty"`
	DurationMS   int            `json:"duration_ms,omitempty"`
	FirstTokenMS int            `json:"first_token_ms,omitempty"`
}

func (p TaskResponse) Validate() error {
	if strings.TrimSpace(p.TaskID) == "" {
		return errors.New("task_id is required")
	}
	switch strings.TrimSpace(p.Status) {
	case TaskStatusSuccess, TaskStatusFailed, TaskStatusTimeout, TaskStatusCancelled:
	default:
		return errors.New("status is invalid")
	}
	if err := p.Usage.Validate(); err != nil {
		return err
	}
	if p.DurationMS < 0 {
		return errors.New("duration_ms must be non-negative")
	}
	if p.FirstTokenMS < 0 {
		return errors.New("first_token_ms must be non-negative")
	}
	return nil
}

type TaskDelta struct {
	TaskID   string `json:"task_id"`
	Sequence int64  `json:"sequence"`
	Data     string `json:"data"`
}

func (p TaskDelta) Validate() error {
	if strings.TrimSpace(p.TaskID) == "" {
		return errors.New("task_id is required")
	}
	if p.Sequence < 0 {
		return errors.New("sequence must be non-negative")
	}
	return nil
}

type TaskCancel struct {
	TaskID string `json:"task_id"`
	Reason string `json:"reason,omitempty"`
}

func (p TaskCancel) Validate() error {
	if strings.TrimSpace(p.TaskID) == "" {
		return errors.New("task_id is required")
	}
	return nil
}

type SettlementNotice struct {
	ID         string `json:"id"`
	AmountCXS  string `json:"amountCxs"`
	TokenFlow  int64  `json:"tokenFlow"`
	TxHash     string `json:"txHash,omitempty"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt,omitempty"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
}

func (p SettlementNotice) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return errors.New("id is required")
	}
	if strings.TrimSpace(p.AmountCXS) == "" {
		return errors.New("amountCxs is required")
	}
	if p.TokenFlow < 0 {
		return errors.New("tokenFlow must be non-negative")
	}
	switch strings.TrimSpace(p.Status) {
	case SettlementStatusPending, SettlementStatusFinalized, SettlementStatusFailed:
	default:
		return errors.New("status is invalid")
	}
	return nil
}
