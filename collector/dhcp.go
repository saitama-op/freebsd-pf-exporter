// collector/dhcp.go
package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/saitama-op/freebsd-pf-exporter/internal"
)

type DHCPCollector struct {
	timeout time.Duration

	activeLeases    *prometheus.Desc
	staticLeases    *prometheus.Desc
	expiredLeases   *prometheus.Desc
	poolCapacity    *prometheus.Desc
	poolUsed        *prometheus.Desc
	poolUtilization *prometheus.Desc
}

type dhcpPool struct {
	iface string
	name  string
	first uint32
	last  uint32
}

func NewDHCPCollector(timeout time.Duration) *DHCPCollector {
	return &DHCPCollector{
		timeout: timeout,
		activeLeases: prometheus.NewDesc(
			"opnsense_dhcp_active_leases",
			"Number of active dynamic DHCP leases",
			nil, nil,
		),
		staticLeases: prometheus.NewDesc(
			"opnsense_dhcp_static_leases",
			"Number of configured static DHCP reservations",
			nil, nil,
		),
		expiredLeases: prometheus.NewDesc(
			"opnsense_dhcp_expired_leases",
			"Number of expired dynamic DHCP leases",
			nil, nil,
		),
		poolCapacity: prometheus.NewDesc(
			"opnsense_dhcp_pool_capacity_addresses",
			"Number of IPv4 addresses configured in the DHCP pool",
			[]string{"interface", "pool"}, nil,
		),
		poolUsed: prometheus.NewDesc(
			"opnsense_dhcp_pool_used_addresses",
			"Number of active dynamic leases in the DHCP pool",
			[]string{"interface", "pool"}, nil,
		),
		poolUtilization: prometheus.NewDesc(
			"opnsense_dhcp_pool_utilization_ratio",
			"Fraction of DHCP pool addresses currently used by active dynamic leases",
			[]string{"interface", "pool"}, nil,
		),
	}
}

func (c *DHCPCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.activeLeases
	ch <- c.staticLeases
	ch <- c.expiredLeases
	ch <- c.poolCapacity
	ch <- c.poolUsed
	ch <- c.poolUtilization
}

func (c *DHCPCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	var active, expired float64
	activeIPs := make(map[string]struct{})
	leasesAvailable := false

	// 1. Check for any active dynamic leases
	out, err := internal.RunCommand(ctx, "configctl", "dhcpd", "list", "leases")
	if err == nil {
		active, expired, activeIPs, err = parseDHCPLeases(out)
		if err != nil {
			log.Printf("Error parsing configctl dhcpd lease output: %v", err)
		} else {
			leasesAvailable = true
		}
	} else {
		log.Printf("Error running configctl dhcpd: %v", err)
	}

	// 2. Count Static Reservations directly from OPNsense config
	var static float64
	configData, err := os.ReadFile("/conf/config.xml")
	if err == nil {
		// OPNsense uses <staticmap> for legacy ISC DHCP and <reservation> for the newer Kea DHCP
		iscCount := bytes.Count(configData, []byte("<staticmap>"))
		keaCount := bytes.Count(configData, []byte("<reservation>"))
		static = float64(iscCount + keaCount)
		for _, pool := range parseDHCPPools(configData) {
			capacity := float64(pool.last-pool.first) + 1
			ch <- prometheus.MustNewConstMetric(c.poolCapacity, prometheus.GaugeValue, capacity, pool.iface, pool.name)
			if leasesAvailable {
				used := countDHCPPoolLeases(pool, activeIPs)
				ch <- prometheus.MustNewConstMetric(c.poolUsed, prometheus.GaugeValue, used, pool.iface, pool.name)
				ch <- prometheus.MustNewConstMetric(c.poolUtilization, prometheus.GaugeValue, used/capacity, pool.iface, pool.name)
			}
		}
	} else {
		log.Printf("Error reading /conf/config.xml: %v", err)
	}

	ch <- prometheus.MustNewConstMetric(c.activeLeases, prometheus.GaugeValue, active)
	ch <- prometheus.MustNewConstMetric(c.staticLeases, prometheus.GaugeValue, static)
	ch <- prometheus.MustNewConstMetric(c.expiredLeases, prometheus.GaugeValue, expired)
}

func parseDHCPLeases(data []byte) (active, expired float64, activeIPs map[string]struct{}, err error) {
	activeIPs = make(map[string]struct{})
	var leases []map[string]any
	if err := json.Unmarshal(data, &leases); err != nil {
		return 0, 0, activeIPs, err
	}
	for _, lease := range leases {
		state, _ := lease["state"].(string)
		switch strings.ToLower(state) {
		case "active":
			active++
			for _, key := range []string{"address", "ip", "ipaddr", "ip_address"} {
				if address, ok := lease[key].(string); ok && ipv4ToUint32(net.ParseIP(address)) != 0 {
					activeIPs[address] = struct{}{}
					break
				}
			}
		case "expired":
			expired++
		}
	}
	return active, expired, activeIPs, nil
}

