// Package registry tracks active QuotaNet client sessions in memory.
package registry

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
)

var (
	ErrSessionNotFound = errors.New("quotanet session not found")
	ErrInvalidSession  = errors.New("quotanet session is invalid")
)

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
	now      func() time.Time
}

func New() *Registry {
	return &Registry{
		sessions: make(map[string]*Session),
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
	if session.DisconnectedAt != nil {
		disconnectedAt := *session.DisconnectedAt
		session.DisconnectedAt = &disconnectedAt
	}
	return session
}

func (r *Registry) currentTime() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}
