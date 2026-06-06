package nodes

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/auth"
)

func TestManagerCreateGeneratesOneTimeToken(t *testing.T) {
	store := &stubManagementStore{}
	manager := NewManager(store)

	result, err := manager.Create(context.Background(), CreateInput{
		Name:          "node a",
		WalletAddress: "wallet-a",
		Status:        StatusActive,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Token == "" {
		t.Fatal("Create() token is empty")
	}
	if result.Node == nil || result.Node.NodeKey == "" {
		t.Fatalf("Create() node = %+v", result.Node)
	}
	if !isStoredHashForToken(t, result.Token, store.createdTokenHash) {
		t.Fatal("stored token hash does not verify returned token")
	}
	if store.createdInput.Status != StatusActive {
		t.Fatalf("created status = %q, want active", store.createdInput.Status)
	}
}

func TestManagerCreateDefaultsPendingStatus(t *testing.T) {
	store := &stubManagementStore{}
	manager := NewManager(store)

	_, err := manager.Create(context.Background(), CreateInput{
		Name:          "node a",
		WalletAddress: "wallet-a",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if store.createdInput.Status != StatusPending {
		t.Fatalf("created status = %q, want pending", store.createdInput.Status)
	}
}

func TestManagerCreateRejectsInvalidInput(t *testing.T) {
	manager := NewManager(&stubManagementStore{})
	if _, err := manager.Create(context.Background(), CreateInput{}); !errors.Is(err, ErrInvalidNodeInput) {
		t.Fatalf("Create(empty) error = %v, want ErrInvalidNodeInput", err)
	}
	if _, err := manager.Create(context.Background(), CreateInput{Name: "node", WalletAddress: "wallet", Status: "unknown"}); !errors.Is(err, ErrInvalidNodeStatus) {
		t.Fatalf("Create(invalid status) error = %v, want ErrInvalidNodeStatus", err)
	}
}

func TestManagerListNormalizesParams(t *testing.T) {
	store := &stubManagementStore{}
	manager := NewManager(store)

	_, _, err := manager.List(context.Background(), ListParams{Page: -1, PageSize: 999, Search: " test "})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if store.listParams.Page != 1 || store.listParams.PageSize != 100 || store.listParams.Search != "test" {
		t.Fatalf("list params = %+v", store.listParams)
	}
}

func TestManagerUpdateStatus(t *testing.T) {
	store := &stubManagementStore{}
	manager := NewManager(store)

	node, err := manager.UpdateStatus(context.Background(), 7, UpdateStatusInput{Status: StatusDisabled})
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if node.Status != StatusDisabled || store.updatedStatus != StatusDisabled {
		t.Fatalf("updated node=%+v status=%q", node, store.updatedStatus)
	}
	if _, err := manager.UpdateStatus(context.Background(), 7, UpdateStatusInput{Status: "bad"}); !errors.Is(err, ErrInvalidNodeStatus) {
		t.Fatalf("UpdateStatus(invalid) error = %v, want ErrInvalidNodeStatus", err)
	}
}

func TestManagerResetToken(t *testing.T) {
	store := &stubManagementStore{}
	manager := NewManager(store)

	result, err := manager.ResetToken(context.Background(), 7)
	if err != nil {
		t.Fatalf("ResetToken() error = %v", err)
	}
	if result.Token == "" || result.Node == nil {
		t.Fatalf("ResetToken() result = %+v", result)
	}
	if !isStoredHashForToken(t, result.Token, store.resetTokenHash) {
		t.Fatal("stored reset hash does not verify returned token")
	}
}

func TestManagerRegisterDevelopmentNodeCreatesActiveNode(t *testing.T) {
	store := &stubManagementStore{}
	manager := NewManager(store)

	result, err := manager.RegisterDevelopmentNode(context.Background(), RegisterDevelopmentNodeInput{
		Name:            "dev node",
		WalletAddress:   "wallet-dev",
		ClientVersion:   "v0.1.0",
		ProtocolVersion: "2026-06-qt1",
	})
	if err != nil {
		t.Fatalf("RegisterDevelopmentNode() error = %v", err)
	}
	if result.Token == "" {
		t.Fatal("RegisterDevelopmentNode() token is empty")
	}
	if result.Node == nil || result.Node.Status != StatusActive || result.Node.NodeKey == "" {
		t.Fatalf("RegisterDevelopmentNode() result = %+v", result)
	}
	if store.createdInput.Status != StatusActive || store.createdInput.ProtocolVersion != "2026-06-qt1" || store.createdInput.ClientVersion != "v0.1.0" {
		t.Fatalf("created input = %+v", store.createdInput)
	}
	if !isStoredHashForToken(t, result.Token, store.createdTokenHash) {
		t.Fatal("stored token hash does not verify returned token")
	}
}

func TestManagerRegisterDevelopmentNodeReusesWalletWithoutResettingToken(t *testing.T) {
	store := &stubManagementStore{
		walletNode: &Node{ID: 7, NodeKey: "qnn_existing", WalletAddress: "wallet-dev", Status: StatusActive, TokenHash: "old-hash"},
	}
	manager := NewManager(store)

	result, err := manager.RegisterDevelopmentNode(context.Background(), RegisterDevelopmentNodeInput{
		Name:            "updated",
		WalletAddress:   "wallet-dev",
		ProtocolVersion: "2026-06-qt1",
	})
	if err != nil {
		t.Fatalf("RegisterDevelopmentNode() error = %v", err)
	}
	if result.Token != "" {
		t.Fatalf("RegisterDevelopmentNode() token = %q, want empty", result.Token)
	}
	if store.updateRegistrationToken {
		t.Fatal("UpdateRegistration updated token, want false")
	}
	if result.Node.ID != 7 || result.Node.Status != StatusActive {
		t.Fatalf("RegisterDevelopmentNode() node = %+v", result.Node)
	}
}

func TestManagerRegisterDevelopmentNodeCanResetTokenWhenAllowed(t *testing.T) {
	store := &stubManagementStore{
		walletNode: &Node{ID: 7, NodeKey: "qnn_existing", WalletAddress: "wallet-dev", Status: StatusActive, TokenHash: "old-hash"},
	}
	manager := NewManager(store)

	result, err := manager.RegisterDevelopmentNode(context.Background(), RegisterDevelopmentNodeInput{
		WalletAddress:   "wallet-dev",
		ProtocolVersion: "2026-06-qt1",
		ResetToken:      true,
		AllowResetToken: true,
	})
	if err != nil {
		t.Fatalf("RegisterDevelopmentNode(reset) error = %v", err)
	}
	if result.Token == "" {
		t.Fatal("RegisterDevelopmentNode(reset) token is empty")
	}
	if !store.updateRegistrationToken {
		t.Fatal("UpdateRegistration did not update token")
	}
	if !isStoredHashForToken(t, result.Token, store.updateRegistrationTokenHash) {
		t.Fatal("stored reset hash does not verify returned token")
	}
}

func TestManagerRegisterDevelopmentNodeRejectsDisabledNode(t *testing.T) {
	store := &stubManagementStore{
		walletNode: &Node{ID: 7, NodeKey: "qnn_existing", WalletAddress: "wallet-dev", Status: StatusDisabled},
	}
	manager := NewManager(store)

	_, err := manager.RegisterDevelopmentNode(context.Background(), RegisterDevelopmentNodeInput{
		WalletAddress:   "wallet-dev",
		ProtocolVersion: "2026-06-qt1",
		ResetToken:      true,
		AllowResetToken: true,
	})
	if !errors.Is(err, ErrNodeInactive) {
		t.Fatalf("RegisterDevelopmentNode(disabled) error = %v, want ErrNodeInactive", err)
	}
	if store.updateRegistrationToken {
		t.Fatal("disabled node should not reset token")
	}
}

func TestManagerRegisterDevelopmentNodeRejectsUnsupportedProtocol(t *testing.T) {
	manager := NewManager(&stubManagementStore{})
	_, err := manager.RegisterDevelopmentNode(context.Background(), RegisterDevelopmentNodeInput{
		WalletAddress:   "wallet-dev",
		ProtocolVersion: "old",
	})
	if err == nil {
		t.Fatal("RegisterDevelopmentNode(old protocol) error is nil")
	}
}

func isStoredHashForToken(t *testing.T, token, hash string) bool {
	t.Helper()
	return auth.VerifyNodeToken(token, hash) == nil
}

type stubManagementStore struct {
	createdInput                CreateInput
	createdNodeKey              string
	createdTokenHash            string
	walletNode                  *Node
	updateRegistrationInput     CreateInput
	updateRegistrationToken     bool
	updateRegistrationTokenHash string
	listParams                  ListParams
	updatedStatus               string
	resetTokenHash              string
}

func (s *stubManagementStore) Create(_ context.Context, input CreateInput, nodeKey, tokenHash string) (*Node, error) {
	s.createdInput = input
	s.createdNodeKey = nodeKey
	s.createdTokenHash = tokenHash
	return &Node{
		ID:            1,
		NodeKey:       nodeKey,
		Name:          input.Name,
		OwnerUserID:   input.OwnerUserID,
		WalletAddress: input.WalletAddress,
		TokenHash:     tokenHash,
		Status:        input.Status,
	}, nil
}

func (s *stubManagementStore) GetByWalletAddress(_ context.Context, walletAddress string) (*Node, error) {
	if s.walletNode == nil || s.walletNode.WalletAddress != walletAddress {
		return nil, ErrNodeNotFound
	}
	return s.walletNode, nil
}

func (s *stubManagementStore) UpdateRegistration(_ context.Context, id int64, input CreateInput, tokenHash string, updateToken bool) (*Node, error) {
	s.updateRegistrationInput = input
	s.updateRegistrationToken = updateToken
	s.updateRegistrationTokenHash = tokenHash
	token := ""
	if s.walletNode != nil {
		token = s.walletNode.TokenHash
	}
	if updateToken {
		token = tokenHash
	}
	return &Node{
		ID:            id,
		NodeKey:       "qnn_existing",
		Name:          input.Name,
		WalletAddress: input.WalletAddress,
		TokenHash:     token,
		Status:        input.Status,
	}, nil
}

func (s *stubManagementStore) List(_ context.Context, params ListParams) ([]*Node, int64, error) {
	s.listParams = params
	return []*Node{{ID: 1, NodeKey: "node-1"}}, 1, nil
}

func (s *stubManagementStore) GetByID(_ context.Context, id int64) (*Node, error) {
	return &Node{ID: id, NodeKey: "node-1"}, nil
}

func (s *stubManagementStore) UpdateStatus(_ context.Context, id int64, status string) (*Node, error) {
	s.updatedStatus = status
	return &Node{ID: id, NodeKey: "node-1", Status: status}, nil
}

func (s *stubManagementStore) ResetToken(_ context.Context, id int64, tokenHash string) (*Node, error) {
	s.resetTokenHash = tokenHash
	return &Node{ID: id, NodeKey: "node-1", TokenHash: tokenHash}, nil
}
