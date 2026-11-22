package collector

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/saitama-op/freebsd-pf-exporter/internal"
)

type WireGuardCollector struct {
	timeout time.Duration

	latestHandshake *prometheus.Desc
	receiveBytes    *prometheus.Desc
	transmitBytes   *prometheus.Desc
}

type wireGuardPeer struct {
	interfaceName   string
	publicKey       string
	latestHandshake float64
	receiveBytes    float64
	transmitBytes   float64
}

func NewWireGuardCollector(timeout time.Duration) *WireGuardCollector {
	labels := []string{"interface", "peer"}
	return &WireGuardCollector{
		timeout: timeout,
		latestHandshake: prometheus.NewDesc(
			"opnsense_wireguard_peer_latest_handshake_timestamp_seconds",
			"Unix timestamp of the peer's latest successful WireGuard handshake; zero if it has never connected",
			labels, nil,
		),
		receiveBytes: prometheus.NewDesc(
			"opnsense_wireguard_peer_receive_bytes_total",
			"Total bytes received from the WireGuard peer",
			labels, nil,
		),
		transmitBytes: prometheus.NewDesc(
			"opnsense_wireguard_peer_transmit_bytes_total",
			"Total bytes transmitted to the WireGuard peer",
			labels, nil,
		),
	}
}

func (c *WireGuardCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.latestHandshake
	ch <- c.receiveBytes
	ch <- c.transmitBytes
}

func (c *WireGuardCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	out, err := internal.RunCommand(ctx, "wg", "show", "all", "dump")
	if err != nil {
		log.Printf("Error running wg show all dump: %v", err)
		return
	}

	peers, err := parseWireGuardDump(out)
	if err != nil {
		log.Printf("Error parsing WireGuard dump: %v", err)
		return
	}

	for _, peer := range peers {
		ch <- prometheus.MustNewConstMetric(
			c.latestHandshake, prometheus.GaugeValue, peer.latestHandshake,
			peer.interfaceName, peer.publicKey,
		)
		ch <- prometheus.MustNewConstMetric(
			c.receiveBytes, prometheus.CounterValue, peer.receiveBytes,
			peer.interfaceName, peer.publicKey,
		)
		ch <- prometheus.MustNewConstMetric(
			c.transmitBytes, prometheus.CounterValue, peer.transmitBytes,
			peer.interfaceName, peer.publicKey,
		)
	}
}

func parseWireGuardDump(data []byte) ([]wireGuardPeer, error) {
	var peers []wireGuardPeer
	for lineNumber, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		// Interface rows have five fields; peer rows have nine.
		if len(fields) == 5 {
			continue
		}
		if len(fields) != 9 {
			return nil, fmt.Errorf("line %d: expected 5 interface or 9 peer fields, got %d", lineNumber+1, len(fields))
		}

		handshake, err := strconv.ParseUint(fields[5], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid latest handshake: %w", lineNumber+1, err)
		}
		received, err := strconv.ParseUint(fields[6], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid received byte count: %w", lineNumber+1, err)
		}
		transmitted, err := strconv.ParseUint(fields[7], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid transmitted byte count: %w", lineNumber+1, err)
		}

		peers = append(peers, wireGuardPeer{
			interfaceName:   fields[0],
			publicKey:       fields[1],
			latestHandshake: float64(handshake),
			receiveBytes:    float64(received),
			transmitBytes:   float64(transmitted),
		})
	}

	return peers, nil
}
