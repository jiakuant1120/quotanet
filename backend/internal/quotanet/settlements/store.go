// Package settlements provides QuotaNet contribution ledger queries.
package settlements

import (
	"context"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/quotanetcontributionledger"
)

type Store struct {
	client *ent.Client
}

func NewStore(client *ent.Client) *Store {
	return &Store{client: client}
}

type Ledger struct {
	ID            int64
	TaskID        string
	UsageLogID    *int64
	NodeID        int64
	WalletAddress string
	AccountID     *int64
	Platform      string
	Model         string
	TokenFlow     int64
	AmountCxs     float64
	Rate          float64
	Status        string
	PayoutBatchID *int64
	SettledAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type WalletSummary struct {
	WalletAddress string  `json:"wallet_address"`
	LedgerCount   int64   `json:"ledger_count"`
	TokenFlow     int64   `json:"token_flow"`
	AmountCxs     float64 `json:"amount_cxs"`
}

type Summary struct {
	LedgerCount int64   `json:"ledger_count"`
	TokenFlow   int64   `json:"token_flow"`
	AmountCxs   float64 `json:"amount_cxs"`
}

type ListParams struct {
	Page          int
	PageSize      int
	Status        string
	WalletAddress string
	NodeID        *int64
	AccountID     *int64
	PayoutBatchID *int64
}

func (s *Store) List(ctx context.Context, params ListParams) ([]*Ledger, int64, error) {
	if s == nil || s.client == nil {
		return nil, 0, nil
	}
	params = normalizeListParams(params)
	query := applyLedgerFilters(s.client.QuotaNetContributionLedger.Query(), params)
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := query.
		Order(quotanetcontributionledger.ByCreatedAt(sql.OrderDesc())).
		Limit(params.PageSize).
		Offset((params.Page - 1) * params.PageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*Ledger, 0, len(rows))
	for _, row := range rows {
		out = append(out, ledgerFromEnt(row))
	}
	return out, int64(total), nil
}

func (s *Store) Summary(ctx context.Context, params ListParams) (*Summary, error) {
	if s == nil || s.client == nil {
		return &Summary{}, nil
	}
	params = normalizeListParams(params)
	var rows []struct {
		LedgerCount int64   `json:"ledger_count"`
		TokenFlow   int64   `json:"token_flow"`
		AmountCxs   float64 `json:"amount_cxs"`
	}
	err := applyLedgerFilters(s.client.QuotaNetContributionLedger.Query(), params).
		Select(
			sql.As(sql.Count("*"), "ledger_count"),
			sql.As(sql.Sum(quotanetcontributionledger.FieldTokenFlow), "token_flow"),
			sql.As(sql.Sum(quotanetcontributionledger.FieldAmountCxs), "amount_cxs"),
		).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return &Summary{}, nil
	}
	return &Summary{
		LedgerCount: rows[0].LedgerCount,
		TokenFlow:   rows[0].TokenFlow,
		AmountCxs:   rows[0].AmountCxs,
	}, nil
}

func (s *Store) WalletSummaries(ctx context.Context, params ListParams) ([]*WalletSummary, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	params = normalizeListParams(params)
	var rows []*WalletSummary
	err := applyLedgerFilters(s.client.QuotaNetContributionLedger.Query(), params).
		GroupBy(quotanetcontributionledger.FieldWalletAddress).
		Aggregate(
			ent.As(ent.Count(), "ledger_count"),
			ent.As(ent.Sum(quotanetcontributionledger.FieldTokenFlow), "token_flow"),
			ent.As(ent.Sum(quotanetcontributionledger.FieldAmountCxs), "amount_cxs"),
		).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func normalizeListParams(params ListParams) ListParams {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	params.Status = strings.TrimSpace(params.Status)
	params.WalletAddress = strings.TrimSpace(params.WalletAddress)
	return params
}

func applyLedgerFilters(query *ent.QuotaNetContributionLedgerQuery, params ListParams) *ent.QuotaNetContributionLedgerQuery {
	if params.Status != "" {
		query = query.Where(quotanetcontributionledger.StatusEQ(params.Status))
	}
	if params.WalletAddress != "" {
		query = query.Where(quotanetcontributionledger.WalletAddressEQ(params.WalletAddress))
	}
	if params.NodeID != nil {
		query = query.Where(quotanetcontributionledger.NodeIDEQ(*params.NodeID))
	}
	if params.AccountID != nil {
		query = query.Where(quotanetcontributionledger.AccountIDEQ(*params.AccountID))
	}
	if params.PayoutBatchID != nil {
		query = query.Where(quotanetcontributionledger.PayoutBatchIDEQ(*params.PayoutBatchID))
	}
	return query
}

func ledgerFromEnt(row *ent.QuotaNetContributionLedger) *Ledger {
	if row == nil {
		return nil
	}
	return &Ledger{
		ID:            row.ID,
		TaskID:        row.TaskID,
		UsageLogID:    row.UsageLogID,
		NodeID:        row.NodeID,
		WalletAddress: row.WalletAddress,
		AccountID:     row.AccountID,
		Platform:      row.Platform,
		Model:         row.Model,
		TokenFlow:     row.TokenFlow,
		AmountCxs:     row.AmountCxs,
		Rate:          row.Rate,
		Status:        row.Status,
		PayoutBatchID: row.PayoutBatchID,
		SettledAt:     row.SettledAt,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}
