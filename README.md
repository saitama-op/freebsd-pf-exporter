# FreeBSD PF Exporter (`freebsd-pf-exporter`)

A lightweight, high-performance Prometheus exporter written in Go for **OPNsense** and **FreeBSD PF-based firewalls**.

This exporter captures firewall, gateway, memory buffer, service status, and network state metrics natively from FreeBSD system utilities (`pfctl`, `sysctl`, `netstat`, `pgrep`, `configctl`, `unbound-control`) that are not provided by standard Linux exporters like `node_exporter`.

---

## Features

* **Native & Lightweight:** Written in Go 1.24+ with zero CGO dependencies. Runs with < 10 MB RAM footprint and < 100 ms scrape latency.
* **Non-Blocking Architecture:** Executes system commands cleanly on HTTP scrape using strict context timeouts (`exec.CommandContext`).
* **Service Monitoring:** Tracks running FreeBSD/OPNsense background services (`unbound`, `suricata`, `dpinger`, `kea-dhcp4`) and in-kernel interfaces (`wireguard`) via `/bin/pgrep` and `/sbin/ifconfig`.
* **GeoIP State Tracking:** Parses live PF state tables (`pfctl -ss`), cross-references external public connections against MaxMind's `GeoLite2-City` database, and aggregates counts by location (latitude/longitude) for Grafana Geomap visualization without high-cardinality memory spikes.
* **Granular Control:** Toggle individual collectors on/off using command-line flags.

---

## Project Structure

