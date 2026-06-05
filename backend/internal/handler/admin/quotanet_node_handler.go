package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/nodes"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/tasks"

	"github.com/gin-gonic/gin"
)

type QuotaNetNodeHandler struct {
	manager   *nodes.Manager
	reg       *registry.Registry
	taskStore *tasks.EntStore
}

func NewQuotaNetNodeHandler(manager *nodes.Manager, reg *registry.Registry, taskStore *tasks.EntStore) *QuotaNetNodeHandler {
	return &QuotaNetNodeHandler{manager: manager, reg: reg, taskStore: taskStore}
}

type quotaNetNodeCreateRequest struct {
	Name          string `json:"name" binding:"required,max=100"`
	WalletAddress string `json:"wallet_address" binding:"required,max=128"`
	OwnerUserID   *int64 `json:"owner_user_id"`
	Status        string `json:"status" binding:"omitempty,oneof=pending active disabled banned"`
}

type quotaNetNodeStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=pending active disabled banned"`
}

type quotaNetNodeResponse struct {
	ID            int64   `json:"id"`
	NodeKey       string  `json:"node_key"`
	Name          string  `json:"name"`
	OwnerUserID   *int64  `json:"owner_user_id,omitempty"`
	WalletAddress string  `json:"wallet_address"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at,omitempty"`
	UpdatedAt     string  `json:"updated_at,omitempty"`
	LastSeenAt    *string `json:"last_seen_at,omitempty"`
}

type quotaNetNodeCreateResponse struct {
	Node  *quotaNetNodeResponse `json:"node"`
	Token string                `json:"token"`
}

type quotaNetNodeSessionResponse struct {
	SessionID          string                      `json:"session_id"`
	NodeID             int64                       `json:"node_id"`
	NodeKey            string                      `json:"node_key"`
	InstanceID         string                      `json:"instance_id"`
	WalletAddress      string                      `json:"wallet_address"`
	ClientVersion      string                      `json:"client_version,omitempty"`
	ProtocolVersion    string                      `json:"protocol_version,omitempty"`
	Capabilities       []protocol.Capability       `json:"capabilities"`
	Status             string                      `json:"status"`
	CurrentConcurrency int                         `json:"current_concurrency"`
	MaxConcurrency     int                         `json:"max_concurrency"`
	QueueSize          int                         `json:"queue_size"`
	MaxQueueSize       int                         `json:"max_queue_size"`
	Accounts           []protocol.AccountHeartbeat `json:"accounts,omitempty"`
	ConnectedAt        string                      `json:"connected_at,omitempty"`
	LastHeartbeatAt    string                      `json:"last_heartbeat_at,omitempty"`
	DisconnectedAt     *string                     `json:"disconnected_at,omitempty"`
	CloseReason        string                      `json:"close_reason,omitempty"`
}

type quotaNetNodeOverviewResponse struct {
	Sessions       quotaNetSessionOverview        `json:"sessions"`
	Capacity       quotaNetCapacityOverview       `json:"capacity"`
	Providers      []quotaNetProviderOverview     `json:"providers"`
	TaskStatuses   map[string]int64               `json:"task_statuses"`
	RecentSessions []*quotaNetNodeSessionResponse `json:"recent_sessions"`
}

type quotaNetSessionOverview struct {
	Total      int64            `json:"total"`
	Connected  int64            `json:"connected"`
	ByStatus   map[string]int64 `json:"by_status"`
	Ready      int64            `json:"ready"`
	Busy       int64            `json:"busy"`
	Offline    int64            `json:"offline"`
	Stale      int64            `json:"stale"`
	StaleAfter string           `json:"stale_after"`
}

type quotaNetCapacityOverview struct {
	CurrentConcurrency int64 `json:"current_concurrency"`
	MaxConcurrency     int64 `json:"max_concurrency"`
	Available          int64 `json:"available"`
	QueueSize          int64 `json:"queue_size"`
	MaxQueueSize       int64 `json:"max_queue_size"`
}

type quotaNetProviderOverview struct {
	Provider string   `json:"provider"`
	Models   []string `json:"models"`
}

func (h *QuotaNetNodeHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.manager.List(c.Request.Context(), nodes.ListParams{
		Page:     page,
		PageSize: pageSize,
		Status:   strings.TrimSpace(c.Query("status")),
		Search:   strings.TrimSpace(c.Query("search")),
	})
	if err != nil {
		quotaNetNodeError(c, err)
		return
	}
	out := make([]*quotaNetNodeResponse, 0, len(items))
	for _, item := range items {
		out = append(out, quotaNetNodeToResponse(item))
	}
	response.Paginated(c, out, total, page, pageSize)
}

