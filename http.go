package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
)

const flatProfileXML = `<flat-profile>
    <Admin_Passwd ua="na"></Admin_Passwd>
    <User_Passwd ua="na"></User_Passwd>
    <Provision_Enable ua="na">No</Provision_Enable>
    <Upgrade_Enable ua="na">No</Upgrade_Enable>
</flat-profile>
`

func newHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[HTTP] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, flatProfileXML)
	})
}

func runHTTPServer(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: newHTTPHandler(),
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("HTTP listen error: %w", err)
	}

	log.Printf("[HTTP] Server starting on %s", addr)

	go func() {
		<-ctx.Done()
		srv.Close()
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
