package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <interface>\n", os.Args[0])
		os.Exit(1)
	}

	ifaceName := os.Args[1]

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		log.Fatalf("Failed to find interface %s: %v", ifaceName, err)
	}

	ifaceIP, ifaceNet, err := getInterfaceIPv4(iface)
	if err != nil {
		log.Printf("No IPv4 address on %s, assigning 10.0.0.1/24...", ifaceName)
		if err := assignIP(ifaceName, "10.0.0.1/24"); err != nil {
			log.Fatalf("Failed to assign IP to %s: %v", ifaceName, err)
		}
		// Re-read the interface to pick up the new address
		iface, err = net.InterfaceByName(ifaceName)
		if err != nil {
			log.Fatalf("Failed to find interface %s after IP assignment: %v", ifaceName, err)
		}
		ifaceIP, ifaceNet, err = getInterfaceIPv4(iface)
		if err != nil {
			log.Fatalf("Failed to get IPv4 address for %s after assignment: %v", ifaceName, err)
		}
	}

	log.Printf("Using interface %s: IP=%s Subnet=%s", ifaceName, ifaceIP, ifaceNet)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		if err := runDHCPServer(ctx, iface, ifaceIP, ifaceNet); err != nil {
			log.Printf("DHCP server error: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := runDNSServer(ctx, ifaceIP, ":53"); err != nil {
			log.Printf("DNS server error: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := runHTTPServer(ctx, ":80"); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	sig := <-sigCh
	log.Printf("Received signal %v, shutting down...", sig)
	cancel()
	wg.Wait()
	log.Println("Shutdown complete")
}

func assignIP(ifaceName, cidr string) error {
	// Bring the interface up
	if out, err := exec.Command("ip", "link", "set", ifaceName, "up").CombinedOutput(); err != nil {
		return fmt.Errorf("ip link set up: %w: %s", err, out)
	}
	// Assign the IP address (flush first to avoid duplicates)
	if out, err := exec.Command("ip", "addr", "flush", "dev", ifaceName).CombinedOutput(); err != nil {
		return fmt.Errorf("ip addr flush: %w: %s", err, out)
	}
	if out, err := exec.Command("ip", "addr", "add", cidr, "dev", ifaceName).CombinedOutput(); err != nil {
		return fmt.Errorf("ip addr add: %w: %s", err, out)
	}
	log.Printf("Assigned %s to %s", cidr, ifaceName)
	return nil
}

func getInterfaceIPv4(iface *net.Interface) (net.IP, *net.IPNet, error) {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, nil, err
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			return ip4, ipNet, nil
		}
	}
	return nil, nil, fmt.Errorf("no IPv4 address found on interface %s", iface.Name)
}
