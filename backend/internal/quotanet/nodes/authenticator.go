// Package nodes contains QuotaNet node persistence and authentication helpers.
package nodes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/auth"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	qws "github.com/Wei-Shaw/sub2api/internal/quotanet/ws"
)

const (
	StatusPending  = "pending"
	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusBanned   = "banned"
)

var (
	ErrNodeNotFound     = errors.New("quotanet node not found")
	ErrNodeInactive     = errors.New("quotanet node is not active")
	ErrWalletMismatch   = errors.New("quotanet node wallet mismatch")
	ErrInvalidNodeToken = errors.New("quotanet node token is invalid")
)

type Node struct {
	ID            int64
	NodeKey       string
	WalletAddress string
	TokenHash     string
	Status        string
}

type NodeStore interface {
	GetByNodeKey(ctx context.Context, nodeKey string) (*Node, error)
}

type Authenticator struct {
	store NodeStore
}

func NewAuthenticator(store NodeStore) *Authenticator {
	return &Authenticator{store: store}
}

func (a *Authenticator) AuthenticateNode(ctx context.Context, token string, hello protocol.ClientHello) (qws.AuthenticatedNode, error) {
	if a == nil || a.store == nil {
		return qws.AuthenticatedNode{}, ErrNodeNotFound
	}
	nodeKey := strings.TrimSpace(hello.ClientID)
	if nodeKey == "" {
		return qws.AuthenticatedNode{}, fmt.Errorf("%w: client_id is required", ErrNodeNotFound)
	}

	node, err := a.store.GetByNodeKey(ctx, nodeKey)
	if err != nil {
		return qws.AuthenticatedNode{}, err
	}
	if node == nil {
		return qws.AuthenticatedNode{}, ErrNodeNotFound
	}
	if strings.TrimSpace(node.Status) != StatusActive {
		return qws.AuthenticatedNode{}, ErrNodeInactive
	}
	if strings.TrimSpace(node.WalletAddress) != strings.TrimSpace(hello.WalletAddress) {
		return qws.AuthenticatedNode{}, ErrWalletMismatch
	}
	if err := auth.VerifyNodeToken(token, node.TokenHash); err != nil {
		return qws.AuthenticatedNode{}, ErrInvalidNodeToken
	}

	return qws.AuthenticatedNode{
		NodeID:        node.ID,
		NodeKey:       strings.TrimSpace(node.NodeKey),
		WalletAddress: strings.TrimSpace(node.WalletAddress),
	}, nil
}
