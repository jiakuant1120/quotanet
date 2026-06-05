package settlements

import (
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
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

func TestValidateUpdateItemStatusInput(t *testing.T) {
	tests := []struct {
		name    string
		input   UpdateItemStatusInput
		wantErr bool
	}{
		{
			name:  "pending allows empty details",
			input: UpdateItemStatusInput{Status: protocol.SettlementStatusPending},
		},
		{
			name:    "finalized requires tx hash",
			input:   UpdateItemStatusInput{Status: protocol.SettlementStatusFinalized},
			wantErr: true,
		},
		{
			name:  "finalized accepts tx hash",
			input: UpdateItemStatusInput{Status: protocol.SettlementStatusFinalized, TxHash: "tx-1"},
		},
		{
			name:    "failed requires error message",
			input:   UpdateItemStatusInput{Status: protocol.SettlementStatusFailed},
			wantErr: true,
		},
		{
			name:  "failed accepts error message",
			input: UpdateItemStatusInput{Status: protocol.SettlementStatusFailed, ErrorMessage: "rejected"},
		},
		{
			name:    "unknown status rejected",
			input:   UpdateItemStatusInput{Status: "done"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUpdateItemStatusInput(tt.input)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidBatchInput) {
					t.Fatalf("error = %v, want ErrInvalidBatchInput", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
		})
	}
}

func TestExplorerTxURL(t *testing.T) {
	tests := []struct {
		name    string
		network string
		txHash  string
		want    string
	}{
		{
			name:    "devnet default",
			network: "solana-devnet",
			txHash:  "tx-1",
			want:    "https://explorer.solana.com/tx/tx-1?cluster=devnet",
		},
		{
			name:    "testnet",
			network: "solana-testnet",
			txHash:  "tx-1",
			want:    "https://explorer.solana.com/tx/tx-1?cluster=testnet",
		},
		{
			name:    "mainnet",
			network: "solana-mainnet",
			txHash:  "tx-1",
			want:    "https://explorer.solana.com/tx/tx-1",
		},
		{
			name:    "empty tx",
			network: "solana-devnet",
			txHash:  " ",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExplorerTxURL(tt.network, tt.txHash)
			if got != tt.want {
				t.Fatalf("ExplorerTxURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
