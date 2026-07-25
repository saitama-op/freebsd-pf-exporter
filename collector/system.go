// collector/system.go
package collector

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/saitama-op/freebsd-pf-exporter/internal"
)

type SystemCollector struct {
	timeout time.Duration

	temperature  *prometheus.Desc
	cpuFrequency *prometheus.Desc
	bootTime     *prometheus.Desc
	uptime       *prometheus.Desc
}

func NewSystemCollector(timeout time.Duration) *SystemCollector {
	return &SystemCollector{
		timeout: timeout,
		temperature: prometheus.NewDesc(
			"opnsense_temperature",
			"CPU/System temperature in degrees Celsius",
			[]string{"sensor"}, nil,
		),
		cpuFrequency: prometheus.NewDesc(
			"opnsense_cpu_frequency",
			"CPU frequency in MHz",
			[]string{"cpu"}, nil,
		),
		bootTime: prometheus.NewDesc(
			"opnsense_boot_time",
			"System boot time as Unix timestamp",
			nil, nil,
		),
		uptime: prometheus.NewDesc(
			"opnsense_uptime",
			"System uptime in seconds",
			nil, nil,
		),
	}
}

func (c *SystemCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.temperature
	ch <- c.cpuFrequency
	ch <- c.bootTime
	ch <- c.uptime
}

func (c *SystemCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// 1. CPU Temperatures (dev.cpu.0.temperature, tz0, etc.)
	if out, err := internal.RunCommand(ctx, "sysctl", "-n", "dev.cpu.0.temperature"); err == nil {
		valStr := strings.TrimSuffix(strings.TrimSpace(string(out)), "C")
		if val, err := strconv.ParseFloat(valStr, 64); err == nil {
			ch <- prometheus.MustNewConstMetric(c.temperature, prometheus.GaugeValue, val, "cpu0")
		}
	} else if out, err := internal.RunCommand(ctx, "sysctl", "-n", "hw.acpi.thermal.tz0.temperature"); err == nil {
		valStr := strings.TrimSuffix(strings.TrimSpace(string(out)), "C")
		if val, err := strconv.ParseFloat(valStr, 64); err == nil {
			ch <- prometheus.MustNewConstMetric(c.temperature, prometheus.GaugeValue, val, "tz0")
		}
	}

	// 2. CPU Frequency
	if out, err := internal.RunCommand(ctx, "sysctl", "-n", "dev.cpu.0.freq"); err == nil {
		if val, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil {
			ch <- prometheus.MustNewConstMetric(c.cpuFrequency, prometheus.GaugeValue, val, "cpu0")
		}
	}

	// 3. Boot time & Uptime
	if out, err := internal.RunCommand(ctx, "sysctl", "-n", "kern.boottime"); err == nil {
		// Output format: { sec = 1715000000, usec = 123456 } ...
		str := string(out)
		if idx := strings.Index(str, "sec = "); idx != -1 {
			sub := str[idx+6:]
			if endIdx := strings.Index(sub, ","); endIdx != -1 {
				if bootSec, err := strconv.ParseFloat(sub[:endIdx], 64); err == nil {
					ch <- prometheus.MustNewConstMetric(c.bootTime, prometheus.GaugeValue, bootSec)
					uptimeSec := float64(time.Now().Unix()) - bootSec
					if uptimeSec >= 0 {
						ch <- prometheus.MustNewConstMetric(c.uptime, prometheus.GaugeValue, uptimeSec)
					}
				}
			}
		}
	} else {
		log.Printf("Error querying sysctl kern.boottime: %v", err)
	}
}
