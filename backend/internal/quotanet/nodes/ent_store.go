package nodes

import (
	"context"
	"strings"

	"entgo.io/ent/dialect/sql"
	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/quotanetnode"
)

type EntStore struct {
	client *ent.Client
}

func NewEntStore(client *ent.Client) *EntStore {
	return &EntStore{client: client}
}

func (s *EntStore) GetByNodeKey(ctx context.Context, nodeKey string) (*Node, error) {
	if s == nil || s.client == nil {
		return nil, ErrNodeNotFound
	}
	row, err := s.client.QuotaNetNode.Query().
		Where(quotanetnode.NodeKeyEQ(nodeKey)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}
	return nodeFromEnt(row), nil
}

func (s *EntStore) Create(ctx context.Context, input CreateInput, nodeKey, tokenHash string) (*Node, error) {
	create := s.client.QuotaNetNode.Create().
		SetNodeKey(nodeKey).
		SetName(input.Name).
		SetWalletAddress(input.WalletAddress).
		SetTokenHash(tokenHash).
		SetStatus(input.Status).
		SetNillableOwnerUserID(input.OwnerUserID)
	if strings.TrimSpace(input.ProtocolVersion) != "" {
		create.SetProtocolVersion(strings.TrimSpace(input.ProtocolVersion))
	}
	if strings.TrimSpace(input.ClientVersion) != "" {
		create.SetClientVersion(strings.TrimSpace(input.ClientVersion))
	}
	if input.LastSeenAt != nil {
		create.SetLastSeenAt(*input.LastSeenAt)
	}
	row, err := create.Save(ctx)
	if err != nil {
		return nil, err
	}
	return nodeFromEnt(row), nil
}

func (s *EntStore) GetByWalletAddress(ctx context.Context, walletAddress string) (*Node, error) {
	if s == nil || s.client == nil {
		return nil, ErrNodeNotFound
	}
	walletAddress = strings.TrimSpace(walletAddress)
	row, err := s.client.QuotaNetNode.Query().
		Where(quotanetnode.WalletAddressEQ(walletAddress)).
		Order(quotanetnode.ByCreatedAt(sql.OrderAsc())).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}
	return nodeFromEnt(row), nil
}

func (s *EntStore) UpdateRegistration(ctx context.Context, id int64, input CreateInput, tokenHash string, updateToken bool) (*Node, error) {
	update := s.client.QuotaNetNode.UpdateOneID(id).
		SetName(input.Name).
		SetWalletAddress(input.WalletAddress).
		SetStatus(input.Status)
	if strings.TrimSpace(input.ProtocolVersion) != "" {
		update.SetProtocolVersion(strings.TrimSpace(input.ProtocolVersion))
	}
	if strings.TrimSpace(input.ClientVersion) != "" {
		update.SetClientVersion(strings.TrimSpace(input.ClientVersion))
	}
	if input.LastSeenAt != nil {
		update.SetLastSeenAt(*input.LastSeenAt)
	}
	if updateToken {
		update.SetTokenHash(tokenHash)
	}
	row, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}
	return nodeFromEnt(row), nil
}

func (s *EntStore) List(ctx context.Context, params ListParams) ([]*Node, int64, error) {
	query := s.client.QuotaNetNode.Query()
	if params.Status != "" {
		query = query.Where(quotanetnode.StatusEQ(params.Status))
	}
	if params.Search != "" {
		search := strings.TrimSpace(params.Search)
		query = query.Where(quotanetnode.Or(
			quotanetnode.NameContainsFold(search),
			quotanetnode.NodeKeyContainsFold(search),
			quotanetnode.WalletAddressContainsFold(search),
		))
	}
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := query.
		Order(quotanetnode.ByCreatedAt(sql.OrderDesc())).
		Limit(params.PageSize).
		Offset((params.Page - 1) * params.PageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*Node, 0, len(rows))
	for _, row := range rows {
		out = append(out, nodeFromEnt(row))
	}
	return out, int64(total), nil
}

func (s *EntStore) GetByID(ctx context.Context, id int64) (*Node, error) {
	row, err := s.client.QuotaNetNode.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}
	return nodeFromEnt(row), nil
}

func (s *EntStore) UpdateStatus(ctx context.Context, id int64, status string) (*Node, error) {
	row, err := s.client.QuotaNetNode.UpdateOneID(id).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}
	return nodeFromEnt(row), nil
}

func (s *EntStore) ResetToken(ctx context.Context, id int64, tokenHash string) (*Node, error) {
	row, err := s.client.QuotaNetNode.UpdateOneID(id).
		SetTokenHash(tokenHash).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}
	return nodeFromEnt(row), nil
}

func nodeFromEnt(row *ent.QuotaNetNode) *Node {
	if row == nil {
		return nil
	}
	return &Node{
		ID:            row.ID,
		NodeKey:       row.NodeKey,
		Name:          row.Name,
		OwnerUserID:   row.OwnerUserID,
		WalletAddress: row.WalletAddress,
		TokenHash:     row.TokenHash,
		Status:        row.Status,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
		LastSeenAt:    row.LastSeenAt,
	}
}
