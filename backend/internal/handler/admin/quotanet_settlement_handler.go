package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/settlements"

	"github.com/gin-gonic/gin"
)

type QuotaNetSettlementHandler struct {
	store    *settlements.Store
	registry *registry.Registry
}

func NewQuotaNetSettlementHandler(store *settlements.Store, regs ...*registry.Registry) *QuotaNetSettlementHandler {
	var reg *registry.Registry
	if len(regs) > 0 {
		reg = regs[0]
	}
	return &QuotaNetSettlementHandler{store: store, registry: reg}
}

type quotaNetContributionLedgerResponse struct {
	ID              int64   `json:"id"`
	TaskID          string  `json:"task_id"`
	UsageLogID      *int64  `json:"usage_log_id,omitempty"`
	NodeID          int64   `json:"node_id"`
	WalletAddress   string  `json:"wallet_address"`
	AccountID       *int64  `json:"account_id,omitempty"`
	Platform        string  `json:"platform"`
	Model           string  `json:"model"`
	TokenFlow       int64   `json:"token_flow"`
	StandardCostUSD float64 `json:"standard_cost_usd"`
	ActualCostUSD   float64 `json:"actual_cost_usd"`
	ContributionUSD float64 `json:"contribution_usd"`
	AmountCxs       float64 `json:"amount_cxs"`
	Rate            float64 `json:"rate"`
	Status          string  `json:"status"`
	PayoutBatchID   *int64  `json:"payout_batch_id,omitempty"`
	SettledAt       *string `json:"settled_at,omitempty"`
	CreatedAt       string  `json:"created_at,omitempty"`
	UpdatedAt       string  `json:"updated_at,omitempty"`
}

type quotaNetPayoutBatchCreateRequest struct {
	BatchKey    string `json:"batch_key"`
	WindowStart string `json:"window_start" binding:"required"`
	WindowEnd   string `json:"window_end" binding:"required"`
	Network     string `json:"network"`
	CreatedBy   *int64 `json:"created_by"`
	ApprovedBy  *int64 `json:"approved_by"`
}

type quotaNetSettlementConfigRequest struct {
	Network string `json:"network"`
}

type quotaNetPayoutItemStatusRequest struct {
	Status       string `json:"status" binding:"required"`
	TxHash       string `json:"tx_hash"`
	ErrorMessage string `json:"error_message"`
}

type quotaNetPayoutBatchResponse struct {
	ID                   int64   `json:"id"`
	BatchKey             string  `json:"batch_key"`
	WindowStart          string  `json:"window_start,omitempty"`
	WindowEnd            string  `json:"window_end,omitempty"`
	Status               string  `json:"status"`
	Network              string  `json:"network"`
	TotalTokenFlow       int64   `json:"total_token_flow"`
	TotalContributionUSD float64 `json:"total_contribution_usd"`
	TotalAmountCxs       float64 `json:"total_amount_cxs"`
	ItemCount            int     `json:"item_count"`
	CreatedBy            *int64  `json:"created_by,omitempty"`
	ApprovedBy           *int64  `json:"approved_by,omitempty"`
	CreatedAt            string  `json:"created_at,omitempty"`
	UpdatedAt            string  `json:"updated_at,omitempty"`
}

type quotaNetPayoutItemResponse struct {
	ID              int64   `json:"id"`
	ItemKey         string  `json:"item_key"`
	BatchID         int64   `json:"batch_id"`
	Network         string  `json:"network,omitempty"`
	NodeID          *int64  `json:"node_id,omitempty"`
	WalletAddress   string  `json:"wallet_address"`
	TokenFlow       int64   `json:"token_flow"`
	ContributionUSD float64 `json:"contribution_usd"`
	AmountCxs       float64 `json:"amount_cxs"`
	Status          string  `json:"status"`
	TxHash          *string `json:"tx_hash,omitempty"`
	TxURL           string  `json:"tx_url,omitempty"`
	ErrorMessage    *string `json:"error_message,omitempty"`
	FinalizedAt     *string `json:"finalized_at,omitempty"`
	CreatedAt       string  `json:"created_at,omitempty"`
	UpdatedAt       string  `json:"updated_at,omitempty"`
}

type quotaNetPayoutBatchCreateResponse struct {
	Batch       *quotaNetPayoutBatchResponse  `json:"batch"`
	Items       []*quotaNetPayoutItemResponse `json:"items"`
	LedgerCount int                           `json:"ledger_count"`
}

func (h *QuotaNetSettlementHandler) Config(c *gin.Context) {
	if h == nil || h.store == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet settlement service is not initialized")
		return
	}
	cfg, err := h.store.GetConfig(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "quotanet settlement operation failed")
		return
	}
	response.Success(c, cfg)
}

