package analysis

import (
	"doc_generator/pkg/store"
	"testing"
)

func TestGameNetcodeScanner(t *testing.T) {
	scanner := &GameNetcodeScanner{}

	// 1. Test Reconciliation Match
	s := store.Symbol{Name: "PlayerReconLoop"}
	methods := []store.Symbol{
		{Name: "ReconcileState"},
	}
	fields := []store.Symbol{
		{Name: "TickBuffer"},
	}

	comp := scanner.Scan(s, methods, fields)
	if comp == nil {
		t.Fatal("Expected GameNetcodeScanner to match PlayerReconLoop, got nil")
	}
	if comp.Type != "Latency Mitigation" {
		t.Errorf("Expected type Latency Mitigation, got %s", comp.Type)
	}

	// 2. Test Lag Compensation
	s2 := store.Symbol{Name: "ServerHitboxRewinder"}
	comp2 := scanner.Scan(s2, nil, nil)
	if comp2 == nil {
		t.Fatal("Expected match for Rewinder, got nil")
	}
	if comp2.Type != "Game Netcode" {
		t.Errorf("Expected type Game Netcode, got %s", comp2.Type)
	}
}

func TestDistributedScanner(t *testing.T) {
	scanner := &DistributedScanner{}

	// 1. Test PubSub Engine
	s := store.Symbol{Name: "DynamicBroadcaster"}
	comp := scanner.Scan(s, nil, nil)
	if comp == nil {
		t.Fatal("Expected broadcaster detection")
	}
	if comp.Type != "Realtime Engine" {
		t.Errorf("Expected type Realtime Engine, got %s", comp.Type)
	}

	// 2. Test Cache Hub
	s2 := store.Symbol{Name: "RedisSessionStore"}
	methods2 := []store.Symbol{
		{Name: "GetItem"},
	}
	comp2 := scanner.Scan(s2, methods2, nil)
	if comp2 == nil {
		t.Fatal("Expected cache match")
	}
}

func TestTransportScanner(t *testing.T) {
	scanner := &TransportScanner{}

	// 1. Test Socket Layer
	s := store.Symbol{Name: "MultiplayerSocket"}
	fields := []store.Symbol{
		{Name: "remoteAddress", Type: "*net.UDPConn"},
	}
	comp := scanner.Scan(s, nil, fields)
	if comp == nil {
		t.Fatal("Expected UDP Socket match")
	}
	if comp.Details["Protocol"] != "UDP / Unreliable" {
		t.Errorf("Expected Protocol Details UDP, got %s", comp.Details["Protocol"])
	}

	// 2. Test API Gateway Match
	s2 := store.Symbol{Name: "GatewayRouter"}
	methods2 := []store.Symbol{
		{Name: "RegisterGetRoute"},
	}
	comp2 := scanner.Scan(s2, methods2, nil)
	if comp2 == nil {
		t.Fatal("Expected API Gateway match")
	}
	if comp2.Type != "Edge Service" {
		t.Errorf("Expected type Edge Service, got %s", comp2.Type)
	}
}

func TestRunNetworkAnalysisFull(t *testing.T) {
	source := &store.Source{
		Symbols: []store.Symbol{
			{Name: "AuthTokenVerifier", Kind: store.SymStruct},
			{Name: "VerifyToken", Kind: store.SymMethod, Parent: "AuthTokenVerifier"},
		},
	}

	RunNetworkAnalysis(source)

	if len(source.NetworkAnalysis) == 0 {
		t.Fatal("Expected network scanning to detect components")
	}

	comp := source.NetworkAnalysis[0]
	if comp.Type != "Security Enforcer" {
		t.Errorf("Expected detected component to be Security Enforcer, got %s", comp.Type)
	}
}
