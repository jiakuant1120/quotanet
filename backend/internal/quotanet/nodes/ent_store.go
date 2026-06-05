package nodes

import (
	"context"

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
	return &Node{
		ID:            row.ID,
		NodeKey:       row.NodeKey,
		WalletAddress: row.WalletAddress,
		TokenHash:     row.TokenHash,
		Status:        row.Status,
	}, nil
}
