package nodes

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/auth"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
)

func TestAuthenticatorAuthenticateNode(t *testing.T) {
	token, hash := nodeTokenAndHash(t)
	store := &stubNodeStore{
		node: &Node{
			ID:            10,
			NodeKey:       "node-10",
			WalletAddress: "wallet-10",
			TokenHash:     hash,
			Status:        StatusActive,
		},
	}
	authenticator := NewAuthenticator(store)

	got, err := authenticator.AuthenticateNode(context.Background(), token, protocol.ClientHello{
		ClientID:      "node-10",
		WalletAddress: "wallet-10",
	})
	if err != nil {
		t.Fatalf("AuthenticateNode() error = %v", err)
	}
	if got.NodeID != 10 || got.NodeKey != "node-10" || got.WalletAddress != "wallet-10" {
		t.Fatalf("authenticated node = %+v", got)
	}
	if store.lastNodeKey != "node-10" {
		t.Fatalf("store lookup key = %q, want node-10", store.lastNodeKey)
	}
}

func TestAuthenticatorRejectsInactiveNode(t *testing.T) {
	token, hash := nodeTokenAndHash(t)
	authenticator := NewAuthenticator(&stubNodeStore{node: &Node{
		ID:            1,
		NodeKey:       "node-1",
		WalletAddress: "wallet-1",
		TokenHash:     hash,
		Status:        StatusPending,
	}})

	_, err := authenticator.AuthenticateNode(context.Background(), token, protocol.ClientHello{
		ClientID:      "node-1",
		WalletAddress: "wallet-1",
	})
	if !errors.Is(err, ErrNodeInactive) {
		t.Fatalf("AuthenticateNode() error = %v, want ErrNodeInactive", err)
	}
}

func TestAuthenticatorRejectsWalletMismatch(t *testing.T) {
	token, hash := nodeTokenAndHash(t)
	authenticator := NewAuthenticator(&stubNodeStore{node: &Node{
		ID:            1,
		NodeKey:       "node-1",
		WalletAddress: "registered-wallet",
		TokenHash:     hash,
		Status:        StatusActive,
	}})

	_, err := authenticator.AuthenticateNode(context.Background(), token, protocol.ClientHello{
		ClientID:      "node-1",
		WalletAddress: "hello-wallet",
	})
	if !errors.Is(err, ErrWalletMismatch) {
		t.Fatalf("AuthenticateNode() error = %v, want ErrWalletMismatch", err)
	}
}

func TestAuthenticatorRejectsInvalidToken(t *testing.T) {
	_, hash := nodeTokenAndHash(t)
	authenticator := NewAuthenticator(&stubNodeStore{node: &Node{
		ID:            1,
		NodeKey:       "node-1",
		WalletAddress: "wallet-1",
		TokenHash:     hash,
		Status:        StatusActive,
	}})

	_, err := authenticator.AuthenticateNode(context.Background(), "wrong-token", protocol.ClientHello{
		ClientID:      "node-1",
		WalletAddress: "wallet-1",
	})
	if !errors.Is(err, ErrInvalidNodeToken) {
		t.Fatalf("AuthenticateNode() error = %v, want ErrInvalidNodeToken", err)
	}
}

func TestAuthenticatorReturnsStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	authenticator := NewAuthenticator(&stubNodeStore{err: wantErr})

	_, err := authenticator.AuthenticateNode(context.Background(), "token", protocol.ClientHello{
		ClientID:      "node-1",
		WalletAddress: "wallet-1",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("AuthenticateNode() error = %v, want store error", err)
	}
}

func nodeTokenAndHash(t *testing.T) (string, string) {
	t.Helper()
	token, err := auth.GenerateNodeToken()
	if err != nil {
		t.Fatalf("GenerateNodeToken() error = %v", err)
	}
	hash, err := auth.HashNodeToken(token)
	if err != nil {
		t.Fatalf("HashNodeToken() error = %v", err)
	}
	return token, hash
}

type stubNodeStore struct {
	node        *Node
	err         error
	lastNodeKey string
}

func (s *stubNodeStore) GetByNodeKey(_ context.Context, nodeKey string) (*Node, error) {
	s.lastNodeKey = nodeKey
	if s.err != nil {
		return nil, s.err
	}
	return s.node, nil
}
