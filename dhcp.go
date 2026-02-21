package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
)

const (
	poolStart   = 100
	poolEnd     = 200
	leaseTime   = 1 * time.Hour
)

type DHCPHandler struct {
	serverIP  net.IP
	subnet    *net.IPNet
	mu        sync.Mutex
	nextIP    byte
	leases    map[string]net.IP // MAC -> IP
}

func NewDHCPHandler(serverIP net.IP, subnet *net.IPNet) *DHCPHandler {
	return &DHCPHandler{
		serverIP: serverIP,
		subnet:   subnet,
		nextIP:   poolStart,
		leases:   make(map[string]net.IP),
	}
}

func (h *DHCPHandler) allocateIP(mac net.HardwareAddr) net.IP {
	h.mu.Lock()
	defer h.mu.Unlock()

	macStr := mac.String()
	if ip, ok := h.leases[macStr]; ok {
		return ip
	}

	if h.nextIP > poolEnd {
		return nil
	}

	ip := make(net.IP, 4)
	copy(ip, h.subnet.IP.To4().Mask(h.subnet.Mask))
	ip[3] = h.nextIP
	h.nextIP++
	h.leases[macStr] = ip
	return ip
}

func (h *DHCPHandler) Handle(conn net.PacketConn, peer net.Addr, req *dhcpv4.DHCPv4) {
	if req.OpCode != dhcpv4.OpcodeBootRequest {
		return
	}

	msgType := req.MessageType()
	log.Printf("[DHCP] Received %s from %s", msgType, req.ClientHWAddr)

	var resp *dhcpv4.DHCPv4
	var err error

	switch msgType {
	case dhcpv4.MessageTypeDiscover:
		resp, err = h.handleDiscover(req)
	case dhcpv4.MessageTypeRequest:
		resp, err = h.handleRequest(req)
	default:
		log.Printf("[DHCP] Ignoring message type %s", msgType)
		return
	}

	if err != nil {
		log.Printf("[DHCP] Error handling %s: %v", msgType, err)
		return
	}

	if resp == nil {
		return
	}

	// Broadcast response
	peer = &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
	if _, err := conn.WriteTo(resp.ToBytes(), peer); err != nil {
		log.Printf("[DHCP] Failed to send response: %v", err)
		return
	}

	log.Printf("[DHCP] Sent %s to %s -> %s",
		resp.MessageType(), req.ClientHWAddr, resp.YourIPAddr)
}

func (h *DHCPHandler) handleDiscover(req *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, error) {
	offerIP := h.allocateIP(req.ClientHWAddr)
	if offerIP == nil {
		return nil, fmt.Errorf("no IPs available in pool")
	}

	resp, err := dhcpv4.NewReplyFromRequest(req,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithServerIP(h.serverIP),
		dhcpv4.WithYourIP(offerIP),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(h.subnet.Mask)),
		dhcpv4.WithOption(dhcpv4.OptRouter(h.serverIP)),
		dhcpv4.WithOption(dhcpv4.OptDNS(h.serverIP)),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(leaseTime)),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(h.serverIP)),
	)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (h *DHCPHandler) handleRequest(req *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, error) {
	offerIP := h.allocateIP(req.ClientHWAddr)
	if offerIP == nil {
		return nil, fmt.Errorf("no IPs available in pool")
	}

	resp, err := dhcpv4.NewReplyFromRequest(req,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithServerIP(h.serverIP),
		dhcpv4.WithYourIP(offerIP),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(h.subnet.Mask)),
		dhcpv4.WithOption(dhcpv4.OptRouter(h.serverIP)),
		dhcpv4.WithOption(dhcpv4.OptDNS(h.serverIP)),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(leaseTime)),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(h.serverIP)),
	)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func runDHCPServer(ctx context.Context, iface *net.Interface, serverIP net.IP, subnet *net.IPNet) error {
	handler := NewDHCPHandler(serverIP, subnet)

	addr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: 67,
	}

	srv, err := server4.NewServer(iface.Name, addr, handler.Handle)
	if err != nil {
		return fmt.Errorf("failed to create DHCP server: %w", err)
	}

	log.Printf("[DHCP] Server starting on %s :67", iface.Name)

	go func() {
		<-ctx.Done()
		srv.Close()
	}()

	if err := srv.Serve(); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}
