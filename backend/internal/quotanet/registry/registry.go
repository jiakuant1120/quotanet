// Package registry tracks active QuotaNet client sessions in memory.
package registry

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
)

var (
	ErrSessionNotFound = errors.New("quotanet session not found")
	ErrInvalidSession  = errors.New("quotanet session is invalid")
	ErrSenderNotFound  = errors.New("quotanet session sender not found")
)

type Sender interface {
	Send(ctx context.Context, envelope protocol.Envelope) error
}

type Session struct {
	SessionID          string
	NodeID             int64
	NodeKey            string
	InstanceID         string
	WalletAddress      string
	ClientVersion      string
	ProtocolVersion    string
	Capabilities       []protocol.Capability
	Status             string
	CurrentConcurrency int
	MaxConcurrency     int
	QueueSize          int
	MaxQueueSize       int
	Accounts           []protocol.AccountHeartbeat
	ConnectedAt        time.Time
	LastHeartbeatAt    time.Time
	DisconnectedAt     *time.Time
	CloseReason        string
}

type Candidate struct {
	SessionID     string
	NodeID        int64
	NodeKey       string
	WalletAddress string
	Provider      string
	Model         string
	Available     int
	QueueSize     int
	MaxQueueSize  int
	LastHeartbeat time.Time
}

type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	senders  map[string]Sender
	now      func() time.Time
}

func New() *Registry {
	return &Registry{
		sessions: make(map[string]*Session),
		senders:  make(map[string]Sender),
		now:      time.Now,
	}
}

func (r *Registry) Register(session Session) error {
	session.SessionID = strings.TrimSpace(session.SessionID)
	session.NodeKey = strings.TrimSpace(session.NodeKey)
	session.InstanceID = strings.TrimSpace(session.InstanceID)
	session.WalletAddress = strings.TrimSpace(session.WalletAddress)
	if session.SessionID == "" || session.NodeID <= 0 || session.NodeKey == "" || session.InstanceID == "" || session.WalletAddress == "" {
		return ErrInvalidSession
	}
	if session.ProtocolVersion == "" {
		session.ProtocolVersion = protocol.Version
	}
	if session.Status == "" {
		session.Status = protocol.NodeStatusReady
	}
	if session.MaxConcurrency < 0 || session.CurrentConcurrency < 0 || session.QueueSize < 0 || session.MaxQueueSize < 0 {
		return ErrInvalidSession
	}
	now := r.currentTime()
	if session.ConnectedAt.IsZero() {
		session.ConnectedAt = now
	}
	if session.LastHeartbeatAt.IsZero() {
		session.LastHeartbeatAt = now
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	copy := cloneSession(session)
	r.sessions[session.SessionID] = &copy
	return nil
}

func (r *Registry) AttachSender(sessionID string, sender Sender) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ErrInvalidSession
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[sessionID]; !ok {
		return ErrSessionNotFound
	}
	if sender == nil {
		delete(r.senders, sessionID)
		return nil
	}
	r.senders[sessionID] = sender
	return nil
}

func (r *Registry) Send(ctx context.Context, sessionID string, envelope protocol.Envelope) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ErrInvalidSession
	}
	r.mu.RLock()
	session, sessionOK := r.sessions[sessionID]
	sender, senderOK := r.senders[sessionID]
	ready := sessionReady(session, r.currentTime(), 0)
	r.mu.RUnlock()
	if !sessionOK {
		return ErrSessionNotFound
	}
	if !ready {
		return ErrInvalidSession
	}
	if !senderOK || sender == nil {
		return ErrSenderNotFound
	}
	return sender.Send(ctx, envelope)
}

func (r *Registry) UpdateHeartbeat(sessionID string, heartbeat protocol.ClientHeartbeat) error {
	if err := heartbeat.Validate(); err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ErrInvalidSession
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	session.WalletAddress = strings.TrimSpace(heartbeat.WalletAddress)
	session.Status = strings.TrimSpace(heartbeat.Status)
	session.CurrentConcurrency = heartbeat.CurrentConcurrency
	session.MaxConcurrency = heartbeat.MaxConcurrency
	session.QueueSize = heartbeat.QueueSize
	session.MaxQueueSize = heartbeat.MaxQueueSize
	session.Accounts = normalizeAccountHeartbeats(heartbeat.Accounts)
	session.LastHeartbeatAt = r.currentTime()
	session.DisconnectedAt = nil
	session.CloseReason = ""
	return nil
}

func (r *Registry) Unregister(sessionID, reason string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ErrInvalidSession
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	now := r.currentTime()
	session.Status = protocol.NodeStatusOffline
	session.DisconnectedAt = &now
	session.CloseReason = strings.TrimSpace(reason)
	delete(r.senders, sessionID)
	return nil
}

func (r *Registry) Get(sessionID string) (Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return Session{}, false
	}
	return cloneSession(*session), true
}

func (r *Registry) Snapshot() []Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Session, 0, len(r.sessions))
	for _, session := range r.sessions {
		out = append(out, cloneSession(*session))
	}
	return out
}

