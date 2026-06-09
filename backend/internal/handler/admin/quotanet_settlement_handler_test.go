package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestQuotaNetSettlementListParamsParsesFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?status=pending&wallet_address=wallet-1&node_id=11&account_id=22&payout_batch_id=33", nil)

	params, ok := quotaNetSettlementListParams(c, 1, 20)
	if !ok {
		t.Fatal("quotaNetSettlementListParams() ok = false")
	}
	if params.Status != "pending" || params.WalletAddress != "wallet-1" {
		t.Fatalf("string filters = %+v", params)
	}
	if params.NodeID == nil || *params.NodeID != 11 {
		t.Fatalf("node_id = %v, want 11", params.NodeID)
	}
	if params.AccountID == nil || *params.AccountID != 22 {
		t.Fatalf("account_id = %v, want 22", params.AccountID)
	}
	if params.PayoutBatchID == nil || *params.PayoutBatchID != 33 {
		t.Fatalf("payout_batch_id = %v, want 33", params.PayoutBatchID)
	}
}

func TestQuotaNetSettlementListParamsRejectsInvalidFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?payout_batch_id=bad", nil)

	_, ok := quotaNetSettlementListParams(c, 1, 20)
	if ok {
		t.Fatal("quotaNetSettlementListParams() ok = true, want false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestQuotaNetPayoutBatchCreateInputParsesRFC3339Window(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{
		"batch_key":"batch-1",
		"window_start":"2026-06-01T00:00:00Z",
		"window_end":"2026-06-02T00:00:00Z",
		"network":"manual"
	}`)))
	c.Request.Header.Set("Content-Type", "application/json")

	input, ok := quotaNetPayoutBatchCreateInput(c)
	if !ok {
		t.Fatal("quotaNetPayoutBatchCreateInput() ok = false")
	}
	if input.BatchKey != "batch-1" || input.Network != "manual" {
		t.Fatalf("input strings = %+v", input)
	}
	if input.WindowStart.IsZero() || input.WindowEnd.IsZero() || !input.WindowEnd.After(input.WindowStart) {
		t.Fatalf("input window = %s - %s", input.WindowStart, input.WindowEnd)
	}
}

func TestQuotaNetPayoutBatchCreateInputRejectsInvalidWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{
		"window_start":"not-time",
		"window_end":"2026-06-02T00:00:00Z"
	}`)))
	c.Request.Header.Set("Content-Type", "application/json")

	_, ok := quotaNetPayoutBatchCreateInput(c)
	if ok {
		t.Fatal("quotaNetPayoutBatchCreateInput() ok = true, want false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
