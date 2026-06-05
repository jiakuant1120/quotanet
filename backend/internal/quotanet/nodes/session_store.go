package nodes

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/quotanetnodesession"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
)

func (s *EntStore) SessionConnected(ctx context.Context, session registry.Session, remoteAddr string) error {
	if _, err := s.client.QuotaNetNodeSession.Create().
		SetSessionID(session.SessionID).
		SetNodeID(session.NodeID).
		SetInstanceID(session.InstanceID).
		SetStatus(session.Status).
		SetNillableRemoteAddr(nillableString(remoteAddr)).
		SetMaxConcurrency(session.MaxConcurrency).
		SetCurrentConcurrency(session.CurrentConcurrency).
		SetQueueSize(session.QueueSize).
		SetMaxQueueSize(session.MaxQueueSize).
		SetCapabilities(sessionCapabilitiesPayload(session.Capabilities, session.Accounts)).
		SetConnectedAt(session.ConnectedAt).
		SetLastHeartbeatAt(session.LastHeartbeatAt).
		Save(ctx); err != nil {
		return err
	}
	return s.client.QuotaNetNode.UpdateOneID(session.NodeID).
		SetProtocolVersion(session.ProtocolVersion).
		SetClientVersion(session.ClientVersion).
		SetLastSeenAt(session.LastHeartbeatAt).
		Exec(ctx)
}

func (s *EntStore) SessionHeartbeat(ctx context.Context, sessionID string, heartbeat protocol.ClientHeartbeat, at time.Time) error {
	existing, err := s.client.QuotaNetNodeSession.Query().
		Where(quotanetnodesession.SessionIDEQ(sessionID)).
		Only(ctx)
	if err != nil {
		return err
	}
	capabilities := mergeSessionAccounts(existing.Capabilities, heartbeat.Accounts)
	if _, err := s.client.QuotaNetNodeSession.Update().
		Where(quotanetnodesession.SessionIDEQ(sessionID)).
		SetStatus(heartbeat.Status).
		SetCurrentConcurrency(heartbeat.CurrentConcurrency).
		SetMaxConcurrency(heartbeat.MaxConcurrency).
		SetQueueSize(heartbeat.QueueSize).
		SetMaxQueueSize(heartbeat.MaxQueueSize).
		SetCapabilities(capabilities).
		SetLastHeartbeatAt(at).
		ClearDisconnectedAt().
		ClearCloseReason().
		Save(ctx); err != nil {
		return err
	}
	return s.client.QuotaNetNode.UpdateOneID(existing.NodeID).
		SetLastSeenAt(at).
		Exec(ctx)
}

func (s *EntStore) SessionDisconnected(ctx context.Context, sessionID, reason string, at time.Time) error {
	_, err := s.client.QuotaNetNodeSession.Update().
		Where(quotanetnodesession.SessionIDEQ(sessionID)).
		SetStatus(protocol.NodeStatusOffline).
		SetDisconnectedAt(at).
		SetCloseReason(reason).
		Save(ctx)
	return err
}

func sessionCapabilitiesPayload(capabilities []protocol.Capability, accounts []protocol.AccountHeartbeat) map[string]any {
	payload := map[string]any{}
	if capabilities != nil {
		payload["items"] = capabilities
	}
	if accounts != nil {
		payload["accounts"] = accounts
	}
	return payload
}

func mergeSessionAccounts(payload map[string]any, accounts []protocol.AccountHeartbeat) map[string]any {
	out := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		out[key] = value
	}
	out["accounts"] = accounts
	return out
}

func nillableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
