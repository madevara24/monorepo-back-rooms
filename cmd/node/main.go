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
	"strconv"
	"syscall"
	"time"

	dockerclient "github.com/docker/docker/client"

	"github.com/madevara24/monorepo-back-rooms/internal/node"
)

// commit is stamped at build time with -ldflags "-X main.commit=<sha>".
var commit = "unknown"

const heartbeatInterval = 30 * time.Second

func main() {
	addr := getEnv("AGENT_ADDR", ":9443")
	dataDir := getEnv("AGENT_DATA_DIR", "/var/lib/mbr-node")
	nodeID := getEnv("AGENT_NODE_ID", "")
	if nodeID == "" {
		if hostname, err := os.Hostname(); err == nil {
			nodeID = hostname
		}
	}
	hostAddress := os.Getenv("AGENT_HOST_ADDRESS")
	image := getEnv("AGENT_IMAGE", "postgres:17")
	portMin := getEnvInt("AGENT_PORT_MIN", 21000)
	portMax := getEnvInt("AGENT_PORT_MAX", 21999)

	tlsCert := os.Getenv("AGENT_TLS_CERT")
	tlsKey := os.Getenv("AGENT_TLS_KEY")
	clientCA := os.Getenv("AGENT_CLIENT_CA")
	if tlsCert == "" || tlsKey == "" || clientCA == "" {
		log.Fatal("AGENT_TLS_CERT, AGENT_TLS_KEY, and AGENT_CLIENT_CA are required")
	}

	controlURL := os.Getenv("CONTROL_URL")
	controlToken := os.Getenv("AGENT_CONTROL_TOKEN")

	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("connect to docker: %v", err)
	}

	n := &node.Node{
		Config: node.Config{
			NodeID:      nodeID,
			HostAddress: hostAddress,
			Image:       image,
			PortMin:     portMin,
			PortMax:     portMax,
			Version:     commit,
		},
		Docker:  node.NewDockerClient(cli),
		Volumes: node.NewLoopVolumeManager(dataDir),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go n.RunLoop(ctx, controlURL, controlToken, heartbeatInterval)

	tlsConfig, err := serverTLSConfig(tlsCert, tlsKey, clientCA)
	if err != nil {
		log.Fatalf("configure mTLS: %v", err)
	}

	server := &http.Server{
		Addr:      addr,
		Handler:   node.NewMux(n),
		TLSConfig: tlsConfig,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		cancel()
		_ = server.Close()
	}()

	log.Printf("node %s listening on %s", nodeID, addr)
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

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("invalid %s=%q: %v", key, v, err)
	}
	return n
}
