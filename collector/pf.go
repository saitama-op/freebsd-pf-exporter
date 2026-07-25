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

type PFCollector struct {
	timeout time.Duration

	statesCurrent *prometheus.Desc
	searches      *prometheus.Desc
	inserts       *prometheus.Desc
	removals      *prometheus.Desc
	matched       *prometheus.Desc
	badOffset     *prometheus.Desc
	fragment      *prometheus.Desc
	shortPacket   *prometheus.Desc
	badTimestamp  *prometheus.Desc
	congestion    *prometheus.Desc
	stateLimit    *prometheus.Desc
}

func NewPFCollector(timeout time.Duration) *PFCollector {
	return &PFCollector{
		timeout: timeout,
		statesCurrent: prometheus.NewDesc("opnsense_pf_states_current", "Current number of entries in the state table", nil, nil),
		searches:      prometheus.NewDesc("opnsense_pf_states_searches_total", "Total number of state table searches", nil, nil),
		inserts:       prometheus.NewDesc("opnsense_pf_states_inserted_total", "Total number of state table inserts", nil, nil),
		removals:      prometheus.NewDesc("opnsense_pf_states_removed_total", "Total number of state table removals", nil, nil),
		matched:       prometheus.NewDesc("opnsense_pf_packets_matched_total", "Total number of packets that matched a rule", nil, nil),
		badOffset:     prometheus.NewDesc("opnsense_pf_bad_offset_total", "Total number of bad offset packets", nil, nil),
		fragment:      prometheus.NewDesc("opnsense_pf_fragment_total", "Total number of fragmented packets", nil, nil),
		shortPacket:   prometheus.NewDesc("opnsense_pf_short_packet_total", "Total number of short packets", nil, nil),
		badTimestamp:  prometheus.NewDesc("opnsense_pf_bad_timestamp_total", "Total number of bad timestamp packets", nil, nil),
		congestion:    prometheus.NewDesc("opnsense_pf_congestion_total", "Total number of congestion drops", nil, nil),
		stateLimit:    prometheus.NewDesc("opnsense_pf_state_limit", "Maximum number of entries in the state table", nil, nil),
	}
}

func (c *PFCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.statesCurrent
	ch <- c.searches
	ch <- c.inserts
	ch <- c.removals
	ch <- c.matched
	ch <- c.badOffset
	ch <- c.fragment
	ch <- c.shortPacket
	ch <- c.badTimestamp
	ch <- c.congestion
	ch <- c.stateLimit
}

func (c *PFCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// 1. Gather Info (pfctl -si)
	outInfo, err := internal.RunCommand(ctx, "pfctl", "-si")
	if err == nil {
		c.parseInfo(outInfo, ch)
	} else {
		log.Printf("Error running pfctl -si: %v", err)
	}

	// 2. Gather Limits (pfctl -sm)
	outMem, err := internal.RunCommand(ctx, "pfctl", "-sm")
	if err == nil {
		c.parseMem(outMem, ch)
	} else {
		log.Printf("Error running pfctl -sm: %v", err)
	}
}

func (c *PFCollector) parseInfo(data []byte, ch chan<- prometheus.Metric) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.Fields(line)

		if len(parts) < 2 {
			continue
		}

		key := parts[0]

		// Handle "current entries 753"
		if key == "current" && parts[1] == "entries" && len(parts) >= 3 {
			if val, err := strconv.ParseFloat(parts[2], 64); err == nil {
				ch <- prometheus.MustNewConstMetric(c.statesCurrent, prometheus.GaugeValue, val)
			}
			continue
		}

		// Handle remaining counters: <key> <value> [<rate>]
		valStr := parts[1]
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}

		switch key {
		case "searches":
			ch <- prometheus.MustNewConstMetric(c.searches, prometheus.CounterValue, val)
		case "inserts":
			ch <- prometheus.MustNewConstMetric(c.inserts, prometheus.CounterValue, val)
		case "removals":
			ch <- prometheus.MustNewConstMetric(c.removals, prometheus.CounterValue, val)
		case "match":
			ch <- prometheus.MustNewConstMetric(c.matched, prometheus.CounterValue, val)
		case "bad-offset":
			ch <- prometheus.MustNewConstMetric(c.badOffset, prometheus.CounterValue, val)
		case "fragment":
			ch <- prometheus.MustNewConstMetric(c.fragment, prometheus.CounterValue, val)
		case "short":
			ch <- prometheus.MustNewConstMetric(c.shortPacket, prometheus.CounterValue, val)
		case "bad-timestamp":
			ch <- prometheus.MustNewConstMetric(c.badTimestamp, prometheus.CounterValue, val)
		case "congestion":
			ch <- prometheus.MustNewConstMetric(c.congestion, prometheus.CounterValue, val)
		}
	}
}

func (c *PFCollector) parseMem(data []byte, ch chan<- prometheus.Metric) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "states") {
			parts := strings.Fields(line)
			// Example line: states        hard limit  1000000
			if len(parts) >= 4 {
				if val, err := strconv.ParseFloat(parts[3], 64); err == nil {
					ch <- prometheus.MustNewConstMetric(c.stateLimit, prometheus.GaugeValue, val)
				}
			}
		}
	}
}