```text
freebsd-pf-exporter/
├── cmd/
│   └── exporter/
│       └── main.go
├── collector/
│   ├── dhcp.go
│   ├── dns.go
│   ├── gateway.go
│   ├── interface.go
│   ├── mbuf.go
│   ├── pf.go
│   ├── pf_geo.go
│   ├── service.go
│   └── system.go
├── internal/
│   ├── exec.go
│   ├── parser.go
│   └── util.go
├── rc.d/
│   └── freebsd_pf_exporter
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## Exported Metrics

### 1. PF Firewall (`-collector.pf`)

| Metric Name | Type | Description |
| --- | --- | --- |
| `opnsense_pf_states_current` | Gauge | Current number of active state entries in the PF state table |
| `opnsense_pf_states_searches_total` | Counter | Total state searches |
| `opnsense_pf_states_inserted_total` | Counter | Total state insertions |
| `opnsense_pf_states_removed_total` | Counter | Total state removals |
| `opnsense_pf_packets_passed_total` | Counter | Total packets passed by firewall |
| `opnsense_pf_packets_blocked_total` | Counter | Total packets blocked by firewall |

### 2. Interface Traffic & Errors (`-collector.interface`)

| Metric Name | Type | Labels | Description |
| --- | --- | --- | --- |
| `opnsense_interface_rx_bytes` | Counter | `interface` | Received bytes count |
| `opnsense_interface_tx_bytes` | Counter | `interface` | Transmitted bytes count |
| `opnsense_interface_rx_errors` | Counter | `interface` | Inbound network error count |
| `opnsense_interface_tx_errors` | Counter | `interface` | Outbound network error count |
| `opnsense_interface_drops` | Counter | `interface` | Dropped packet count |
| `opnsense_interface_link_up` | Gauge | `interface` | Link state (`1` = active, `0` = inactive) |
| `opnsense_interface_link_speed_mbps` | Gauge | `interface` | Reported link speed in Mbps; `0` if unavailable or inactive |

### 3. Service Status (`-collector.service`)

| Metric Name | Type | Labels | Description |
| --- | --- | --- | --- |
| `opnsense_service_up` | Gauge | `service` | Service health status (`1` = running/active, `0` = down) |

### 4. Active Connections GeoIP (`-collector.geoip`)

| Metric Name | Type | Labels | Description |
| --- | --- | --- | --- |
| `opnsense_active_connections_geo` | Gauge | `country`, `city`, `latitude`, `longitude` | Aggregated count of active firewall state connections by geographic coordinates |

### 5. Gateway Health (`-collector.gateway`)

| Metric Name | Type | Labels | Description |
| --- | --- | --- | --- |
| `opnsense_gateway_up` | Gauge | `gateway` | Gateway connectivity state (`1` = up, `0` = down) |
| `opnsense_gateway_latency_ms` | Gauge | `gateway` | Gateway round-trip latency in milliseconds |
| `opnsense_gateway_loss_percent` | Gauge | `gateway` | Packet loss percentage |

### 6. Memory Buffers & System (`-collector.mbuf`, `-collector.system`)

| Metric Name | Type | Labels | Description |
| --- | --- | --- | --- |
| `opnsense_mbuf_used` | Gauge | None | Number of networking memory buffers (mbufs) currently in use |
| `opnsense_temperature` | Gauge | `core` | CPU core temperature in Celsius |
| `opnsense_uptime` | Gauge | None | System uptime in seconds |

### 7. WireGuard Peers (`-collector.wireguard`)

| Metric Name | Type | Labels | Description |
| --- | --- | --- | --- |
| `opnsense_wireguard_peer_latest_handshake_timestamp_seconds` | Gauge | `interface`, `peer` | Unix timestamp of the peer's latest successful handshake; `0` if it has never connected |
| `opnsense_wireguard_peer_receive_bytes_total` | Counter | `interface`, `peer` | Total bytes received from the peer |
| `opnsense_wireguard_peer_transmit_bytes_total` | Counter | `interface`, `peer` | Total bytes transmitted to the peer |

The collector reads `wg show all dump`. The `peer` label is the peer's public key; endpoint addresses are not exported.

### 8. Unbound DNS (`-collector.dns`)

| Metric Name | Type | Description |
| --- | --- | --- |
| `opnsense_dns_queries_total` | Counter | Total DNS queries processed |
| `opnsense_dns_cache_hits_total` | Counter | Total cache hits |
| `opnsense_dns_cache_misses_total` | Counter | Total cache misses |
| `opnsense_dns_prefetches_total` | Counter | Total cache prefetches |
| `opnsense_dns_expired_answers_total` | Counter | Total expired cache entries served |
| `opnsense_dns_recursive_replies_total` | Counter | Total recursive replies |
| `opnsense_dns_requestlist_avg` | Gauge | Average outstanding recursive requests |
| `opnsense_dns_requestlist_max` | Gauge | Maximum outstanding recursive requests |
| `opnsense_dns_requestlist_overwritten_total` | Counter | Request-list entries overwritten |
| `opnsense_dns_requestlist_exceeded_total` | Counter | Requests that exceeded the request-list limit |
| `opnsense_dns_message_cache_entries` | Gauge | Current message-cache entries |
| `opnsense_dns_rrset_cache_entries` | Gauge | Current RRset-cache entries |
| `opnsense_dns_infrastructure_cache_entries` | Gauge | Current infrastructure-cache entries |
| `opnsense_dns_key_cache_entries` | Gauge | Current key-cache entries |

Unbound versions may omit some statistics; only metrics present in `unbound-control stats_noreset` output are emitted.

### 9. DHCP Pool Utilization (`-collector.dhcp`)

| Metric Name | Type | Labels | Description |
| --- | --- | --- | --- |
| `opnsense_dhcp_pool_capacity_addresses` | Gauge | `interface`, `pool` | Number of IPv4 addresses configured in the pool |
| `opnsense_dhcp_pool_used_addresses` | Gauge | `interface`, `pool` | Number of active dynamic leases whose IP addresses fall within the pool |
| `opnsense_dhcp_pool_utilization_ratio` | Gauge | `interface`, `pool` | Used addresses divided by pool capacity, from `0` to `1` |

Pool metrics support legacy DHCP `<range>` entries and Kea IPv4 pools in `/conf/config.xml`. Usage is based on active lease entries that include a recognizable IPv4 address; pools with no matching active leases report zero used. These metrics do not include static reservations unless they also appear in the active lease list.

---

## Building and Installation

### Cross-Compiling for OPNsense (FreeBSD)

Since OPNsense runs on FreeBSD `amd64`, you can build the binary on any Linux development host or build server:

```bash
# Clone repository
git clone https://github.com/saitama-op/freebsd-pf-exporter.git
cd freebsd-pf-exporter

