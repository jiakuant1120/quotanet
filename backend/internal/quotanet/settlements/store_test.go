package settlements

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/ent"
)

func TestBuildWalletPayoutsAggregatesByWallet(t *testing.T) {
	ledgers := []*ent.QuotaNetContributionLedger{
		{WalletAddress: "wallet-1", NodeID: 11, TokenFlow: 100},
		{WalletAddress: "wallet-2", NodeID: 22, TokenFlow: 300},
		{WalletAddress: "wallet-1", NodeID: 11, TokenFlow: 200},
	}

	items := buildWalletPayouts(ledgers, 0.01)
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	if items[0].WalletAddress != "wallet-1" || items[0].TokenFlow != 300 || items[0].AmountCxs != 3 {
		t.Fatalf("wallet-1 item = %+v", items[0])
	}
	if items[0].NodeID == nil || *items[0].NodeID != 11 {
		t.Fatalf("wallet-1 node_id = %v, want 11", items[0].NodeID)
	}
	if items[1].WalletAddress != "wallet-2" || items[1].TokenFlow != 300 || items[1].AmountCxs != 3 {
		t.Fatalf("wallet-2 item = %+v", items[1])
	}
}

func TestBuildWalletPayoutsClearsNodeIDForMultiNodeWallet(t *testing.T) {
	ledgers := []*ent.QuotaNetContributionLedger{
		{WalletAddress: "wallet-1", NodeID: 11, TokenFlow: 100},
		{WalletAddress: "wallet-1", NodeID: 12, TokenFlow: 200},
	}

	items := buildWalletPayouts(ledgers, 0)
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if items[0].NodeID != nil {
		t.Fatalf("node_id = %v, want nil for multi-node wallet", items[0].NodeID)
	}
	if items[0].TokenFlow != 300 {
		t.Fatalf("token_flow = %d, want 300", items[0].TokenFlow)
	}
}
