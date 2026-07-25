// collector/mbuf.go
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

type MbufCollector struct {
	timeout time.Duration
	used    *prometheus.Desc
}

func NewMbufCollector(timeout time.Duration) *MbufCollector {
	return &MbufCollector{
		timeout: timeout,
		used:    prometheus.NewDesc("opnsense_mbuf_used", "Number of mbufs currently in use", nil, nil),
	}
}

func (c *MbufCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.used
}

func (c *MbufCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	out, err := internal.RunCommand(ctx, "netstat", "-m")
	if err != nil {
		log.Printf("Error running netstat: %v", err)
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "mbufs in use") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				if strings.Contains(parts[0], "/") {
					// Handle format like 1234/5678/9000 mbufs in use
					valParts := strings.Split(parts[0], "/")
					if val, err := strconv.ParseFloat(valParts[0], 64); err == nil {
						ch <- prometheus.MustNewConstMetric(c.used, prometheus.GaugeValue, val)
					}
				} else {
					if val, err := strconv.ParseFloat(parts[0], 64); err == nil {
						ch <- prometheus.MustNewConstMetric(c.used, prometheus.GaugeValue, val)
					}
				}
			}
			break
		}
	}
}
