package collector

import (
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"time"
	"context"

	"github.com/oschwald/geoip2-golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/saitama-op/freebsd-pf-exporter/internal"
)

type PFGeoCollector struct {
	dbPath string
	timeout time.Duration
	desc   *prometheus.Desc
}

func NewPFGeoCollector(dbPath string,timeout time.Duration) *PFGeoCollector {
	return &PFGeoCollector{
		dbPath: dbPath,
		timeout: timeout,
		desc: prometheus.NewDesc(
			"opnsense_active_connections_geo",
			"Count of active pf states aggregated by geographic location",
			[]string{"country", "city", "latitude", "longitude"},
			nil,
		),
	}
}

func (c *PFGeoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

// isPublicIP ensures we only look up external internet addresses
func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	return ip.IsGlobalUnicast() && !ip.IsPrivate()
}

func (c *PFGeoCollector) Collect(ch chan<- prometheus.Metric) {
	// 1. Get active firewall states natively from pf
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	out, err := internal.RunCommand(ctx, "pfctl", "-ss")
	if err != nil {
		log.Printf("Error running pfctl: %v", err)
		return
	}

	// 2. Open MaxMind Database
	db, err := geoip2.Open(c.dbPath)
	if err != nil {
		log.Printf("Error opening MaxMind DB: %v", err)
		return
	}
	defer db.Close()

	// Regex to extract IPv4 addresses (ignores ports)
	ipRegex := regexp.MustCompile(`\b(\d{1,3}(?:\.\d{1,3}){3})\b`)
	
	// Map to aggregate connection counts by location string
	locationCounts := make(map[string]int)
	
	// Map to store the label metadata for each location key
	type locData struct {
		country string
		city    string
		lat     string
		lon     string
	}
	locationMeta := make(map[string]locData)

	lines := strings.Split(string(out), "\n")
	
	for _, line := range lines {
		if line == "" {
			continue
		}

		// Find all IPs in this specific state line
		matches := ipRegex.FindAllStringSubmatch(line, -1)
		
		// Use a map to deduplicate IPs per state line (NAT causes the same IP to appear twice)
		uniquePublicIPs := make(map[string]net.IP)
		for _, match := range matches {
			ip := net.ParseIP(match[1])
			if isPublicIP(ip) {
				uniquePublicIPs[ip.String()] = ip
			}
		}

		// Lookup and aggregate
		for _, ip := range uniquePublicIPs {
if ctx.Err() != nil {
				log.Println("GeoIP processing aborted: Scrape timeout reached")
				return
			}
			record, err := db.City(ip)
			if err != nil || record.Location.Latitude == 0 {
				continue // Skip if not found or no coordinates
			}

			country := record.Country.Names["en"]
			city := record.City.Names["en"]
			lat := fmt.Sprintf("%f", record.Location.Latitude)
			lon := fmt.Sprintf("%f", record.Location.Longitude)

			if country == "" { country = "Unknown" }
			if city == "" { city = "Unknown" }

			// Create a unique key for the aggregator map
			locKey := fmt.Sprintf("%s|%s|%s|%s", country, city, lat, lon)
			
			locationCounts[locKey]++
			
			if _, exists := locationMeta[locKey]; !exists {
				locationMeta[locKey] = locData{
					country: country,
					city:    city,
					lat:     lat,
					lon:     lon,
				}
			}
		}
	}

	// 3. Emit the aggregated metrics
	for key, count := range locationCounts {
		meta := locationMeta[key]
		ch <- prometheus.MustNewConstMetric(
			c.desc,
			prometheus.GaugeValue,
			float64(count),
			meta.country, meta.city, meta.lat, meta.lon,
		)
	}
}
