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
	metrics []dnsMetric
}

type dnsMetric struct {
	statKey string
	desc    *prometheus.Desc
	kind    prometheus.ValueType
}

func NewDNSCollector(timeout time.Duration) *DNSCollector {
	return &DNSCollector{
		timeout: timeout,
		metrics: []dnsMetric{
			{"total.num.queries", prometheus.NewDesc("opnsense_dns_queries_total", "Total number of DNS queries processed by Unbound", nil, nil), prometheus.CounterValue},
			{"total.num.cachehits", prometheus.NewDesc("opnsense_dns_cache_hits_total", "Total number of DNS cache hits", nil, nil), prometheus.CounterValue},
			{"total.num.cachemiss", prometheus.NewDesc("opnsense_dns_cache_misses_total", "Total number of DNS cache misses", nil, nil), prometheus.CounterValue},
			{"total.num.prefetch", prometheus.NewDesc("opnsense_dns_prefetches_total", "Total number of cache prefetches performed by Unbound", nil, nil), prometheus.CounterValue},
			{"total.num.expired", prometheus.NewDesc("opnsense_dns_expired_answers_total", "Total number of expired cache entries served by Unbound", nil, nil), prometheus.CounterValue},
			{"total.num.recursivereplies", prometheus.NewDesc("opnsense_dns_recursive_replies_total", "Total number of recursive replies sent by Unbound", nil, nil), prometheus.CounterValue},
			{"total.requestlist.avg", prometheus.NewDesc("opnsense_dns_requestlist_avg", "Average number of outstanding recursive requests in Unbound's request list", nil, nil), prometheus.GaugeValue},
			{"total.requestlist.max", prometheus.NewDesc("opnsense_dns_requestlist_max", "Maximum number of outstanding recursive requests in Unbound's request list", nil, nil), prometheus.GaugeValue},
			{"total.requestlist.overwritten", prometheus.NewDesc("opnsense_dns_requestlist_overwritten_total", "Total number of request list entries overwritten by Unbound", nil, nil), prometheus.CounterValue},
			{"total.requestlist.exceeded", prometheus.NewDesc("opnsense_dns_requestlist_exceeded_total", "Total number of requests that exceeded Unbound's request list limit", nil, nil), prometheus.CounterValue},
			{"msg.cache.count", prometheus.NewDesc("opnsense_dns_message_cache_entries", "Current number of entries in Unbound's message cache", nil, nil), prometheus.GaugeValue},
			{"rrset.cache.count", prometheus.NewDesc("opnsense_dns_rrset_cache_entries", "Current number of entries in Unbound's RRset cache", nil, nil), prometheus.GaugeValue},
			{"infra.cache.count", prometheus.NewDesc("opnsense_dns_infrastructure_cache_entries", "Current number of entries in Unbound's infrastructure cache", nil, nil), prometheus.GaugeValue},
			{"key.cache.count", prometheus.NewDesc("opnsense_dns_key_cache_entries", "Current number of entries in Unbound's key cache", nil, nil), prometheus.GaugeValue},
		},
	}
}

func (c *DNSCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range c.metrics {
		ch <- metric.desc
	}
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

	stats := parseUnboundStats(out)
	for _, metric := range c.metrics {
		if value, ok := stats[metric.statKey]; ok {
			ch <- prometheus.MustNewConstMetric(metric.desc, metric.kind, value)
		}
	}
}

func parseUnboundStats(data []byte) map[string]float64 {
	stats := make(map[string]float64)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		key, valueText, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !ok {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(valueText), 64)
		if err != nil {
			continue
		}
		stats[strings.TrimSpace(key)] = value
	}
	return stats
}
