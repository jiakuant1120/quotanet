package admin

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/settlements"

	"github.com/gin-gonic/gin"
)

type QuotaNetSettlementHandler struct {
	store *settlements.Store
}

func NewQuotaNetSettlementHandler(store *settlements.Store) *QuotaNetSettlementHandler {
	return &QuotaNetSettlementHandler{store: store}
}

type quotaNetContributionLedgerResponse struct {
	ID            int64   `json:"id"`
	TaskID        string  `json:"task_id"`
	UsageLogID    *int64  `json:"usage_log_id,omitempty"`
	NodeID        int64   `json:"node_id"`
	WalletAddress string  `json:"wallet_address"`
	AccountID     *int64  `json:"account_id,omitempty"`
	Platform      string  `json:"platform"`
	Model         string  `json:"model"`
	TokenFlow     int64   `json:"token_flow"`
	AmountCxs     float64 `json:"amount_cxs"`
	Rate          float64 `json:"rate"`
	Status        string  `json:"status"`
	PayoutBatchID *int64  `json:"payout_batch_id,omitempty"`
	SettledAt     *string `json:"settled_at,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
	UpdatedAt     string  `json:"updated_at,omitempty"`
}

func (h *QuotaNetSettlementHandler) Ledgers(c *gin.Context) {
	if h == nil || h.store == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet settlement service is not initialized")
		return
	}
	page, pageSize := response.ParsePagination(c)
	params, ok := quotaNetSettlementListParams(c, page, pageSize)
	if !ok {
		return
	}
	items, total, err := h.store.List(c.Request.Context(), params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "quotanet settlement operation failed")
		return
	}
	out := make([]*quotaNetContributionLedgerResponse, 0, len(items))
	for _, item := range items {
		out = append(out, quotaNetContributionLedgerToResponse(item))
	}
	response.Paginated(c, out, total, page, pageSize)
}

func (h *QuotaNetSettlementHandler) Summary(c *gin.Context) {
	if h == nil || h.store == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet settlement service is not initialized")
		return
	}
	params, ok := quotaNetSettlementListParams(c, 1, 1)
	if !ok {
		return
	}
	summary, err := h.store.Summary(c.Request.Context(), params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "quotanet settlement operation failed")
		return
	}
	response.Success(c, summary)
}

func (h *QuotaNetSettlementHandler) WalletSummaries(c *gin.Context) {
	if h == nil || h.store == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet settlement service is not initialized")
		return
	}
	params, ok := quotaNetSettlementListParams(c, 1, 1)
	if !ok {
		return
	}
	items, err := h.store.WalletSummaries(c.Request.Context(), params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "quotanet settlement operation failed")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func quotaNetSettlementListParams(c *gin.Context, page, pageSize int) (settlements.ListParams, bool) {
	params := settlements.ListParams{
		Page:          page,
		PageSize:      pageSize,
		Status:        strings.TrimSpace(c.Query("status")),
		WalletAddress: strings.TrimSpace(c.Query("wallet_address")),
	}
	var ok bool
	if params.NodeID, ok = optionalInt64Query(c, "node_id"); !ok {
		return settlements.ListParams{}, false
	}
	if params.AccountID, ok = optionalInt64Query(c, "account_id"); !ok {
		return settlements.ListParams{}, false
	}
	if params.PayoutBatchID, ok = optionalInt64Query(c, "payout_batch_id"); !ok {
		return settlements.ListParams{}, false
	}
	return params, true
}

func quotaNetContributionLedgerToResponse(item *settlements.Ledger) *quotaNetContributionLedgerResponse {
	if item == nil {
		return nil
	}
	return &quotaNetContributionLedgerResponse{
		ID:            item.ID,
		TaskID:        item.TaskID,
		UsageLogID:    item.UsageLogID,
		NodeID:        item.NodeID,
		WalletAddress: item.WalletAddress,
		AccountID:     item.AccountID,
		Platform:      item.Platform,
		Model:         item.Model,
		TokenFlow:     item.TokenFlow,
		AmountCxs:     item.AmountCxs,
		Rate:          item.Rate,
		Status:        item.Status,
		PayoutBatchID: item.PayoutBatchID,
		SettledAt:     quotaNetOptionalTime(item.SettledAt),
		CreatedAt:     formatQuotaNetTime(item.CreatedAt),
		UpdatedAt:     formatQuotaNetTime(item.UpdatedAt),
	}
}
