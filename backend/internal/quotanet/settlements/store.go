// Package settlements provides QuotaNet contribution ledger queries.
package settlements

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/quotanetcontributionledger"
	"github.com/Wei-Shaw/sub2api/ent/quotanetpayoutbatch"
	"github.com/Wei-Shaw/sub2api/ent/quotanetpayoutitem"
	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/google/uuid"
)

var (
	ErrInvalidBatchInput = errors.New("invalid quotanet settlement batch input")
	ErrNoPendingLedger   = errors.New("no pending quotanet contribution ledger")
)

const (
	defaultSettlementNetwork = "manual"

	settingKeySettlementNetwork = "quotanet.settlement.network"
)

type Store struct {
	client *ent.Client
}

func NewStore(client *ent.Client) *Store {
	return &Store{client: client}
}

type Ledger struct {
	ID              int64
	TaskID          string
	UsageLogID      *int64
	NodeID          int64
	WalletAddress   string
	AccountID       *int64
	Platform        string
	Model           string
	TokenFlow       int64
	StandardCostUSD float64
	ActualCostUSD   float64
	ContributionUSD float64
	AmountCxs       float64
	Rate            float64
	Status          string
	PayoutBatchID   *int64
	SettledAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type WalletSummary struct {
	WalletAddress   string  `json:"wallet_address"`
	LedgerCount     int64   `json:"ledger_count"`
	TokenFlow       int64   `json:"token_flow"`
	ContributionUSD float64 `json:"contribution_usd"`
	AmountCxs       float64 `json:"amount_cxs"`
}

type Summary struct {
	LedgerCount     int64   `json:"ledger_count"`
	TokenFlow       int64   `json:"token_flow"`
	ContributionUSD float64 `json:"contribution_usd"`
	AmountCxs       float64 `json:"amount_cxs"`
}

type Config struct {
	Network string `json:"network"`
}

type PayoutBatch struct {
	ID                   int64
	BatchKey             string
	WindowStart          time.Time
	WindowEnd            time.Time
	Status               string
	Network              string
	TotalTokenFlow       int64
	TotalContributionUSD float64
	TotalAmountCxs       float64
	ItemCount            int
	CreatedBy            *int64
	ApprovedBy           *int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type PayoutItem struct {
	ID              int64
	ItemKey         string
	BatchID         int64
	Network         string
	NodeID          *int64
	WalletAddress   string
	TokenFlow       int64
	ContributionUSD float64
	AmountCxs       float64
	Status          string
	TxHash          *string
	ErrorMessage    *string
	FinalizedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateBatchInput struct {
	BatchKey    string
	WindowStart time.Time
	WindowEnd   time.Time
	Network     string
	CreatedBy   *int64
	ApprovedBy  *int64
}

type UpdateItemStatusInput struct {
	Status       string
	TxHash       string
	ErrorMessage string
}

type CreateBatchResult struct {
	Batch       *PayoutBatch
	Items       []*PayoutItem
	LedgerCount int
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

type BatchListParams struct {
	Page     int
	PageSize int
	Status   string
}

type ItemListParams struct {
	Page          int
	PageSize      int
	BatchID       int64
	Status        string
	WalletAddress string
	TxHash        string
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

func (s *Store) ListBatches(ctx context.Context, params BatchListParams) ([]*PayoutBatch, int64, error) {
	if s == nil || s.client == nil {
		return nil, 0, nil
	}
	params = normalizeBatchListParams(params)
	query := s.client.QuotaNetPayoutBatch.Query()
	if params.Status != "" {
		query = query.Where(quotanetpayoutbatch.StatusEQ(params.Status))
	}
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := query.
		Order(quotanetpayoutbatch.ByCreatedAt(sql.OrderDesc())).
		Limit(params.PageSize).
		Offset((params.Page - 1) * params.PageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*PayoutBatch, 0, len(rows))
	for _, row := range rows {
		out = append(out, payoutBatchFromEnt(row))
	}
	return out, int64(total), nil
}

func (s *Store) ListItems(ctx context.Context, params ItemListParams) ([]*PayoutItem, int64, error) {
	if s == nil || s.client == nil {
		return nil, 0, nil
	}
	params = normalizeItemListParams(params)
	query := s.client.QuotaNetPayoutItem.Query().
		Where(quotanetpayoutitem.BatchIDEQ(params.BatchID))
	if params.Status != "" {
		query = query.Where(quotanetpayoutitem.StatusEQ(params.Status))
	}
	if params.WalletAddress != "" {
		query = query.Where(quotanetpayoutitem.WalletAddressEQ(params.WalletAddress))
	}
	if params.TxHash != "" {
		query = query.Where(quotanetpayoutitem.TxHashContainsFold(params.TxHash))
	}
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := query.
		Order(quotanetpayoutitem.ByCreatedAt(sql.OrderAsc())).
		Limit(params.PageSize).
		Offset((params.Page - 1) * params.PageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*PayoutItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, payoutItemFromEnt(row))
	}
	if err := s.attachItemNetworks(ctx, out); err != nil {
		return nil, 0, err
	}
	return out, int64(total), nil
}

func (s *Store) UpdateItemStatus(ctx context.Context, id int64, input UpdateItemStatusInput) (*PayoutItem, error) {
	if s == nil || s.client == nil || id <= 0 {
		return nil, ErrInvalidBatchInput
	}
	input = normalizeUpdateItemStatusInput(input)
	if err := validateUpdateItemStatusInput(input); err != nil {
		return nil, err
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	item, err := tx.QuotaNetPayoutItem.Query().
		Where(quotanetpayoutitem.IDEQ(id)).
		Only(ctx)
	if err != nil {
		return nil, err
	}

	update := tx.QuotaNetPayoutItem.UpdateOneID(id).
		SetStatus(input.Status)
	switch input.Status {
	case protocol.SettlementStatusFinalized:
		update.SetTxHash(input.TxHash)
		update.ClearErrorMessage()
		update.SetFinalizedAt(time.Now().UTC())
	case protocol.SettlementStatusFailed:
		update.ClearTxHash()
		update.SetErrorMessage(input.ErrorMessage)
		update.ClearFinalizedAt()
	case protocol.SettlementStatusPending:
		update.ClearTxHash()
		update.ClearErrorMessage()
		update.ClearFinalizedAt()
	}
	row, err := update.Save(ctx)
	if err != nil {
		return nil, err
	}

	ledgerUpdate := tx.QuotaNetContributionLedger.Update().
		Where(
			quotanetcontributionledger.PayoutBatchIDEQ(item.BatchID),
			quotanetcontributionledger.WalletAddressEQ(item.WalletAddress),
		).
		SetStatus(input.Status)
	switch input.Status {
	case protocol.SettlementStatusFinalized:
		ledgerUpdate.SetSettledAt(time.Now().UTC())
	default:
		ledgerUpdate.ClearSettledAt()
	}
	if _, err := ledgerUpdate.Save(ctx); err != nil {
		return nil, err
	}

	if err := updateBatchStatus(ctx, tx, item.BatchID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	out := payoutItemFromEnt(row)
	if out != nil {
		out.Network = s.payoutNetwork(ctx, out.BatchID)
	}
	return out, nil
}

func (s *Store) GetConfig(ctx context.Context) (*Config, error) {
	if s == nil || s.client == nil {
		return defaultConfig(), nil
	}
	cfg := defaultConfig()
	if row, err := s.client.Setting.Query().Where(setting.KeyEQ(settingKeySettlementNetwork)).Only(ctx); err == nil {
		if v := strings.TrimSpace(row.Value); v != "" {
			cfg.Network = v
		}
	} else if !ent.IsNotFound(err) {
		return nil, err
	}
	return cfg, nil
}

func (s *Store) UpdateConfig(ctx context.Context, input Config) (*Config, error) {
	if s == nil || s.client == nil {
		return nil, ErrInvalidBatchInput
	}
	input.Network = strings.TrimSpace(input.Network)
	if input.Network == "" {
		input.Network = defaultSettlementNetwork
	}
	now := time.Now().UTC()
	if err := s.client.Setting.Create().
		SetKey(settingKeySettlementNetwork).
		SetValue(input.Network).
		SetUpdatedAt(now).
		OnConflictColumns(setting.FieldKey).
		UpdateNewValues().
		Exec(ctx); err != nil {
		return nil, err
	}
	return s.GetConfig(ctx)
}

func (s *Store) Summary(ctx context.Context, params ListParams) (*Summary, error) {
	if s == nil || s.client == nil {
		return &Summary{}, nil
	}
	params = normalizeListParams(params)
	where, args := ledgerSQLWhere(params)
	query := `SELECT
		COUNT(*) AS ledger_count,
		COALESCE(SUM(token_flow), 0) AS token_flow,
		COALESCE(SUM(contribution_usd), 0) AS contribution_usd,
		COALESCE(SUM(amount_cxs), 0) AS amount_cxs
	FROM quotanet_contribution_ledger` + where
	rows, err := s.client.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return &Summary{}, nil
	}
	var summary Summary
	if err := rows.Scan(&summary.LedgerCount, &summary.TokenFlow, &summary.ContributionUSD, &summary.AmountCxs); err != nil {
		return nil, err
	}
	return &summary, rows.Err()
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
			ent.As(ent.Sum(quotanetcontributionledger.FieldContributionUsd), "contribution_usd"),
			ent.As(ent.Sum(quotanetcontributionledger.FieldAmountCxs), "amount_cxs"),
		).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) CreateBatch(ctx context.Context, input CreateBatchInput) (*CreateBatchResult, error) {
	if s == nil || s.client == nil {
		return nil, ErrInvalidBatchInput
	}
	input = normalizeCreateBatchInput(input)
	if strings.TrimSpace(input.Network) == "" || input.Network == defaultSettlementNetwork {
		cfg, err := s.GetConfig(ctx)
		if err != nil {
			return nil, err
		}
		input.Network = cfg.Network
		input = normalizeCreateBatchInput(input)
	}
	if err := validateCreateBatchInput(input); err != nil {
		return nil, err
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	ledgers, err := tx.QuotaNetContributionLedger.Query().
		Where(
			quotanetcontributionledger.StatusEQ(protocol.SettlementStatusPending),
			quotanetcontributionledger.PayoutBatchIDIsNil(),
			quotanetcontributionledger.CreatedAtGTE(input.WindowStart),
			quotanetcontributionledger.CreatedAtLT(input.WindowEnd),
		).
		Order(quotanetcontributionledger.ByWalletAddress(sql.OrderAsc())).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if len(ledgers) == 0 {
		return nil, ErrNoPendingLedger
	}

	wallets := buildWalletPayouts(ledgers)
	var totalTokenFlow int64
	var totalContributionUSD float64
	var totalAmountCxs float64
	for _, item := range wallets {
		totalTokenFlow += item.TokenFlow
		totalContributionUSD += item.ContributionUSD
		totalAmountCxs += item.AmountCxs
	}

	batchRow, err := tx.QuotaNetPayoutBatch.Create().
		SetBatchKey(input.BatchKey).
		SetWindowStart(input.WindowStart).
		SetWindowEnd(input.WindowEnd).
		SetStatus(protocol.SettlementStatusPending).
		SetNetwork(input.Network).
		SetTotalTokenFlow(totalTokenFlow).
		SetTotalContributionUsd(totalContributionUSD).
		SetTotalAmountCxs(totalAmountCxs).
		SetItemCount(len(wallets)).
		SetNillableCreatedBy(input.CreatedBy).
		SetNillableApprovedBy(input.ApprovedBy).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	itemBuilders := make([]*ent.QuotaNetPayoutItemCreate, 0, len(wallets))
	for _, wallet := range wallets {
		itemBuilders = append(itemBuilders, tx.QuotaNetPayoutItem.Create().
			SetItemKey("qni_"+strings.ReplaceAll(uuid.NewString(), "-", "")).
			SetBatchID(batchRow.ID).
			SetNillableNodeID(wallet.NodeID).
			SetWalletAddress(wallet.WalletAddress).
			SetTokenFlow(wallet.TokenFlow).
			SetContributionUsd(wallet.ContributionUSD).
			SetAmountCxs(wallet.AmountCxs).
			SetStatus(protocol.SettlementStatusPending))
	}
	itemRows, err := tx.QuotaNetPayoutItem.CreateBulk(itemBuilders...).Save(ctx)
	if err != nil {
		return nil, err
	}

	for _, ledger := range ledgers {
		affected, err := tx.QuotaNetContributionLedger.Update().
			Where(
				quotanetcontributionledger.IDEQ(ledger.ID),
				quotanetcontributionledger.StatusEQ(protocol.SettlementStatusPending),
				quotanetcontributionledger.PayoutBatchIDIsNil(),
			).
			SetStatus(protocol.SettlementStatusPending).
			SetPayoutBatchID(batchRow.ID).
			ClearSettledAt().
			Save(ctx)
		if err != nil {
			return nil, err
		}
		if affected != 1 {
			return nil, fmt.Errorf("quotanet settlement ledger changed before update: ledger_id=%d", ledger.ID)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	items := make([]*PayoutItem, 0, len(itemRows))
	for _, row := range itemRows {
		item := payoutItemFromEnt(row)
		if item != nil {
			item.Network = batchRow.Network
		}
		items = append(items, item)
	}
	return &CreateBatchResult{
		Batch:       payoutBatchFromEnt(batchRow),
		Items:       items,
		LedgerCount: len(ledgers),
	}, nil
}

func (s *Store) attachItemNetworks(ctx context.Context, items []*PayoutItem) error {
	if len(items) == 0 {
		return nil
	}
	batchIDs := make([]int64, 0, len(items))
	seen := make(map[int64]struct{})
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, ok := seen[item.BatchID]; ok {
			continue
		}
		seen[item.BatchID] = struct{}{}
		batchIDs = append(batchIDs, item.BatchID)
	}
	if len(batchIDs) == 0 {
		return nil
	}
	rows, err := s.client.QuotaNetPayoutBatch.Query().
		Where(quotanetpayoutbatch.IDIn(batchIDs...)).
		All(ctx)
	if err != nil {
		return err
	}
	networks := make(map[int64]string, len(rows))
	for _, row := range rows {
		networks[row.ID] = row.Network
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		item.Network = networks[item.BatchID]
	}
	return nil
}

func (s *Store) payoutNetwork(ctx context.Context, batchID int64) string {
	if s == nil || s.client == nil || batchID <= 0 {
		return ""
	}
	row, err := s.client.QuotaNetPayoutBatch.Get(ctx, batchID)
	if err != nil {
		return ""
	}
	return row.Network
}

func ExplorerTxURL(network, txHash string) string {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "solana-mainnet", "mainnet", "solana":
		return "https://explorer.solana.com/tx/" + txHash
	case "solana-testnet", "testnet":
		return "https://explorer.solana.com/tx/" + txHash + "?cluster=testnet"
	default:
		return "https://explorer.solana.com/tx/" + txHash + "?cluster=devnet"
	}
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

func normalizeBatchListParams(params BatchListParams) BatchListParams {
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
	return params
}

func normalizeItemListParams(params ItemListParams) ItemListParams {
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
	params.TxHash = strings.TrimSpace(params.TxHash)
	return params
}

func normalizeCreateBatchInput(input CreateBatchInput) CreateBatchInput {
	input.BatchKey = strings.TrimSpace(input.BatchKey)
	if input.BatchKey == "" {
		input.BatchKey = "qnp_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	input.Network = strings.TrimSpace(input.Network)
	if input.Network == "" {
		input.Network = defaultSettlementNetwork
	}
	input.WindowStart = input.WindowStart.UTC()
	input.WindowEnd = input.WindowEnd.UTC()
	return input
}

func validateCreateBatchInput(input CreateBatchInput) error {
	if input.BatchKey == "" {
		return fmt.Errorf("%w: batch_key is required", ErrInvalidBatchInput)
	}
	if input.WindowStart.IsZero() || input.WindowEnd.IsZero() || !input.WindowEnd.After(input.WindowStart) {
		return fmt.Errorf("%w: invalid settlement window", ErrInvalidBatchInput)
	}
	if input.Network == "" {
		return fmt.Errorf("%w: network is required", ErrInvalidBatchInput)
	}
	return nil
}

func normalizeUpdateItemStatusInput(input UpdateItemStatusInput) UpdateItemStatusInput {
	input.Status = strings.TrimSpace(input.Status)
	input.TxHash = strings.TrimSpace(input.TxHash)
	input.ErrorMessage = strings.TrimSpace(input.ErrorMessage)
	return input
}

func validateUpdateItemStatusInput(input UpdateItemStatusInput) error {
	switch input.Status {
	case protocol.SettlementStatusPending:
		return nil
	case protocol.SettlementStatusFinalized:
		return nil
	case protocol.SettlementStatusFailed:
		if input.ErrorMessage == "" {
			return fmt.Errorf("%w: error_message is required for failed item", ErrInvalidBatchInput)
		}
		return nil
	default:
		return fmt.Errorf("%w: invalid item status", ErrInvalidBatchInput)
	}
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

func ledgerSQLWhere(params ListParams) (string, []any) {
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if params.Status != "" {
		add("status = $%d", params.Status)
	}
	if params.WalletAddress != "" {
		add("wallet_address = $%d", params.WalletAddress)
	}
	if params.NodeID != nil {
		add("node_id = $%d", *params.NodeID)
	}
	if params.AccountID != nil {
		add("account_id = $%d", *params.AccountID)
	}
	if params.PayoutBatchID != nil {
		add("payout_batch_id = $%d", *params.PayoutBatchID)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func ledgerFromEnt(row *ent.QuotaNetContributionLedger) *Ledger {
	if row == nil {
		return nil
	}
	return &Ledger{
		ID:              row.ID,
		TaskID:          row.TaskID,
		UsageLogID:      row.UsageLogID,
		NodeID:          row.NodeID,
		WalletAddress:   row.WalletAddress,
		AccountID:       row.AccountID,
		Platform:        row.Platform,
		Model:           row.Model,
		TokenFlow:       row.TokenFlow,
		StandardCostUSD: row.StandardCostUsd,
		ActualCostUSD:   row.ActualCostUsd,
		ContributionUSD: row.ContributionUsd,
		AmountCxs:       row.AmountCxs,
		Rate:            row.Rate,
		Status:          row.Status,
		PayoutBatchID:   row.PayoutBatchID,
		SettledAt:       row.SettledAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

type walletPayout struct {
	WalletAddress   string
	NodeID          *int64
	TokenFlow       int64
	ContributionUSD float64
	AmountCxs       float64
}

func buildWalletPayouts(ledgers []*ent.QuotaNetContributionLedger) []*walletPayout {
	byWallet := make(map[string]*walletPayout)
	order := make([]string, 0)
	for _, ledger := range ledgers {
		if ledger == nil {
			continue
		}
		item, ok := byWallet[ledger.WalletAddress]
		if !ok {
			nodeID := ledger.NodeID
			item = &walletPayout{
				WalletAddress: ledger.WalletAddress,
				NodeID:        &nodeID,
			}
			byWallet[ledger.WalletAddress] = item
			order = append(order, ledger.WalletAddress)
		} else if item.NodeID != nil && *item.NodeID != ledger.NodeID {
			item.NodeID = nil
		}
		item.TokenFlow += ledger.TokenFlow
		item.ContributionUSD += ledger.ContributionUsd
		item.AmountCxs += ledger.AmountCxs
	}
	out := make([]*walletPayout, 0, len(order))
	for _, wallet := range order {
		out = append(out, byWallet[wallet])
	}
	return out
}

func defaultConfig() *Config {
	return &Config{
		Network: defaultSettlementNetwork,
	}
}

func updateBatchStatus(ctx context.Context, tx *ent.Tx, batchID int64) error {
	rows, err := tx.QuotaNetPayoutItem.Query().
		Where(quotanetpayoutitem.BatchIDEQ(batchID)).
		All(ctx)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	next := protocol.SettlementStatusFinalized
	for _, row := range rows {
		switch row.Status {
		case protocol.SettlementStatusFailed:
			next = protocol.SettlementStatusFailed
		case protocol.SettlementStatusPending:
			if next != protocol.SettlementStatusFailed {
				next = protocol.SettlementStatusPending
			}
		}
	}
	_, err = tx.QuotaNetPayoutBatch.Update().
		Where(quotanetpayoutbatch.IDEQ(batchID)).
		SetStatus(next).
		Save(ctx)
	return err
}

func payoutBatchFromEnt(row *ent.QuotaNetPayoutBatch) *PayoutBatch {
	if row == nil {
		return nil
	}
	return &PayoutBatch{
		ID:                   row.ID,
		BatchKey:             row.BatchKey,
		WindowStart:          row.WindowStart,
		WindowEnd:            row.WindowEnd,
		Status:               row.Status,
		Network:              row.Network,
		TotalTokenFlow:       row.TotalTokenFlow,
		TotalContributionUSD: row.TotalContributionUsd,
		TotalAmountCxs:       row.TotalAmountCxs,
		ItemCount:            row.ItemCount,
		CreatedBy:            row.CreatedBy,
		ApprovedBy:           row.ApprovedBy,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}
}

func payoutItemFromEnt(row *ent.QuotaNetPayoutItem) *PayoutItem {
	if row == nil {
		return nil
	}
	return &PayoutItem{
		ID:              row.ID,
		ItemKey:         row.ItemKey,
		BatchID:         row.BatchID,
		NodeID:          row.NodeID,
		WalletAddress:   row.WalletAddress,
		TokenFlow:       row.TokenFlow,
		ContributionUSD: row.ContributionUsd,
		AmountCxs:       row.AmountCxs,
		Status:          row.Status,
		TxHash:          row.TxHash,
		ErrorMessage:    row.ErrorMessage,
		FinalizedAt:     row.FinalizedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
