// Package ws contains QuotaNet WebSocket session orchestration.
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
)

const (
	AckStatusOK    = "ok"
	AckStatusError = "error"
)

var (
	ErrUnexpectedEvent = errors.New("unexpected quotanet websocket event")
	ErrNodeRejected    = errors.New("quotanet node rejected")
)

type NodeAuthenticator interface {
	AuthenticateNode(ctx context.Context, token string, hello protocol.ClientHello) (AuthenticatedNode, error)
}

type AuthenticatedNode struct {
	NodeID        int64
	NodeKey       string
	WalletAddress string
}

type SessionManager struct {
	authenticator NodeAuthenticator
	registry      *registry.Registry
	sessionStore  SessionStore
	taskStore     TaskStore
	now           func() time.Time
}

type SessionStore interface {
	SessionConnected(ctx context.Context, session registry.Session, remoteAddr string) error
	SessionHeartbeat(ctx context.Context, sessionID string, heartbeat protocol.ClientHeartbeat, at time.Time) error
	SessionDisconnected(ctx context.Context, sessionID, reason string, at time.Time) error
}

type TaskStore interface {
	TaskResponseReceived(ctx context.Context, sessionID string, response protocol.TaskResponse, at time.Time) error
}

func NewSessionManager(authenticator NodeAuthenticator, reg *registry.Registry) *SessionManager {
	if reg == nil {
		reg = registry.New()
	}
	return &SessionManager{
		authenticator: authenticator,
		registry:      reg,
		now:           time.Now,
	}
}

func (m *SessionManager) WithSessionStore(store SessionStore) *SessionManager {
	m.sessionStore = store
	return m
}

func (m *SessionManager) WithTaskStore(store TaskStore) *SessionManager {
	m.taskStore = store
	return m
}

func (m *SessionManager) HandleHello(ctx context.Context, sessionID, instanceID, token, remoteAddr string, envelope protocol.Envelope) (protocol.Envelope, registry.Session, error) {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(instanceID) == "" {
		return protocol.Envelope{}, registry.Session{}, registry.ErrInvalidSession
	}
	if envelope.Event != protocol.EventClientHello {
		return m.errorAck(envelope.MsgID, fmt.Sprintf("expected %s", protocol.EventClientHello)), registry.Session{}, ErrUnexpectedEvent
	}

	var hello protocol.ClientHello
	if err := envelope.DecodeData(&hello); err != nil {
		return m.errorAck(envelope.MsgID, err.Error()), registry.Session{}, err
	}
	if err := hello.Validate(); err != nil {
		return m.errorAck(envelope.MsgID, err.Error()), registry.Session{}, err
	}
	if m.authenticator == nil {
		return m.errorAck(envelope.MsgID, "node authenticator is not configured"), registry.Session{}, ErrNodeRejected
	}

	node, err := m.authenticator.AuthenticateNode(ctx, token, hello)
	if err != nil {
		return m.errorAck(envelope.MsgID, "node authentication failed"), registry.Session{}, fmt.Errorf("%w: %v", ErrNodeRejected, err)
	}
	node.NodeKey = strings.TrimSpace(node.NodeKey)
	node.WalletAddress = strings.TrimSpace(node.WalletAddress)
	if node.NodeID <= 0 || node.NodeKey == "" {
		return m.errorAck(envelope.MsgID, "node authentication failed"), registry.Session{}, ErrNodeRejected
	}
	if node.WalletAddress == "" {
		node.WalletAddress = strings.TrimSpace(hello.WalletAddress)
	}
	if node.WalletAddress != strings.TrimSpace(hello.WalletAddress) {
		return m.errorAck(envelope.MsgID, "wallet_address does not match registered node"), registry.Session{}, ErrNodeRejected
	}

	capabilities := normalizeCapabilities(hello.Capabilities)
	session := registry.Session{
		SessionID:       strings.TrimSpace(sessionID),
		NodeID:          node.NodeID,
		NodeKey:         node.NodeKey,
		InstanceID:      strings.TrimSpace(instanceID),
		WalletAddress:   node.WalletAddress,
		ClientVersion:   strings.TrimSpace(hello.ClientVersion),
		ProtocolVersion: strings.TrimSpace(hello.ProtocolVersion),
		Capabilities:    capabilities,
		Status:          protocol.NodeStatusReady,
		MaxConcurrency:  maxCapabilityConcurrency(capabilities),
		ConnectedAt:     m.currentTime(),
		LastHeartbeatAt: m.currentTime(),
	}
	if err := m.registry.Register(session); err != nil {
		return m.errorAck(envelope.MsgID, err.Error()), registry.Session{}, err
	}
	if m.sessionStore != nil {
		if err := m.sessionStore.SessionConnected(ctx, session, strings.TrimSpace(remoteAddr)); err != nil {
			_ = m.registry.Unregister(session.SessionID, "session_store_connect_failed")
			return m.errorAck(envelope.MsgID, "failed to persist node session"), registry.Session{}, err
		}
	}

	ack, err := m.okAck(envelope.MsgID, "node connected")
	if err != nil {
		return protocol.Envelope{}, registry.Session{}, err
	}
	return ack, session, nil
}