func (r *Registry) Candidates(provider, model string, staleAfter time.Duration) []Candidate {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	now := r.currentTime()

	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Candidate, 0, len(r.sessions))
	for _, session := range r.sessions {
		if !sessionReady(session, now, staleAfter) {
			continue
		}
		available := session.MaxConcurrency - session.CurrentConcurrency
		if available <= 0 {
			continue
		}
		if session.MaxQueueSize > 0 && session.QueueSize >= session.MaxQueueSize {
			continue
		}
		for _, cap := range session.Capabilities {
			if capabilityMatches(cap, provider, model) {
				out = append(out, Candidate{
					SessionID:     session.SessionID,
					NodeID:        session.NodeID,
					NodeKey:       session.NodeKey,
					WalletAddress: session.WalletAddress,
					Provider:      cap.Provider,
					Model:         model,
					Available:     available,
					QueueSize:     session.QueueSize,
					MaxQueueSize:  session.MaxQueueSize,
					LastHeartbeat: session.LastHeartbeatAt,
				})
				break
			}
		}
	}
	sortCandidates(out)
	return out
}

func (r *Registry) AvailableModels(provider string, staleAfter time.Duration) []string {
	provider = strings.TrimSpace(provider)
	now := r.currentTime()

	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, session := range r.sessions {
		if !sessionReady(session, now, staleAfter) {
			continue
		}
		available := session.MaxConcurrency - session.CurrentConcurrency
		if available <= 0 {
			continue
		}
		if session.MaxQueueSize > 0 && session.QueueSize >= session.MaxQueueSize {
			continue
		}
		for _, cap := range session.Capabilities {
			if provider != "" && !strings.EqualFold(strings.TrimSpace(cap.Provider), provider) {
				continue
			}
			for _, model := range cap.Models {
				model = strings.TrimSpace(model)
				if model == "" {
					continue
				}
				key := strings.ToLower(model)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				out = append(out, model)
			}
		}
	}
	sortStrings(out)
	return out
}

func sessionReady(session *Session, now time.Time, staleAfter time.Duration) bool {
	if session == nil || session.DisconnectedAt != nil {
		return false
	}
	switch session.Status {
	case "", protocol.NodeStatusReady, protocol.NodeStatusBusy:
	default:
		return false
	}
	if staleAfter > 0 && !session.LastHeartbeatAt.IsZero() && now.Sub(session.LastHeartbeatAt) > staleAfter {
		return false
	}
	return true
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		current := values[i]
		j := i - 1
		for ; j >= 0 && strings.ToLower(current) < strings.ToLower(values[j]); j-- {
			values[j+1] = values[j]
		}
		values[j+1] = current
	}
}

func capabilityMatches(cap protocol.Capability, provider, model string) bool {
	if provider != "" && !strings.EqualFold(strings.TrimSpace(cap.Provider), provider) {
		return false
	}
	if model == "" || len(cap.Models) == 0 {
		return true
	}
	for _, candidate := range cap.Models {
		if strings.EqualFold(strings.TrimSpace(candidate), model) {
			return true
		}
	}
	return false
}

func sortCandidates(candidates []Candidate) {
	for i := 1; i < len(candidates); i++ {
		current := candidates[i]
		j := i - 1
		for ; j >= 0 && lessCandidate(current, candidates[j]); j-- {
			candidates[j+1] = candidates[j]
		}
		candidates[j+1] = current
	}
}

func lessCandidate(a, b Candidate) bool {
	if a.Available != b.Available {
		return a.Available > b.Available
	}
	if a.QueueSize != b.QueueSize {
		return a.QueueSize < b.QueueSize
	}
	if !a.LastHeartbeat.Equal(b.LastHeartbeat) {
		return a.LastHeartbeat.After(b.LastHeartbeat)
	}
	return a.SessionID < b.SessionID
}

func cloneSession(session Session) Session {
	session.Capabilities = append([]protocol.Capability(nil), session.Capabilities...)
	session.Accounts = append([]protocol.AccountHeartbeat(nil), session.Accounts...)
	if session.DisconnectedAt != nil {
		disconnectedAt := *session.DisconnectedAt
		session.DisconnectedAt = &disconnectedAt
	}
	return session
}

func normalizeAccountHeartbeats(accounts []protocol.AccountHeartbeat) []protocol.AccountHeartbeat {
	out := make([]protocol.AccountHeartbeat, 0, len(accounts))
	for _, account := range accounts {
		account.Provider = strings.TrimSpace(account.Provider)
		account.Status = strings.TrimSpace(account.Status)
		if account.Provider == "" || account.Status == "" {
			continue
		}
		if account.CurrentConcurrency < 0 {
			account.CurrentConcurrency = 0
		}
		if account.MaxConcurrency < 0 {
			account.MaxConcurrency = 0
		}
		models := make([]string, 0, len(account.Models))
		for _, model := range account.Models {
			model = strings.TrimSpace(model)
			if model != "" {
				models = append(models, model)
			}
		}
		account.Models = models
		out = append(out, account)
	}
	return out
}

func (r *Registry) currentTime() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}
