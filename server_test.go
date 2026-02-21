package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/miekg/dns"
)

func TestDNSServer(t *testing.T) {
	serverIP := net.ParseIP("192.168.1.1")

	// Find a free port
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := pc.LocalAddr().String()
	pc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- runDNSServer(ctx, serverIP, addr)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Send a DNS query
	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)

	r, _, err := c.Exchange(m, addr)
	if err != nil {
		t.Fatalf("DNS query failed: %v", err)
	}

	if len(r.Answer) == 0 {
		t.Fatal("Expected at least one answer")
	}

	aRecord, ok := r.Answer[0].(*dns.A)
	if !ok {
		t.Fatal("Expected A record")
	}

	if !aRecord.A.Equal(serverIP) {
		t.Errorf("Expected IP %s, got %s", serverIP, aRecord.A)
	}

	// Test with a different query type (AAAA)
	m2 := new(dns.Msg)
	m2.SetQuestion("anything.test.", dns.TypeAAAA)

	r2, _, err := c.Exchange(m2, addr)
	if err != nil {
		t.Fatalf("DNS query failed: %v", err)
	}

	if len(r2.Answer) == 0 {
		t.Fatal("Expected at least one answer for AAAA query")
	}

	aRecord2, ok := r2.Answer[0].(*dns.A)
	if !ok {
		t.Fatal("Expected A record even for AAAA query")
	}

	if !aRecord2.A.Equal(serverIP) {
		t.Errorf("Expected IP %s, got %s", serverIP, aRecord2.A)
	}

	cancel()
}

func TestHTTPServer(t *testing.T) {
	// Find a free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- runHTTPServer(ctx, addr)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test GET request
	resp, err := http.Get(fmt.Sprintf("http://%s/some/path", addr))
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/xml" {
		t.Errorf("Expected Content-Type text/xml, got %s", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	if string(body) != flatProfileXML {
		t.Errorf("Body mismatch.\nExpected:\n%s\nGot:\n%s", flatProfileXML, string(body))
	}

	// Test POST request to a different path
	resp2, err := http.Post(fmt.Sprintf("http://%s/other/path", addr), "text/plain", nil)
	if err != nil {
		t.Fatalf("HTTP POST request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp2.StatusCode)
	}

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	if string(body2) != flatProfileXML {
		t.Errorf("POST body mismatch.\nExpected:\n%s\nGot:\n%s", flatProfileXML, string(body2))
	}

	cancel()
}

func TestDHCPHandler(t *testing.T) {
	serverIP := net.ParseIP("192.168.1.1")
	_, subnet, _ := net.ParseCIDR("192.168.1.0/24")
	handler := NewDHCPHandler(serverIP, subnet)

	clientMAC, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")

	// Create a DISCOVER packet
	discover, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeDiscover),
		dhcpv4.WithHwAddr(clientMAC),
	)
	if err != nil {
		t.Fatalf("Failed to create DISCOVER: %v", err)
	}

	// Handle DISCOVER
	offer, err := handler.handleDiscover(discover)
	if err != nil {
		t.Fatalf("handleDiscover failed: %v", err)
	}

	if offer.MessageType() != dhcpv4.MessageTypeOffer {
		t.Errorf("Expected OFFER, got %s", offer.MessageType())
	}

	offeredIP := offer.YourIPAddr
	if offeredIP[0] != 192 || offeredIP[1] != 168 || offeredIP[2] != 1 || offeredIP[3] != poolStart {
		t.Errorf("Expected offered IP 192.168.1.%d, got %s", poolStart, offeredIP)
	}

	// Check options
	subnetMask := offer.SubnetMask()
	if subnetMask == nil {
		t.Error("Expected subnet mask option")
	} else if !net.IP(subnetMask).Equal(net.IP(subnet.Mask)) {
		t.Errorf("Expected subnet mask %s, got %s", net.IP(subnet.Mask), net.IP(subnetMask))
	}

	routers := offer.Router()
	if len(routers) == 0 {
		t.Error("Expected router option")
	} else if !routers[0].Equal(serverIP) {
		t.Errorf("Expected router %s, got %s", serverIP, routers[0])
	}

	dnsServers := offer.DNS()
	if len(dnsServers) == 0 {
		t.Error("Expected DNS option")
	} else if !dnsServers[0].Equal(serverIP) {
		t.Errorf("Expected DNS %s, got %s", serverIP, dnsServers[0])
	}

	// Create a REQUEST packet
	request, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeRequest),
		dhcpv4.WithHwAddr(clientMAC),
	)
	if err != nil {
		t.Fatalf("Failed to create REQUEST: %v", err)
	}

	// Handle REQUEST
	ack, err := handler.handleRequest(request)
	if err != nil {
		t.Fatalf("handleRequest failed: %v", err)
	}

	if ack.MessageType() != dhcpv4.MessageTypeAck {
		t.Errorf("Expected ACK, got %s", ack.MessageType())
	}

	// Same client should get the same IP
	if !ack.YourIPAddr.Equal(offeredIP) {
		t.Errorf("Expected same IP %s, got %s", offeredIP, ack.YourIPAddr)
	}

	// Different client should get a different IP
	clientMAC2, _ := net.ParseMAC("11:22:33:44:55:66")
	discover2, _ := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeDiscover),
		dhcpv4.WithHwAddr(clientMAC2),
	)
	offer2, err := handler.handleDiscover(discover2)
	if err != nil {
		t.Fatalf("handleDiscover for second client failed: %v", err)
	}

	if offer2.YourIPAddr.Equal(offeredIP) {
		t.Error("Second client should get a different IP")
	}

	expectedSecond := net.IP{192, 168, 1, poolStart + 1}
	if !offer2.YourIPAddr.Equal(expectedSecond) {
		t.Errorf("Expected second IP %s, got %s", expectedSecond, offer2.YourIPAddr)
	}
}