func (m *SessionManager) HandleHeartbeat(sessionID string, envelope protocol.Envelope) (protocol.Envelope, error) {
	if envelope.Event != protocol.EventClientHeartbeat {
		return m.errorAck(envelope.MsgID, fmt.Sprintf("expected %s", protocol.EventClientHeartbeat)), ErrUnexpectedEvent
	}
	var heartbeat protocol.ClientHeartbeat
	if err := envelope.DecodeData(&heartbeat); err != nil {
		return m.errorAck(envelope.MsgID, err.Error()), err
	}
	if err := m.registry.UpdateHeartbeat(sessionID, heartbeat); err != nil {
		return m.errorAck(envelope.MsgID, err.Error()), err
	}
	if m.sessionStore != nil {
		if err := m.sessionStore.SessionHeartbeat(context.Background(), strings.TrimSpace(sessionID), heartbeat, m.currentTime()); err != nil {
			return m.errorAck(envelope.MsgID, "failed to persist heartbeat"), err
		}
	}
	return m.okAck(envelope.MsgID, "heartbeat accepted")
}

func (m *SessionManager) HandleTaskResponse(ctx context.Context, sessionID string, envelope protocol.Envelope) (protocol.Envelope, error) {
	if envelope.Event != protocol.EventTaskResponse {
		return m.errorAck(envelope.MsgID, fmt.Sprintf("expected %s", protocol.EventTaskResponse)), ErrUnexpectedEvent
	}
	var response protocol.TaskResponse
	if err := envelope.DecodeData(&response); err != nil {
		return m.errorAck(envelope.MsgID, err.Error()), err
	}
	if err := response.Validate(); err != nil {
		return m.errorAck(envelope.MsgID, err.Error()), err
	}
	if m.taskStore == nil {
		return m.errorAck(envelope.MsgID, "task response store is not configured"), ErrUnexpectedEvent
	}
	if err := m.taskStore.TaskResponseReceived(ctx, strings.TrimSpace(sessionID), response, m.currentTime()); err != nil {
		return m.errorAck(envelope.MsgID, "failed to persist task response"), err
	}
	return m.okAck(envelope.MsgID, "task response accepted")
}

func (m *SessionManager) Disconnect(ctx context.Context, sessionID, reason string) {
	sessionID = strings.TrimSpace(sessionID)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = defaultDisconnectReason
	}
	_ = m.registry.Unregister(sessionID, reason)
	if m.sessionStore != nil {
		_ = m.sessionStore.SessionDisconnected(ctx, sessionID, reason, m.currentTime())
	}
}

func (m *SessionManager) Registry() *registry.Registry {
	return m.registry
}

func (m *SessionManager) okAck(msgID, message string) (protocol.Envelope, error) {
	return protocol.NewEnvelope(protocol.EventServerAck, strings.TrimSpace(msgID), protocol.Ack{
		Status:  AckStatusOK,
		Message: message,
	})
}

func (m *SessionManager) errorAck(msgID, message string) protocol.Envelope {
	envelope, err := protocol.NewEnvelope(protocol.EventServerAck, strings.TrimSpace(msgID), protocol.Ack{
		Status:  AckStatusError,
		Message: message,
	})
	if err != nil {
		payload, _ := json.Marshal(protocol.Ack{Status: AckStatusError, Message: message})
		return protocol.Envelope{
			Version:   protocol.Version,
			Event:     protocol.EventServerAck,
			MsgID:     strings.TrimSpace(msgID),
			Timestamp: m.currentTime().Unix(),
			Data:      payload,
		}
	}
	return envelope
}

func (m *SessionManager) currentTime() time.Time {
	if m != nil && m.now != nil {
		return m.now()
	}
	return time.Now()
}

func normalizeCapabilities(capabilities []protocol.Capability) []protocol.Capability {
	out := make([]protocol.Capability, 0, len(capabilities))
	for _, cap := range capabilities {
		cap.Provider = strings.TrimSpace(cap.Provider)
		if cap.Provider == "" {
			continue
		}
		models := make([]string, 0, len(cap.Models))
		for _, model := range cap.Models {
			model = strings.TrimSpace(model)
			if model != "" {
				models = append(models, model)
			}
		}
		cap.Models = models
		if cap.MaxConcurrency < 0 {
			cap.MaxConcurrency = 0
		}
		out = append(out, cap)
	}
	return out
}

func maxCapabilityConcurrency(capabilities []protocol.Capability) int {
	maxConcurrency := 1
	for _, cap := range capabilities {
		if cap.MaxConcurrency > maxConcurrency {
			maxConcurrency = cap.MaxConcurrency
		}
	}
	return maxConcurrency
}
