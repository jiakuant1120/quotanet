package nodes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
)

func TestSessionCapabilitiesPayloadIncludesAccounts(t *testing.T) {
	payload := sessionCapabilitiesPayload(
		[]protocol.Capability{{Provider: "openai", Models: []string{"gpt-4.1"}}},
		[]protocol.AccountHeartbeat{{Provider: "openai", Status: protocol.NodeStatusReady}},
	)

	if _, ok := payload["items"]; !ok {
		t.Fatalf("payload = %+v, want items", payload)
	}
	if _, ok := payload["accounts"]; !ok {
		t.Fatalf("payload = %+v, want accounts", payload)
	}
}

func TestMergeSessionAccountsKeepsCapabilities(t *testing.T) {
	existing := map[string]any{
		"items": []protocol.Capability{{Provider: "openai", Models: []string{"gpt-4.1"}}},
	}
	merged := mergeSessionAccounts(existing, []protocol.AccountHeartbeat{{Provider: "openai", Status: protocol.NodeStatusReady}})

	if _, ok := merged["items"]; !ok {
		t.Fatalf("merged = %+v, want existing items", merged)
	}
	accounts, ok := merged["accounts"].([]protocol.AccountHeartbeat)
	if !ok || len(accounts) != 1 || accounts[0].Provider != "openai" {
		t.Fatalf("accounts = %+v", merged["accounts"])
	}
	if _, ok := existing["accounts"]; ok {
		t.Fatalf("existing payload was mutated: %+v", existing)
	}
}