func (h *QuotaNetSettlementHandler) UpdateConfig(c *gin.Context) {
	if h == nil || h.store == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet settlement service is not initialized")
		return
	}
	var req quotaNetSettlementConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.store.UpdateConfig(c.Request.Context(), settlements.Config{
		Network: strings.TrimSpace(req.Network),
	})
	if err != nil {
		quotaNetSettlementError(c, err)
		return
	}
	response.Success(c, cfg)
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

func (h *QuotaNetSettlementHandler) Batches(c *gin.Context) {
	if h == nil || h.store == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet settlement service is not initialized")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.store.ListBatches(c.Request.Context(), settlements.BatchListParams{
		Page:     page,
		PageSize: pageSize,
		Status:   strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "quotanet settlement operation failed")
		return
	}
	out := make([]*quotaNetPayoutBatchResponse, 0, len(items))
	for _, item := range items {
		out = append(out, quotaNetPayoutBatchToResponse(item))
	}
	response.Paginated(c, out, total, page, pageSize)
}

func (h *QuotaNetSettlementHandler) BatchItems(c *gin.Context) {
	if h == nil || h.store == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet settlement service is not initialized")
		return
	}
	batchID, ok := requiredPositiveInt64(c, c.Param("id"), "batch id")
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.store.ListItems(c.Request.Context(), settlements.ItemListParams{
		Page:          page,
		PageSize:      pageSize,
		BatchID:       batchID,
		Status:        strings.TrimSpace(c.Query("status")),
		WalletAddress: strings.TrimSpace(c.Query("wallet_address")),
		TxHash:        strings.TrimSpace(c.Query("tx_hash")),
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "quotanet settlement operation failed")
		return
	}
	out := make([]*quotaNetPayoutItemResponse, 0, len(items))
	for _, item := range items {
		out = append(out, quotaNetPayoutItemToResponse(item))
	}
	response.Paginated(c, out, total, page, pageSize)
}

func (h *QuotaNetSettlementHandler) CreateBatch(c *gin.Context) {
	if h == nil || h.store == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet settlement service is not initialized")
		return
	}
	input, ok := quotaNetPayoutBatchCreateInput(c)
	if !ok {
		return
	}
	result, err := h.store.CreateBatch(c.Request.Context(), input)
	if err != nil {
		quotaNetSettlementError(c, err)
		return
	}
	items := make([]*quotaNetPayoutItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, quotaNetPayoutItemToResponse(item))
	}
	response.Created(c, quotaNetPayoutBatchCreateResponse{
		Batch:       quotaNetPayoutBatchToResponse(result.Batch),
		Items:       items,
		LedgerCount: result.LedgerCount,
	})
}

func (h *QuotaNetSettlementHandler) UpdateItemStatus(c *gin.Context) {
	if h == nil || h.store == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet settlement service is not initialized")
		return
	}
	itemID, ok := requiredPositiveInt64(c, c.Param("id"), "item id")
	if !ok {
		return
	}
	var req quotaNetPayoutItemStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.store.UpdateItemStatus(c.Request.Context(), itemID, settlements.UpdateItemStatusInput{
		Status:       strings.TrimSpace(req.Status),
		TxHash:       strings.TrimSpace(req.TxHash),
		ErrorMessage: strings.TrimSpace(req.ErrorMessage),
	})
	if err != nil {
		quotaNetSettlementError(c, err)
		return
	}
	h.notifySettlement(c.Request.Context(), item)
	response.Success(c, quotaNetPayoutItemToResponse(item))
}

func (h *QuotaNetSettlementHandler) notifySettlement(ctx context.Context, item *settlements.PayoutItem) {
	if h == nil || h.registry == nil || item == nil {
		return
	}
	notice := protocol.SettlementNotice{
		ID:        item.ItemKey,
		AmountCXS: strconv.FormatFloat(item.AmountCxs, 'f', -1, 64),
		TokenFlow: item.TokenFlow,
		Status:    item.Status,
		CreatedAt: formatQuotaNetTime(item.CreatedAt),
		UpdatedAt: formatQuotaNetTime(item.UpdatedAt),
	}
	if item.TxHash != nil {
		notice.TxHash = *item.TxHash
	}
	envelope, err := protocol.NewEnvelope(protocol.EventSettlementNotice, "qns_"+strconv.FormatInt(item.ID, 10), notice)
	if err != nil {
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for _, session := range h.registry.Snapshot() {
		if strings.TrimSpace(session.WalletAddress) != strings.TrimSpace(item.WalletAddress) {
			continue
		}
		if item.NodeID != nil && session.NodeID != *item.NodeID {
			continue
		}
		_ = h.registry.Send(sendCtx, session.SessionID, envelope)
	}
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

func requiredPositiveInt64(c *gin.Context, value, field string) (int64, bool) {
	value = strings.TrimSpace(value)
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid "+field)
		return 0, false
	}
	return id, true
}

func quotaNetPayoutBatchCreateInput(c *gin.Context) (settlements.CreateBatchInput, bool) {
	var req quotaNetPayoutBatchCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return settlements.CreateBatchInput{}, false
	}
	windowStart, ok := parseQuotaNetSettlementTime(c, req.WindowStart, "window_start")
	if !ok {
		return settlements.CreateBatchInput{}, false
	}
	windowEnd, ok := parseQuotaNetSettlementTime(c, req.WindowEnd, "window_end")
	if !ok {
		return settlements.CreateBatchInput{}, false
	}
	return settlements.CreateBatchInput{
		BatchKey:    strings.TrimSpace(req.BatchKey),
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Network:     strings.TrimSpace(req.Network),
		CreatedBy:   req.CreatedBy,
		ApprovedBy:  req.ApprovedBy,
	}, true
}

