package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/miekg/dns"
)

func newDNSHandler(serverIP net.IP) dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		msg := new(dns.Msg)
		msg.SetReply(r)
		msg.Authoritative = true

		for _, q := range r.Question {
			log.Printf("[DNS] Query: %s %s from %s", dns.TypeToString[q.Qtype], q.Name, w.RemoteAddr())

			rr := &dns.A{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				A: serverIP,
			}
			msg.Answer = append(msg.Answer, rr)
		}

		if err := w.WriteMsg(msg); err != nil {
			log.Printf("[DNS] Failed to write response: %v", err)
		}
	}
}

func runDNSServer(ctx context.Context, serverIP net.IP, addr string) error {
	handler := newDNSHandler(serverIP)

	srv := &dns.Server{
		Addr:    addr,
		Net:     "udp",
		Handler: handler,
	}

	log.Printf("[DNS] Server starting on %s", addr)

	go func() {
		<-ctx.Done()
		srv.Shutdown()
	}()

	if err := srv.ListenAndServe(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("DNS server error: %w", err)
	}
	return nil
}
