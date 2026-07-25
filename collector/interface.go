// collector/interface.go
package collector

import (
	"bufio"
	"bytes"
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/saitama-op/freebsd-pf-exporter/internal"
)

type InterfaceCollector struct {
	timeout time.Duration

	rxBytes   *prometheus.Desc
	txBytes   *prometheus.Desc
	rxPackets *prometheus.Desc
	txPackets *prometheus.Desc
	rxErrors  *prometheus.Desc
	txErrors  *prometheus.Desc
	drops     *prometheus.Desc
}

func NewInterfaceCollector(timeout time.Duration) *InterfaceCollector {
	labels := []string{"interface"}
	return &InterfaceCollector{
		timeout: timeout,
		rxBytes:   prometheus.NewDesc("opnsense_interface_rx_bytes", "Received bytes on interface", labels, nil),
		txBytes:   prometheus.NewDesc("opnsense_interface_tx_bytes", "Transmitted bytes on interface", labels, nil),
		rxPackets: prometheus.NewDesc("opnsense_interface_rx_packets", "Received packets on interface", labels, nil),
		txPackets: prometheus.NewDesc("opnsense_interface_tx_packets", "Transmitted packets on interface", labels, nil),
		rxErrors:  prometheus.NewDesc("opnsense_interface_rx_errors", "Receive errors on interface", labels, nil),
		txErrors:  prometheus.NewDesc("opnsense_interface_tx_errors", "Transmit errors on interface", labels, nil),
		drops:     prometheus.NewDesc("opnsense_interface_drops", "Dropped packets on interface", labels, nil),
	}
}

func (c *InterfaceCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.rxBytes
	ch <- c.txBytes
	ch <- c.rxPackets
	ch <- c.txPackets
	ch <- c.rxErrors
	ch <- c.txErrors
	ch <- c.drops
}

func (c *InterfaceCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// -i: interfaces, -b: bytes, -n: numeric (skip DNS resolution for speed)
	out, err := internal.RunCommand(ctx, "netstat", "-i", "-b", "-n")
	if err != nil {
		log.Printf("Error running netstat -ibn: %v", err)
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	seen := make(map[string]bool)

	// Skip the header line
	scanner.Scan()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.Fields(line)

		// We need at least 10 columns to safely index from the right
		if len(parts) < 10 {
			continue
		}

		iface := parts[0]

		// netstat prints multiple lines per interface (Link layer, IPv4, IPv6).
		// The first line is the Link layer, which holds the cumulative hardware counters.
		if seen[iface] {
			continue
		}
		seen[iface] = true

		// Read from right to left to avoid the shifting "Address" column issue
		// Coll (last, -1), Obytes (-2), Oerrs (-3), Opkts (-4), Ibytes (-5), Idrop (-6), Ierrs (-7), Ipkts (-8)
		l := len(parts)

		if val, err := strconv.ParseFloat(parts[l-5], 64); err == nil {
			ch <- prometheus.MustNewConstMetric(c.rxBytes, prometheus.CounterValue, val, iface)
		}
		if val, err := strconv.ParseFloat(parts[l-2], 64); err == nil {
			ch <- prometheus.MustNewConstMetric(c.txBytes, prometheus.CounterValue, val, iface)
		}
		if val, err := strconv.ParseFloat(parts[l-8], 64); err == nil {
			ch <- prometheus.MustNewConstMetric(c.rxPackets, prometheus.CounterValue, val, iface)
		}
		if val, err := strconv.ParseFloat(parts[l-4], 64); err == nil {
			ch <- prometheus.MustNewConstMetric(c.txPackets, prometheus.CounterValue, val, iface)
		}
		if val, err := strconv.ParseFloat(parts[l-7], 64); err == nil {
			ch <- prometheus.MustNewConstMetric(c.rxErrors, prometheus.CounterValue, val, iface)
		}
		if val, err := strconv.ParseFloat(parts[l-3], 64); err == nil {
			ch <- prometheus.MustNewConstMetric(c.txErrors, prometheus.CounterValue, val, iface)
		}
		if val, err := strconv.ParseFloat(parts[l-6], 64); err == nil {
			ch <- prometheus.MustNewConstMetric(c.drops, prometheus.CounterValue, val, iface)
		}
	}
}