func (h *QuotaNetNodeHandler) Sessions(c *gin.Context) {
	if h == nil || h.reg == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet registry is not initialized")
		return
	}
	sessions := h.reg.Snapshot()
	out := make([]*quotaNetNodeSessionResponse, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, quotaNetSessionToResponse(session))
	}
	response.Success(c, gin.H{"items": out})
}

func (h *QuotaNetNodeHandler) Overview(c *gin.Context) {
	if h == nil || h.reg == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet registry is not initialized")
		return
	}
	sessions := h.reg.Snapshot()
	taskStatuses := map[string]int64{}
	if h.taskStore != nil {
		counts, err := h.taskStore.StatusCounts(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "quotanet task overview failed")
			return
		}
		for _, item := range counts {
			taskStatuses[item.Status] = item.Count
		}
	}
	response.Success(c, quotaNetNodeOverview(sessions, taskStatuses))
}

func (h *QuotaNetNodeHandler) Get(c *gin.Context) {
	id, ok := quotaNetNodeID(c)
	if !ok {
		return
	}
	node, err := h.manager.GetByID(c.Request.Context(), id)
	if err != nil {
		quotaNetNodeError(c, err)
		return
	}
	response.Success(c, quotaNetNodeToResponse(node))
}

func (h *QuotaNetNodeHandler) Create(c *gin.Context) {
	var req quotaNetNodeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.manager.Create(c.Request.Context(), nodes.CreateInput{
		Name:          strings.TrimSpace(req.Name),
		WalletAddress: strings.TrimSpace(req.WalletAddress),
		OwnerUserID:   req.OwnerUserID,
		Status:        strings.TrimSpace(req.Status),
	})
	if err != nil {
		quotaNetNodeError(c, err)
		return
	}
	response.Created(c, quotaNetNodeCreateResponse{
		Node:  quotaNetNodeToResponse(result.Node),
		Token: result.Token,
	})
}

func (h *QuotaNetNodeHandler) UpdateStatus(c *gin.Context) {
	id, ok := quotaNetNodeID(c)
	if !ok {
		return
	}
	var req quotaNetNodeStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	node, err := h.manager.UpdateStatus(c.Request.Context(), id, nodes.UpdateStatusInput{
		Status: strings.TrimSpace(req.Status),
	})
	if err != nil {
		quotaNetNodeError(c, err)
		return
	}
	response.Success(c, quotaNetNodeToResponse(node))
}

func (h *QuotaNetNodeHandler) ResetToken(c *gin.Context) {
	id, ok := quotaNetNodeID(c)
	if !ok {
		return
	}
	result, err := h.manager.ResetToken(c.Request.Context(), id)
	if err != nil {
		quotaNetNodeError(c, err)
		return
	}
	response.Success(c, quotaNetNodeCreateResponse{
		Node:  quotaNetNodeToResponse(result.Node),
		Token: result.Token,
	})
}

func quotaNetNodeID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid node id")
		return 0, false
	}
	return id, true
}

func quotaNetNodeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, nodes.ErrInvalidNodeInput):
		response.BadRequest(c, "invalid quotanet node input")
	case errors.Is(err, nodes.ErrInvalidNodeStatus):
		response.BadRequest(c, "invalid quotanet node status")
	case errors.Is(err, nodes.ErrNodeNotFound):
		response.NotFound(c, "quotanet node not found")
	default:
		response.Error(c, http.StatusInternalServerError, "quotanet node operation failed")
	}
}

func quotaNetNodeToResponse(node *nodes.Node) *quotaNetNodeResponse {
	if node == nil {
		return nil
	}
	resp := &quotaNetNodeResponse{
		ID:            node.ID,
		NodeKey:       node.NodeKey,
		Name:          node.Name,
		OwnerUserID:   node.OwnerUserID,
		WalletAddress: node.WalletAddress,
		Status:        node.Status,
		CreatedAt:     formatQuotaNetTime(node.CreatedAt),
		UpdatedAt:     formatQuotaNetTime(node.UpdatedAt),
	}
	if node.LastSeenAt != nil {
		v := formatQuotaNetTime(*node.LastSeenAt)
		resp.LastSeenAt = &v
	}
	return resp
}

