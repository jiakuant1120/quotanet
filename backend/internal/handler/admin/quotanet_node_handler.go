package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/nodes"

	"github.com/gin-gonic/gin"
)

type QuotaNetNodeHandler struct {
	manager *nodes.Manager
}

func NewQuotaNetNodeHandler(manager *nodes.Manager) *QuotaNetNodeHandler {
	return &QuotaNetNodeHandler{manager: manager}
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

func formatQuotaNetTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
