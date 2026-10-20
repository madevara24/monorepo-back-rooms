// Command node runs the MBR data-plane agent (mbr-node). It is the only
// process on a data-plane host that is meant to talk to Docker; the
// control-plane API calls it over mTLS.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/madevara24/monorepo-back-rooms/node-agent-service/config"
	container "github.com/madevara24/monorepo-back-rooms/node-agent-service/internal"
)

// commit is stamped at build time with -ldflags "-X main.commit=<sha>".
var commit = "unknown"

const heartbeatInterval = 30 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	c, err := container.New(cfg, commit)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Node.RunLoop(ctx, cfg.ControlURL, cfg.ControlToken, heartbeatInterval)

	tlsConfig, err := serverTLSConfig(cfg.TLSCert, cfg.TLSKey, cfg.ClientCA)
	if err != nil {
		log.Fatalf("configure mTLS: %v", err)
	}

	server := &http.Server{
		Addr:      cfg.Addr,
		Handler:   c.Mux,
		TLSConfig: tlsConfig,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		cancel()
		_ = server.Close()
	}()

	log.Printf("node %s listening on %s", cfg.NodeID, cfg.Addr)
	if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// serverTLSConfig builds an mTLS config that requires every client to
// present a certificate signed by clientCAPath.
func serverTLSConfig(certPath, keyPath, clientCAPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	caPEM, err := os.ReadFile(clientCAPath)
	if err != nil {
		return nil, err
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, os.ErrInvalid
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}, nil
}