func parseQuotaNetSettlementTime(c *gin.Context, value, field string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		response.BadRequest(c, field+" is required")
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		response.BadRequest(c, "invalid "+field)
		return time.Time{}, false
	}
	return t, true
}

func quotaNetSettlementError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, settlements.ErrInvalidBatchInput):
		response.BadRequest(c, "invalid quotanet settlement batch input")
	case errors.Is(err, settlements.ErrNoPendingLedger):
		response.BadRequest(c, "no pending quotanet contribution ledger")
	default:
		response.Error(c, http.StatusInternalServerError, "quotanet settlement operation failed")
	}
}

func quotaNetContributionLedgerToResponse(item *settlements.Ledger) *quotaNetContributionLedgerResponse {
	if item == nil {
		return nil
	}
	return &quotaNetContributionLedgerResponse{
		ID:              item.ID,
		TaskID:          item.TaskID,
		UsageLogID:      item.UsageLogID,
		NodeID:          item.NodeID,
		WalletAddress:   item.WalletAddress,
		AccountID:       item.AccountID,
		Platform:        item.Platform,
		Model:           item.Model,
		TokenFlow:       item.TokenFlow,
		StandardCostUSD: item.StandardCostUSD,
		ActualCostUSD:   item.ActualCostUSD,
		ContributionUSD: item.ContributionUSD,
		AmountCxs:       item.AmountCxs,
		Rate:            item.Rate,
		Status:          item.Status,
		PayoutBatchID:   item.PayoutBatchID,
		SettledAt:       quotaNetOptionalTime(item.SettledAt),
		CreatedAt:       formatQuotaNetTime(item.CreatedAt),
		UpdatedAt:       formatQuotaNetTime(item.UpdatedAt),
	}
}

func quotaNetPayoutBatchToResponse(batch *settlements.PayoutBatch) *quotaNetPayoutBatchResponse {
	if batch == nil {
		return nil
	}
	return &quotaNetPayoutBatchResponse{
		ID:                   batch.ID,
		BatchKey:             batch.BatchKey,
		WindowStart:          formatQuotaNetTime(batch.WindowStart),
		WindowEnd:            formatQuotaNetTime(batch.WindowEnd),
		Status:               batch.Status,
		Network:              batch.Network,
		TotalTokenFlow:       batch.TotalTokenFlow,
		TotalContributionUSD: batch.TotalContributionUSD,
		TotalAmountCxs:       batch.TotalAmountCxs,
		ItemCount:            batch.ItemCount,
		CreatedBy:            batch.CreatedBy,
		ApprovedBy:           batch.ApprovedBy,
		CreatedAt:            formatQuotaNetTime(batch.CreatedAt),
		UpdatedAt:            formatQuotaNetTime(batch.UpdatedAt),
	}
}

func quotaNetPayoutItemToResponse(item *settlements.PayoutItem) *quotaNetPayoutItemResponse {
	if item == nil {
		return nil
	}
	return &quotaNetPayoutItemResponse{
		ID:              item.ID,
		ItemKey:         item.ItemKey,
		BatchID:         item.BatchID,
		Network:         item.Network,
		NodeID:          item.NodeID,
		WalletAddress:   item.WalletAddress,
		TokenFlow:       item.TokenFlow,
		ContributionUSD: item.ContributionUSD,
		AmountCxs:       item.AmountCxs,
		Status:          item.Status,
		TxHash:          item.TxHash,
		TxURL:           quotaNetPayoutItemTxURL(item),
		ErrorMessage:    item.ErrorMessage,
		FinalizedAt:     quotaNetOptionalTime(item.FinalizedAt),
		CreatedAt:       formatQuotaNetTime(item.CreatedAt),
		UpdatedAt:       formatQuotaNetTime(item.UpdatedAt),
	}
}

func quotaNetPayoutItemTxURL(item *settlements.PayoutItem) string {
	if item == nil || item.TxHash == nil {
		return ""
	}
	return settlements.ExplorerTxURL(item.Network, *item.TxHash)
}