type dhcpXMLNode struct {
	XMLName  xml.Name
	Text     string        `xml:",chardata"`
	Children []dhcpXMLNode `xml:",any"`
}

func parseDHCPPools(data []byte) []dhcpPool {
	var root dhcpXMLNode
	if xml.Unmarshal(data, &root) != nil {
		return nil
	}
	var pools []dhcpPool

	if dhcpd := childNode(root, "dhcpd"); dhcpd != nil {
		for _, iface := range dhcpd.Children {
			for _, node := range descendants(iface, "range") {
				from := childText(node, "from")
				to := childText(node, "to")
				if pool, ok := newDHCPPool(iface.XMLName.Local, from+"-"+to); ok {
					pools = append(pools, pool)
				}
			}
		}
	}

	var walkKea func(dhcpXMLNode, string, bool)
	walkKea = func(node dhcpXMLNode, subnet string, inKea bool) {
		name := strings.ToLower(node.XMLName.Local)
		inKea = inKea || name == "kea"
		if inKea && name == "subnet" {
			subnet = strings.TrimSpace(node.Text)
		} else if inKea && name == "item" && subnet == "" {
			subnet = descendantText(node, "subnet")
		}
		if inKea && name == "pool" {
			if pool, ok := newDHCPPool("kea", strings.TrimSpace(node.Text)); ok {
				if subnet != "" {
					pool.name = subnet + " " + pool.name
				}
				pools = append(pools, pool)
			}
		}
		for _, child := range node.Children {
			walkKea(child, subnet, inKea)
		}
	}
	walkKea(root, "", false)
	return pools
}

func newDHCPPool(iface, value string) (dhcpPool, bool) {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "/") {
		ip, network, err := net.ParseCIDR(value)
		if err != nil {
			return dhcpPool{}, false
		}
		first := ipv4ToUint32(ip)
		if first == 0 || len(network.Mask) != net.IPv4len {
			return dhcpPool{}, false
		}
		mask := uint32(network.Mask[0])<<24 | uint32(network.Mask[1])<<16 | uint32(network.Mask[2])<<8 | uint32(network.Mask[3])
		first &= mask
		last := first | ^mask
		return dhcpPool{iface: iface, name: value, first: first, last: last}, true
	}

	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return dhcpPool{}, false
	}
	firstIP := ipv4ToUint32(net.ParseIP(strings.TrimSpace(parts[0])))
	lastIP := ipv4ToUint32(net.ParseIP(strings.TrimSpace(parts[1])))
	if firstIP == 0 || lastIP < firstIP {
		return dhcpPool{}, false
	}
	return dhcpPool{iface: iface, name: value, first: firstIP, last: lastIP}, true
}

func ipv4ToUint32(ip net.IP) uint32 {
	ipv4 := ip.To4()
	if ipv4 == nil {
		return 0
	}
	return uint32(ipv4[0])<<24 | uint32(ipv4[1])<<16 | uint32(ipv4[2])<<8 | uint32(ipv4[3])
}

func countDHCPPoolLeases(pool dhcpPool, addresses map[string]struct{}) float64 {
	used := float64(0)
	for address := range addresses {
		if ip := ipv4ToUint32(net.ParseIP(address)); ip >= pool.first && ip <= pool.last {
			used++
		}
	}
	return used
}

func childNode(node dhcpXMLNode, name string) *dhcpXMLNode {
	for i := range node.Children {
		if node.Children[i].XMLName.Local == name {
			return &node.Children[i]
		}
	}
	return nil
}

func childText(node dhcpXMLNode, name string) string {
	if child := childNode(node, name); child != nil {
		return strings.TrimSpace(child.Text)
	}
	return ""
}

func descendantText(node dhcpXMLNode, name string) string {
	for _, child := range node.Children {
		if strings.EqualFold(child.XMLName.Local, name) {
			return strings.TrimSpace(child.Text)
		}
		if value := descendantText(child, name); value != "" {
			return value
		}
	}
	return ""
}

func descendants(node dhcpXMLNode, name string) []dhcpXMLNode {
	var found []dhcpXMLNode
	for _, child := range node.Children {
		if child.XMLName.Local == name {
			found = append(found, child)
		}
		found = append(found, descendants(child, name)...)
	}
	return found
}
