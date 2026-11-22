package collector

import (
	"strings"
	"testing"
)

func TestParseWireGuardDump(t *testing.T) {
	dump := strings.Join([]string{
		"wg0 private public 51820 off",
		"wg0 peerkey presharedkey 192.0.2.1:51820 10.0.0.2/32 1700000000 1024 2048 25",
		"wg1 private public 51821 off",
		"wg1 neverconnected presharedkey (none) (none) 0 0 0 off",
	}, "\n")

	peers, err := parseWireGuardDump([]byte(dump))
	if err != nil {
		t.Fatalf("parseWireGuardDump returned error: %v", err)
	}
	if len(peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(peers))
	}

	if peers[0].interfaceName != "wg0" || peers[0].publicKey != "peerkey" {
		t.Errorf("unexpected peer identity: %+v", peers[0])
	}
	if peers[0].latestHandshake != 1700000000 || peers[0].receiveBytes != 1024 || peers[0].transmitBytes != 2048 {
		t.Errorf("unexpected peer metrics: %+v", peers[0])
	}
	if peers[1].latestHandshake != 0 || peers[1].receiveBytes != 0 || peers[1].transmitBytes != 0 {
		t.Errorf("unexpected never-connected peer metrics: %+v", peers[1])
	}
}

func TestParseWireGuardDumpRejectsMalformedPeer(t *testing.T) {
	dump := "wg0 peerkey presharedkey endpoint allowedips 1700000000 invalid 2048 25"
	if _, err := parseWireGuardDump([]byte(dump)); err == nil {
		t.Fatal("expected malformed byte count to return an error")
	}
}