# Cross-compile static FreeBSD binary
GOOS=freebsd GOARCH=amd64 go build -ldflags="-s -w" -o freebsd-pf-exporter ./cmd/exporter
```

### Installation on OPNsense

1. Transfer the binary and MaxMind GeoIP database to OPNsense via `scp`:

```bash
scp freebsd-pf-exporter root@<OPNSENSE_IP>:/usr/local/bin/
scp GeoLite2-City.mmdb root@<OPNSENSE_IP>:/usr/local/share/GeoIP/
```

2. SSH into OPNsense and set execution permissions:

```bash
chmod +x /usr/local/bin/freebsd-pf-exporter
```

---

## Running as a FreeBSD Service (`rc.d`)

To run `freebsd-pf-exporter` automatically at system startup on OPNsense:

1. Create `/usr/local/etc/rc.d/freebsd_pf_exporter`:

```sh
#!/bin/sh

# PROVIDE: freebsd_pf_exporter
# REQUIRE: LOGIN
# KEYWORD: shutdown

. /etc/rc.subr

name="freebsd_pf_exporter"
rcvar="freebsd_pf_exporter_enable"

command="/usr/local/bin/freebsd-pf-exporter"
command_args="&"

load_rc_config $name
: ${freebsd_pf_exporter_enable:="NO"}

run_rc_command "$1"
```

2. Set script permissions and enable the service in `/etc/rc.conf`:

```bash
chmod +x /usr/local/etc/rc.d/freebsd_pf_exporter
echo 'freebsd_pf_exporter_enable="YES"' >> /etc/rc.conf
echo 'freebsd_pf_exporter_flags="-web.listen-address=:9101 -collector.service.services=unbound,wireguard,dpinger,suricata,kea-dhcp4"' >> /etc/rc.conf
```

3. Start the service:

```bash
service freebsd_pf_exporter start
```

---

## Configuration Flags

```text
Usage of freebsd-pf-exporter:
  -collector.dhcp
    	Enable the DHCP lease collector (default true)
  -collector.dns
    	Enable the unbound DNS collector (default true)
  -collector.gateway
    	Enable the OPNsense gateway collector (default true)
  -collector.geoip
    	Enable the Geo IP location for Public IPs (default true)
  -collector.interface
    	Enable the interface traffic collector (default true)
  -collector.mbuf
    	Enable the netstat mbuf collector (default true)
  -collector.pf
    	Enable the PF firewall collector (default true)
  -collector.service
    	Enable the FreeBSD service status collector (default true)
  -collector.service.services string
    	Comma-separated list of services to monitor (default "unbound,wireguard,dpinger,suricata,kea-dhcp4")
  -collector.system
    	Enable the sysctl system collector (default true)
  -collector.wireguard
      Enable the WireGuard peer collector (default true)
  -scrape.timeout duration
    	Timeout for scraping individual system commands (default 5s)
  -web.listen-address string
    	Address on which to expose metrics and web interface (default ":9101")
  -web.telemetry-path string
    	Path under which to expose metrics (default "/metrics")
```

---

## Prometheus Configuration

Add the scrape target to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'opnsense'
    scrape_interval: 15s
    scrape_timeout: 10s
    static_configs:
      - targets: ['192.168.1.1:9101']
```

---

## Grafana Panel Setup Examples

### 1. Renaming Network Interfaces & Filtering Virtual Interfaces

To strip unneeded FreeBSD virtual interfaces (`enc0*`, `pfsync0*`) and clean up display labels:

```promql
label_replace(
  label_replace(
    label_replace(
      rate(opnsense_interface_rx_errors{interface=~"igc0|igc1|wg0"}[$__rate_interval])
      +
      rate(opnsense_interface_tx_errors{interface=~"igc0|igc1|wg0"}[$__rate_interval]),
      "interface", "LAN", "interface", "igc0"
    ),
    "interface", "WAN", "interface", "igc1"
  ),
  "interface", "VPN", "interface", "wg0"
)
```

### 2. Geomap Visualization

1. Select **Geomap** as the panel type.
2. Enter query: `opnsense_active_connections_geo`.
3. In **Query Options**, set **Format** = `Table` and **Type** = `Instant`.
4. Under **Transformations**, add **Labels to fields**.
5. Add a second transformation: **Convert field type** and set `latitude` and `longitude` fields to **Number**.
6. In the panel options sidebar under **Map Layer**, set **Location Mode** to **Coords**, **Latitude Field** to `latitude`, and **Longitude Field** to `longitude`.

---

## License

This project is licensed under the [MIT License](LICENSE).
