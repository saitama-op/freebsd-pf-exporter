// collector/service.go
package collector

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/saitama-op/freebsd-pf-exporter/internal"
)

type ServiceCollector struct {
	timeout  time.Duration
	services []string
	upDesc   *prometheus.Desc
}

func NewServiceCollector(serviceList string, timeout time.Duration) *ServiceCollector {
	var services []string
	for _, s := range strings.Split(serviceList, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			services = append(services, s)
		}
	}

	return &ServiceCollector{
		timeout:  timeout,
		services: services,
		upDesc: prometheus.NewDesc(
			"opnsense_service_up",
			"Indicates if the specified FreeBSD/OPNsense service is running (1 = up, 0 = down)",
			[]string{"service"},
			nil,
		),
	}
}

func (c *ServiceCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.upDesc
}

func (c *ServiceCollector) Collect(ch chan<- prometheus.Metric) {
	if len(c.services) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	for _, srv := range c.services {
		if ctx.Err() != nil {
			log.Printf("Service collector timeout reached while checking %s", srv)
			break
		}

		// Run 'service <srv> onestatus' via the internal package helper
		_, err := internal.RunCommand(ctx, "service", srv, "onestatus")

		val := 0.0
		if err == nil {
			val = 1.0
		}

		ch <- prometheus.MustNewConstMetric(
			c.upDesc,
			prometheus.GaugeValue,
			val,
			srv,
		)
	}
}