func quotaNetSessionToResponse(session registry.Session) *quotaNetNodeSessionResponse {
	resp := &quotaNetNodeSessionResponse{
		SessionID:          session.SessionID,
		NodeID:             session.NodeID,
		NodeKey:            session.NodeKey,
		InstanceID:         session.InstanceID,
		WalletAddress:      session.WalletAddress,
		ClientVersion:      session.ClientVersion,
		ProtocolVersion:    session.ProtocolVersion,
		Capabilities:       session.Capabilities,
		Status:             session.Status,
		CurrentConcurrency: session.CurrentConcurrency,
		MaxConcurrency:     session.MaxConcurrency,
		QueueSize:          session.QueueSize,
		MaxQueueSize:       session.MaxQueueSize,
		Accounts:           session.Accounts,
		ConnectedAt:        formatQuotaNetTime(session.ConnectedAt),
		LastHeartbeatAt:    formatQuotaNetTime(session.LastHeartbeatAt),
		CloseReason:        session.CloseReason,
	}
	if session.DisconnectedAt != nil {
		v := formatQuotaNetTime(*session.DisconnectedAt)
		resp.DisconnectedAt = &v
	}
	return resp
}

func quotaNetNodeOverview(sessions []registry.Session, taskStatuses map[string]int64) quotaNetNodeOverviewResponse {
	statuses := map[string]int64{}
	providerModels := map[string]map[string]struct{}{}
	capacity := quotaNetCapacityOverview{}
	recent := make([]*quotaNetNodeSessionResponse, 0, len(sessions))
	now := time.Now().UTC()
	const staleAfter = 60 * time.Second

	var connected, stale int64
	for _, session := range sessions {
		status := strings.TrimSpace(session.Status)
		if status == "" {
			status = protocol.NodeStatusReady
		}
		statuses[status]++
		if session.DisconnectedAt == nil {
			connected++
		}
		if session.DisconnectedAt == nil && !session.LastHeartbeatAt.IsZero() && now.Sub(session.LastHeartbeatAt) > staleAfter {
			stale++
		}
		capacity.CurrentConcurrency += int64(session.CurrentConcurrency)
		capacity.MaxConcurrency += int64(session.MaxConcurrency)
		available := session.MaxConcurrency - session.CurrentConcurrency
		if available > 0 {
			capacity.Available += int64(available)
		}
		capacity.QueueSize += int64(session.QueueSize)
		capacity.MaxQueueSize += int64(session.MaxQueueSize)
		for _, cap := range session.Capabilities {
			provider := strings.TrimSpace(cap.Provider)
			if provider == "" {
				continue
			}
			models, ok := providerModels[provider]
			if !ok {
				models = map[string]struct{}{}
				providerModels[provider] = models
			}
			for _, model := range cap.Models {
				model = strings.TrimSpace(model)
				if model != "" {
					models[model] = struct{}{}
				}
			}
		}
		recent = append(recent, quotaNetSessionToResponse(session))
	}

	return quotaNetNodeOverviewResponse{
		Sessions: quotaNetSessionOverview{
			Total:      int64(len(sessions)),
			Connected:  connected,
			ByStatus:   statuses,
			Ready:      statuses[protocol.NodeStatusReady],
			Busy:       statuses[protocol.NodeStatusBusy],
			Offline:    statuses[protocol.NodeStatusOffline],
			Stale:      stale,
			StaleAfter: staleAfter.String(),
		},
		Capacity:       capacity,
		Providers:      quotaNetProviderOverviewList(providerModels),
		TaskStatuses:   normalizeQuotaNetTaskStatuses(taskStatuses),
		RecentSessions: recent,
	}
}

func quotaNetProviderOverviewList(providerModels map[string]map[string]struct{}) []quotaNetProviderOverview {
	out := make([]quotaNetProviderOverview, 0, len(providerModels))
	for provider, models := range providerModels {
		list := make([]string, 0, len(models))
		for model := range models {
			list = append(list, model)
		}
		sortStringsCaseInsensitive(list)
		out = append(out, quotaNetProviderOverview{Provider: provider, Models: list})
	}
	for i := 1; i < len(out); i++ {
		current := out[i]
		j := i - 1
		for ; j >= 0 && strings.ToLower(current.Provider) < strings.ToLower(out[j].Provider); j-- {
			out[j+1] = out[j]
		}
		out[j+1] = current
	}
	return out
}

func normalizeQuotaNetTaskStatuses(counts map[string]int64) map[string]int64 {
	out := map[string]int64{
		protocol.TaskStatusQueued:    0,
		protocol.TaskStatusRunning:   0,
		protocol.TaskStatusSuccess:   0,
		protocol.TaskStatusFailed:    0,
		protocol.TaskStatusTimeout:   0,
		protocol.TaskStatusCancelled: 0,
	}
	for status, count := range counts {
		status = strings.TrimSpace(status)
		if status != "" {
			out[status] = count
		}
	}
	return out
}

func sortStringsCaseInsensitive(values []string) {
	for i := 1; i < len(values); i++ {
		current := values[i]
		j := i - 1
		for ; j >= 0 && strings.ToLower(current) < strings.ToLower(values[j]); j-- {
			values[j+1] = values[j]
		}
		values[j+1] = current
	}
}

func formatQuotaNetTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
