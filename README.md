# FreeBSD PF Exporter (OPNsense Prometheus Exporter)

A lightweight Prometheus exporter written in Go for OPNsense and FreeBSD systems. It exposes firewall (PF), interface, gateway, DHCP, DNS, and system metrics using native system commands without daemon threads.

## Features
* **Zero Background Threads:** Only executes commands when Prometheus scrapes the endpoint.
* **Low Memory:** Consumes <10 MB RAM.
* **Modular:** Enable or disable specific collectors via flags.
* **Native:** Written in Go 1.24+ using standard `prometheus/client_golang`.

## Build Instructions

Compile the static binary for FreeBSD natively (no CGO required):

```bash
git clone [https://github.com/yourusername/freebsd-pf-exporter.git](https://github.com/yourusername/freebsd-pf-exporter.git)
cd freebsd-pf-exporter
make clean && make
