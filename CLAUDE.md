# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

A Go tool for unlocking Cisco/Linksys PAP2T VoIP adapters. It runs a fake provisioning environment (DHCP + DNS + HTTP servers) on a specified network interface. When the PAP2T connects, it receives an IP via DHCP, all DNS queries resolve to the server (captive portal), and the HTTP server serves a flat-profile XML that clears passwords and disables provisioning/upgrades.

## Build & Test Commands

```bash
go build -o pap2t-unlock .                                    # build
GOOS=linux GOARCH=arm64 go build -o pap2t-unlock-arm64 .      # cross-compile for arm64
go test ./...                                                  # run all tests
go test -run TestDHCPHandler ./...                             # run a single test
```

Requires root/sudo to run (binds to ports 67, 53, 80). Usage: `sudo ./pap2t-unlock <interface>`

## Architecture

Single-package (`main`) app with four source files:

- **main.go** — CLI entry point. Takes a network interface name as argument, auto-assigns `10.0.0.1/24` if no IPv4 exists, launches all three servers concurrently with `sync.WaitGroup`, handles graceful shutdown via `SIGINT`/`SIGTERM`.
- **dhcp.go** — DHCP server on port 67 using `insomniacslk/dhcp`. `DHCPHandler` manages a simple IP pool (last octet 100–200) with MAC-to-IP lease tracking. Responds to DISCOVER with OFFER and REQUEST with ACK, setting the server as router and DNS.
- **dns.go** — DNS server on port 53 using `miekg/dns`. Wildcard handler that responds to every query with an A record pointing to the server IP (regardless of query type or name).
- **http.go** — HTTP server on port 80. Every request (any method, any path) returns the `flatProfileXML` constant — a Cisco flat-profile that clears `Admin_Passwd`, `User_Passwd`, disables `Provision_Enable` and `Upgrade_Enable`.

All three servers accept a `context.Context` for coordinated shutdown.
