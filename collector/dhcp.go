// collector/dhcp.go
package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/saitama-op/freebsd-pf-exporter/internal"
)

type DHCPCollector struct {
	timeout time.Duration

	activeLeases  *prometheus.Desc
	staticLeases  *prometheus.Desc
	expiredLeases *prometheus.Desc
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
	}
}

func (c *DHCPCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.activeLeases
	ch <- c.staticLeases
	ch <- c.expiredLeases
}

func (c *DHCPCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	var active, expired float64

	// 1. Check for any active dynamic leases
	out, err := internal.RunCommand(ctx, "configctl", "dhcpd", "list", "leases")
	if err == nil {
		var leases []struct {
			State string `json:"state"`
		}
		if json.Unmarshal(out, &leases) == nil {
			for _, lease := range leases {
				state := strings.ToLower(lease.State)
				if state == "active" {
					active++
				} else if state == "expired" {
					expired++
				}
			}
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
	} else {
		log.Printf("Error reading /conf/config.xml: %v", err)
	}

	ch <- prometheus.MustNewConstMetric(c.activeLeases, prometheus.GaugeValue, active)
	ch <- prometheus.MustNewConstMetric(c.staticLeases, prometheus.GaugeValue, static)
	ch <- prometheus.MustNewConstMetric(c.expiredLeases, prometheus.GaugeValue, expired)
}
