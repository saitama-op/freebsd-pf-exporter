// collector/interface.go
package collector

import (
	"bufio"
	"bytes"
	"context"
	"log"
	"regexp"
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
	linkUp    *prometheus.Desc
	linkSpeed *prometheus.Desc
}

func NewInterfaceCollector(timeout time.Duration) *InterfaceCollector {
	labels := []string{"interface"}
	return &InterfaceCollector{
		timeout:   timeout,
		rxBytes:   prometheus.NewDesc("opnsense_interface_rx_bytes", "Received bytes on interface", labels, nil),
		txBytes:   prometheus.NewDesc("opnsense_interface_tx_bytes", "Transmitted bytes on interface", labels, nil),
		rxPackets: prometheus.NewDesc("opnsense_interface_rx_packets", "Received packets on interface", labels, nil),
		txPackets: prometheus.NewDesc("opnsense_interface_tx_packets", "Transmitted packets on interface", labels, nil),
		rxErrors:  prometheus.NewDesc("opnsense_interface_rx_errors", "Receive errors on interface", labels, nil),
		txErrors:  prometheus.NewDesc("opnsense_interface_tx_errors", "Transmit errors on interface", labels, nil),
		drops:     prometheus.NewDesc("opnsense_interface_drops", "Dropped packets on interface", labels, nil),
		linkUp:    prometheus.NewDesc("opnsense_interface_link_up", "Whether the interface link is active (1 = active, 0 = inactive)", labels, nil),
		linkSpeed: prometheus.NewDesc("opnsense_interface_link_speed_mbps", "Reported interface link speed in Mbps; zero if unavailable or inactive", labels, nil),
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
	ch <- c.linkUp
	ch <- c.linkSpeed
}

func (c *InterfaceCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// -i: interfaces, -b: bytes, -n: numeric (skip DNS resolution for speed)
	out, err := internal.RunCommand(ctx, "netstat", "-i", "-b", "-n")
	if err != nil {
		log.Printf("Error running netstat -ibn: %v", err)
	} else {
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

	linkOut, err := internal.RunCommand(ctx, "ifconfig", "-a")
	if err != nil {
		log.Printf("Error running ifconfig -a: %v", err)
		return
	}
	for iface, state := range parseInterfaceLinkState(linkOut) {
		up := 0.0
		speed := 0.0
		if state.active {
			up = 1
			speed = state.speedMbps
		}
		ch <- prometheus.MustNewConstMetric(c.linkUp, prometheus.GaugeValue, up, iface)
		ch <- prometheus.MustNewConstMetric(c.linkSpeed, prometheus.GaugeValue, speed, iface)
	}
}

type interfaceLinkState struct {
	active    bool
	speedMbps float64
}

var interfaceSpeedPattern = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)([kmg]?)base`)

func parseInterfaceLinkState(data []byte) map[string]interfaceLinkState {
	states := make(map[string]interfaceLinkState)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var current string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		flagsIndex := strings.Index(line, "flags=")
		if flagsIndex > 0 && strings.Contains(line[:flagsIndex], ":") {
			current = strings.TrimSpace(strings.SplitN(line[:flagsIndex], ":", 2)[0])
			if current != "" {
				states[current] = interfaceLinkState{}
			}
			continue
		}
		if current == "" {
			continue
		}

		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "status:") {
			states[current] = interfaceLinkState{
				active:    strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(lower, "status:")), "active"),
				speedMbps: states[current].speedMbps,
			}
		}
		if strings.HasPrefix(lower, "media:") {
			if speed := parseInterfaceSpeed(line); speed > 0 {
				state := states[current]
				state.speedMbps = speed
				states[current] = state
			}
		}
	}
	return states
}

func parseInterfaceSpeed(media string) float64 {
	match := interfaceSpeedPattern.FindStringSubmatch(media)
	if len(match) != 3 {
		return 0
	}
	speed, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(match[2]) {
	case "g":
		speed *= 1000
	case "k":
		speed /= 1000
	}
	return speed
}
