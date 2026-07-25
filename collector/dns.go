// collector/dns.go
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

type DNSCollector struct {
	timeout time.Duration

	queriesTotal   *prometheus.Desc
	cacheHitsTotal *prometheus.Desc
	cacheMissTotal *prometheus.Desc
}

func NewDNSCollector(timeout time.Duration) *DNSCollector {
	return &DNSCollector{
		timeout: timeout,
		queriesTotal: prometheus.NewDesc(
			"opnsense_dns_queries_total",
			"Total number of DNS queries processed by Unbound",
			nil, nil,
		),
		cacheHitsTotal: prometheus.NewDesc(
			"opnsense_dns_cache_hits_total",
			"Total number of DNS cache hits",
			nil, nil,
		),
		cacheMissTotal: prometheus.NewDesc(
			"opnsense_dns_cache_misses_total",
			"Total number of DNS cache misses",
			nil, nil,
		),
	}
}

func (c *DNSCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.queriesTotal
	ch <- c.cacheHitsTotal
	ch <- c.cacheMissTotal
}

func (c *DNSCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Using full absolute path /usr/local/sbin/unbound-control
	out, err := internal.RunCommand(ctx, "/usr/local/sbin/unbound-control", "-c", "/var/unbound/unbound.conf", "stats_noreset")
	if err != nil {
		log.Printf("Error running unbound-control: %v", err)
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		val, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			continue
		}

		switch key {
		case "total.num.queries":
			ch <- prometheus.MustNewConstMetric(c.queriesTotal, prometheus.CounterValue, val)
		case "total.num.cachehits":
			ch <- prometheus.MustNewConstMetric(c.cacheHitsTotal, prometheus.CounterValue, val)
		case "total.num.cachemiss":
			ch <- prometheus.MustNewConstMetric(c.cacheMissTotal, prometheus.CounterValue, val)
		}
	}
}
