// cmd/exporter/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/saitama-op/freebsd-pf-exporter/collector"
)

var (
	version = "1.0.0"

	listenAddress = flag.String("web.listen-address", ":9101", "Address on which to expose metrics and web interface.")
	metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	scrapeTimeout = flag.Duration("scrape.timeout", 5*time.Second, "Timeout for scraping individual system commands.")

	// Collector flags
	pfEnabled        = flag.Bool("collector.pf", true, "Enable the PF firewall collector")
	gatewayEnabled   = flag.Bool("collector.gateway", true, "Enable the OPNsense gateway collector")
	mbufEnabled      = flag.Bool("collector.mbuf", true, "Enable the netstat mbuf collector")
	interfaceEnabled = flag.Bool("collector.interface", true, "Enable the interface traffic collector")
	systemEnabled    = flag.Bool("collector.system", true, "Enable the sysctl system collector")
	dnsEnabled       = flag.Bool("collector.dns", true, "Enable the unbound DNS collector")
	dhcpEnabled      = flag.Bool("collector.dhcp", true, "Enable the DHCP lease collector")
	geoIpEnabled    = flag.Bool("collector.geoip",true,"Enable the Geo IP location for PublicIps")
)

func main() {
	flag.Parse()

	log.Printf("Starting FreeBSD PF Exporter v%s", version)
	log.Printf("Listening on %s", *listenAddress)

	registry := prometheus.NewRegistry()

	// Register enabled collectors
	if *pfEnabled {
		registry.MustRegister(collector.NewPFCollector(*scrapeTimeout))
		log.Println("Collector enabled: pf")
	}
	if *gatewayEnabled {
		registry.MustRegister(collector.NewGatewayCollector(*scrapeTimeout))
		log.Println("Collector enabled: gateway")
	}
	if *mbufEnabled {
		registry.MustRegister(collector.NewMbufCollector(*scrapeTimeout))
		log.Println("Collector enabled: mbuf")
	}
	if *interfaceEnabled {
		registry.MustRegister(collector.NewInterfaceCollector(*scrapeTimeout))
		log.Println("Collector enabled: interface")
	}
	if *systemEnabled {
		registry.MustRegister(collector.NewSystemCollector(*scrapeTimeout))
		log.Println("Collector enabled: system")
	}
	if *dnsEnabled {
		registry.MustRegister(collector.NewDNSCollector(*scrapeTimeout))
		log.Println("Collector enabled: dns")
	}
	if *dhcpEnabled {
		registry.MustRegister(collector.NewDHCPCollector(*scrapeTimeout))
		log.Println("Collector enabled: dhcp")
	}
	if *geoIpEnabled{
		registry.MustRegister(collector.NewPFGeoCollector("/usr/local/share/GeoIP/GeoLite2-City.mmdb",*scrapeTimeout))
		log.Println("Collector enabled: geo ip")
	}

	// Handlers
	http.Handle(*metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		Timeout: *scrapeTimeout * 2,
	}))

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	http.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "FreeBSD PF Exporter v%s\n", version)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>FreeBSD PF Exporter</title></head>
             <body>
             <h1>FreeBSD PF Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		log.Fatalf("Error starting HTTP server: %v", err)
		os.Exit(1)
	}
}
