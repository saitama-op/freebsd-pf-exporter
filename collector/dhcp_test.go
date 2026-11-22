package collector

import "testing"

func TestParseDHCPPools(t *testing.T) {
	config := `<opnsense>
  <dhcpd>
    <lan><range><from>192.168.1.100</from><to>192.168.1.110</to></range></lan>
  </dhcpd>
  <Kea>
    <dhcp4>
      <subnet4>
        <item>
          <subnet>10.0.0.0/24</subnet>
          <pools><item><pool>10.0.0.20-10.0.0.30</pool></item></pools>
        </item>
      </subnet4>
    </dhcp4>
  </Kea>
</opnsense>`

	pools := parseDHCPPools([]byte(config))
	if len(pools) != 2 {
		t.Fatalf("expected 2 pools, got %d: %+v", len(pools), pools)
	}
	if pools[0].iface != "lan" || pools[0].name != "192.168.1.100-192.168.1.110" {
		t.Errorf("unexpected legacy pool: %+v", pools[0])
	}
	if capacity := pools[0].last - pools[0].first + 1; capacity != 11 {
		t.Errorf("legacy pool capacity = %d; want 11", capacity)
	}
	if pools[1].iface != "kea" || pools[1].name != "10.0.0.0/24 10.0.0.20-10.0.0.30" {
		t.Errorf("unexpected Kea pool: %+v", pools[1])
	}
	if capacity := pools[1].last - pools[1].first + 1; capacity != 11 {
		t.Errorf("Kea pool capacity = %d; want 11", capacity)
	}
	leases := map[string]struct{}{
		"192.168.1.103": {},
		"192.168.1.109": {},
		"192.168.1.200": {},
		"10.0.0.25":     {},
	}
	if used := countDHCPPoolLeases(pools[0], leases); used != 2 {
		t.Errorf("legacy pool used = %v; want 2", used)
	}
	if used := countDHCPPoolLeases(pools[1], leases); used != 1 {
		t.Errorf("Kea pool used = %v; want 1", used)
	}
}

func TestNewDHCPPoolCIDR(t *testing.T) {
	pool, ok := newDHCPPool("kea", "192.168.2.0/29")
	if !ok {
		t.Fatal("expected valid CIDR pool")
	}
	if capacity := pool.last - pool.first + 1; capacity != 8 {
		t.Errorf("CIDR pool capacity = %d; want 8", capacity)
	}
}

func TestParseDHCPLeases(t *testing.T) {
	data := []byte(`[{"state":"active","address":"192.168.1.103"},{"state":"active","ip":"10.0.0.25"},{"state":"expired","address":"192.168.1.104"}]`)
	active, expired, activeIPs, err := parseDHCPLeases(data)
	if err != nil {
		t.Fatalf("parseDHCPLeases returned error: %v", err)
	}
	if active != 2 || expired != 1 {
		t.Errorf("lease counts = active %v, expired %v; want 2 and 1", active, expired)
	}
	if len(activeIPs) != 2 {
		t.Errorf("active IPs = %v; want two addresses", activeIPs)
	}
}

func TestParseDHCPLeasesRejectsInvalidJSON(t *testing.T) {
	_, _, _, err := parseDHCPLeases([]byte(`{"not":"an array"}`))
	if err == nil {
		t.Fatal("expected invalid lease payload to return an error")
	}
}
