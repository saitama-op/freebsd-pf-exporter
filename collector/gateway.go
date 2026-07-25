// collector/gateway.go
package collector

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/saitama-op/freebsd-pf-exporter/internal"
)

type GatewayCollector struct {
	timeout time.Duration

	latency *prometheus.Desc
	loss    *prometheus.Desc
	up      *prometheus.Desc
}

func NewGatewayCollector(timeout time.Duration) *GatewayCollector {
	labels := []string{"gateway"}
	return &GatewayCollector{
		timeout: timeout,
		latency: prometheus.NewDesc("opnsense_gateway_latency_ms", "Gateway latency in ms", labels, nil),
		loss:    prometheus.NewDesc("opnsense_gateway_loss_percent", "Gateway packet loss percentage", labels, nil),
		up:      prometheus.NewDesc("opnsense_gateway_up", "Gateway status (1 = up, 0 = down)", labels, nil),
	}
}

func (c *GatewayCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.latency
	ch <- c.loss
	ch <- c.up
}

func (c *GatewayCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Using full absolute path /usr/local/sbin/pluginctl
	out, err := internal.RunCommand(ctx, "/usr/local/sbin/pluginctl", "-r", "return_gateways_status")
	if err != nil {
		log.Printf("Error running pluginctl return_gateways_status: %v", err)
		return
	}

	var data struct {
		Dpinger map[string]struct {
			Status string `json:"status"`
			Delay  string `json:"delay"`
			Loss   string `json:"loss"`
			Name   string `json:"name"`
		} `json:"dpinger"`
	}

	if err := json.Unmarshal(out, &data); err != nil {
		log.Printf("Error parsing gateway json: %v", err)
		return
	}

	for name, gw := range data.Dpinger {
		delayParts := strings.Fields(gw.Delay)
		if len(delayParts) > 0 && delayParts[0] != "~" {
			if delay, err := strconv.ParseFloat(delayParts[0], 64); err == nil {
				ch <- prometheus.MustNewConstMetric(c.latency, prometheus.GaugeValue, delay, name)
			}
		}

		lossParts := strings.Fields(gw.Loss)
		if len(lossParts) > 0 && lossParts[0] != "~" {
			if loss, err := strconv.ParseFloat(lossParts[0], 64); err == nil {
				ch <- prometheus.MustNewConstMetric(c.loss, prometheus.GaugeValue, loss, name)
			}
		}

		up := 0.0
		if gw.Status == "none" || strings.ToLower(gw.Status) == "online" {
			up = 1.0
		}
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, up, name)
	}
}
