package collector

import (
	"testing"
)

func TestParseInterfaceLinkState(t *testing.T) {
	output := `igc0: flags=8863<UP,BROADCAST,RUNNING,SIMPLEX,MULTICAST> metric 0 mtu 1500
	media: Ethernet autoselect (1000baseT <full-duplex>)
	status: active
igc1: flags=8822<BROADCAST,SIMPLEX,MULTICAST> metric 0 mtu 1500
	media: Ethernet autoselect
	status: no carrier
wg0: flags=1008043<UP,RUNNING,LOWER_UP>
	media: Ethernet autoselect (10Gbase-T <full-duplex>)
	status: active
`

	states := parseInterfaceLinkState([]byte(output))
	if len(states) != 3 {
		t.Fatalf("expected 3 interfaces, got %d", len(states))
	}
	if !states["igc0"].active || states["igc0"].speedMbps != 1000 {
		t.Errorf("unexpected igc0 state: %+v", states["igc0"])
	}
	if states["igc1"].active || states["igc1"].speedMbps != 0 {
		t.Errorf("unexpected igc1 state: %+v", states["igc1"])
	}
	if !states["wg0"].active || states["wg0"].speedMbps != 10000 {
		t.Errorf("unexpected wg0 state: %+v", states["wg0"])
	}
}

func TestParseInterfaceSpeed(t *testing.T) {
	for _, test := range []struct {
		media string
		want  float64
	}{
		{"Ethernet autoselect (100baseTX <full-duplex>)", 100},
		{"Ethernet autoselect (2.5Gbase-T <full-duplex>)", 2500},
		{"Ethernet autoselect (1000baseT <full-duplex>)", 1000},
		{"Ethernet autoselect", 0},
	} {
		if got := parseInterfaceSpeed(test.media); got != test.want {
			t.Errorf("parseInterfaceSpeed(%q) = %v; want %v", test.media, got, test.want)
		}
	}
}
