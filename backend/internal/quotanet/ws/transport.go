package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
)

const defaultDisconnectReason = "connection_closed"

var ErrNilTransport = errors.New("quotanet websocket transport is nil")

type Conn interface {
	ReadJSON(v any) error
	WriteJSON(v any) error
	Close() error
}

type ServeOptions struct {
	SessionID   string
	InstanceID  string
	Token       string
	RemoteAddr  string
	CloseReason string
}

func (m *SessionManager) Serve(ctx context.Context, conn Conn, opts ServeOptions) error {
	if conn == nil {
		return ErrNilTransport
	}
	defer func() { _ = conn.Close() }()

	var helloEnvelope protocol.Envelope
	if err := conn.ReadJSON(&helloEnvelope); err != nil {
		return fmt.Errorf("read quotanet hello: %w", err)
	}
	ack, session, err := m.HandleHello(ctx, opts.SessionID, opts.InstanceID, opts.Token, opts.RemoteAddr, helloEnvelope)
	if writeErr := conn.WriteJSON(ack); writeErr != nil {
		return fmt.Errorf("write quotanet hello ack: %w", writeErr)
	}
	if err != nil {
		return err
	}
	if sender, ok := conn.(registrySender); ok {
		if err := m.registry.AttachSender(session.SessionID, sender); err != nil {
			return err
		}
	}

	closeReason := strings.TrimSpace(opts.CloseReason)
	if closeReason == "" {
		closeReason = defaultDisconnectReason
	}
	defer func() {
		m.Disconnect(context.Background(), session.SessionID, closeReason)
	}()

	for {
		var envelope protocol.Envelope
		if err := conn.ReadJSON(&envelope); err != nil {
			return err
		}
		switch envelope.Event {
		case protocol.EventClientHeartbeat:
			ack, err := m.HandleHeartbeat(session.SessionID, envelope)
			if writeErr := conn.WriteJSON(ack); writeErr != nil {
				return fmt.Errorf("write quotanet heartbeat ack: %w", writeErr)
			}
			if err != nil {
				return err
			}
		case protocol.EventTaskResponse:
			ack, err := m.HandleTaskResponse(ctx, session.SessionID, envelope)
			if writeErr := conn.WriteJSON(ack); writeErr != nil {
				return fmt.Errorf("write quotanet task response ack: %w", writeErr)
			}
			if err != nil {
				return err
			}
		default:
			ack := m.errorAck(envelope.MsgID, fmt.Sprintf("unsupported event %q", envelope.Event))
			if writeErr := conn.WriteJSON(ack); writeErr != nil {
				return fmt.Errorf("write quotanet error ack: %w", writeErr)
			}
			return ErrUnexpectedEvent
		}
	}
}

func EncodeEnvelopeForTest(envelope protocol.Envelope) []byte {
	data, _ := json.Marshal(envelope)
	return data
}

type registrySender interface {
	Send(ctx context.Context, envelope protocol.Envelope) error
}

type SerialConn struct {
	conn Conn
	mu   sync.Mutex
}

func NewSerialConn(conn Conn) *SerialConn {
	return &SerialConn{conn: conn}
}

func (c *SerialConn) ReadJSON(v any) error {
	return c.conn.ReadJSON(v)
}

func (c *SerialConn) WriteJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}

func (c *SerialConn) Send(ctx context.Context, envelope protocol.Envelope) error {
	done := make(chan error, 1)
	go func() {
		done <- c.WriteJSON(envelope)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func (c *SerialConn) Close() error {
	return c.conn.Close()
}
