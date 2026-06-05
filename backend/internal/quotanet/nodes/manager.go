package nodes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/auth"
)

var (
	ErrInvalidNodeInput  = errors.New("invalid quotanet node input")
	ErrInvalidNodeStatus = errors.New("invalid quotanet node status")
)

type ListParams struct {
	Page     int
	PageSize int
	Status   string
	Search   string
}

type CreateInput struct {
	Name          string
	WalletAddress string
	OwnerUserID   *int64
	Status        string
}

type UpdateStatusInput struct {
	Status string
}

type CreateResult struct {
	Node  *Node
	Token string
}

type ResetTokenResult struct {
	Node  *Node
	Token string
}

type ManagementStore interface {
	Create(ctx context.Context, input CreateInput, nodeKey, tokenHash string) (*Node, error)
	List(ctx context.Context, params ListParams) ([]*Node, int64, error)
	GetByID(ctx context.Context, id int64) (*Node, error)
	UpdateStatus(ctx context.Context, id int64, status string) (*Node, error)
	ResetToken(ctx context.Context, id int64, tokenHash string) (*Node, error)
}

type Manager struct {
	store ManagementStore
}

func NewManager(store ManagementStore) *Manager {
	return &Manager{store: store}
}

func (m *Manager) Create(ctx context.Context, input CreateInput) (*CreateResult, error) {
	if m == nil || m.store == nil {
		return nil, ErrNodeNotFound
	}
	input.Name = strings.TrimSpace(input.Name)
	input.WalletAddress = strings.TrimSpace(input.WalletAddress)
	input.Status = normalizeStatus(input.Status, StatusPending)
	if input.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidNodeInput)
	}
	if input.WalletAddress == "" {
		return nil, fmt.Errorf("%w: wallet_address is required", ErrInvalidNodeInput)
	}
	if !isValidStatus(input.Status) {
		return nil, ErrInvalidNodeStatus
	}

	token, err := auth.GenerateNodeToken()
	if err != nil {
		return nil, err
	}
	tokenHash, err := auth.HashNodeToken(token)
	if err != nil {
		return nil, err
	}
	nodeKey, err := auth.FingerprintNodeToken(token)
	if err != nil {
		return nil, err
	}
	nodeKey = "qnn_" + nodeKey

	node, err := m.store.Create(ctx, input, nodeKey, tokenHash)
	if err != nil {
		return nil, err
	}
	return &CreateResult{Node: node, Token: token}, nil
}

func (m *Manager) List(ctx context.Context, params ListParams) ([]*Node, int64, error) {
	if m == nil || m.store == nil {
		return nil, 0, ErrNodeNotFound
	}
	params.Page = normalizePositive(params.Page, 1)
	params.PageSize = normalizePageSize(params.PageSize)
	params.Status = strings.TrimSpace(params.Status)
	params.Search = strings.TrimSpace(params.Search)
	if params.Status != "" && !isValidStatus(params.Status) {
		return nil, 0, ErrInvalidNodeStatus
	}
	return m.store.List(ctx, params)
}

func (m *Manager) GetByID(ctx context.Context, id int64) (*Node, error) {
	if m == nil || m.store == nil {
		return nil, ErrNodeNotFound
	}
	if id <= 0 {
		return nil, ErrInvalidNodeInput
	}
	return m.store.GetByID(ctx, id)
}

func (m *Manager) UpdateStatus(ctx context.Context, id int64, input UpdateStatusInput) (*Node, error) {
	if m == nil || m.store == nil {
		return nil, ErrNodeNotFound
	}
	status := normalizeStatus(input.Status, "")
	if id <= 0 || status == "" {
		return nil, ErrInvalidNodeInput
	}
	if !isValidStatus(status) {
		return nil, ErrInvalidNodeStatus
	}
	return m.store.UpdateStatus(ctx, id, status)
}

func (m *Manager) ResetToken(ctx context.Context, id int64) (*ResetTokenResult, error) {
	if m == nil || m.store == nil {
		return nil, ErrNodeNotFound
	}
	if id <= 0 {
		return nil, ErrInvalidNodeInput
	}
	token, err := auth.GenerateNodeToken()
	if err != nil {
		return nil, err
	}
	tokenHash, err := auth.HashNodeToken(token)
	if err != nil {
		return nil, err
	}
	node, err := m.store.ResetToken(ctx, id, tokenHash)
	if err != nil {
		return nil, err
	}
	return &ResetTokenResult{Node: node, Token: token}, nil
}

func normalizeStatus(status, fallback string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return fallback
	}
	return status
}

func isValidStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case StatusPending, StatusActive, StatusDisabled, StatusBanned:
		return true
	default:
		return false
	}
}

func normalizePositive(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func normalizePageSize(value int) int {
	if value <= 0 {
		return 20
	}
	if value > 100 {
		return 100
	}
	return value
}
