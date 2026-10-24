// Package integration holds the control plane's outbound clients for
// external processes: the mTLS HTTP client for mbr-node (the data-plane
// agent) and the SNI proxy's host-map writer.
package integration

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Database is the agent's view of a tenant container, returned by
// POST /v1/databases. Its shape matches node-agent-service's
// databaseResponse.
type Database struct {
	ID     string `json:"id"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Status string `json:"status"`
}

// Client calls a single mbr-node agent over HTTP(S).
type Client struct {
	httpClient *http.Client
}

// New builds a Client that authenticates to the agent with mTLS, using the
// decoded PEM-encoded CA that signs the agent's server certificate and the
// client certificate and key the same CA issued to the control plane.
func New(caPEM, certPEM, keyPEM []byte) (*Client, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("load agent client certificate: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("parse agent CA certificate")
	}

	return NewWithHTTPClient(&http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				RootCAs:      pool,
			},
		},
	}), nil
}

// NewWithHTTPClient builds a Client around an already-configured HTTP
// client. Production code should use New; tests use this to point the
// same request and response handling at a plain-HTTP fake agent.
func NewWithHTTPClient(httpClient *http.Client) *Client {
	return &Client{httpClient: httpClient}
}

// ResetCounts is the agent's report after a bulk demo reset.
type ResetCounts struct {
	Containers int `json:"containers_deleted"`
	Volumes    int `json:"volumes_deleted"`
	Orphans    int `json:"orphans_deleted"`
}

// ResetDemo asks the agent to remove every tenant container, volume, and
// orphan on its host. It is a no-op on an already-clean host.
func (c *Client) ResetDemo(ctx context.Context, address string) (ResetCounts, error) {
	var out ResetCounts
	err := c.do(ctx, http.MethodPost, address+"/v1/demo/reset", nil, &out)
	return out, err
}

// ListDatabases reads the agent's actual tenant container list.
func (c *Client) ListDatabases(ctx context.Context, address string) ([]Database, error) {
	var out []Database
	err := c.do(ctx, http.MethodGet, address+"/v1/databases", nil, &out)
	return out, err
}

// CreateDatabase calls the agent's POST /v1/databases, asking it to
// provision (or, if it already exists, confirm) a tenant container.
// address is the node's address as stored in nodes.address.
func (c *Client) CreateDatabase(ctx context.Context, address, id string, sizeGB int, password string) (Database, error) {
	body, err := json.Marshal(struct {
		ID       string `json:"id"`
		SizeGB   int    `json:"size_gb"`
		Password string `json:"password"`
	}{id, sizeGB, password})
	if err != nil {
		return Database{}, err
	}

	var out Database
	err = c.do(ctx, http.MethodPost, address+"/v1/databases", bytes.NewReader(body), &out)
	return out, err
}

// DeleteDatabase calls the agent's DELETE /v1/databases/{id}, removing the
// tenant container and its volume.
func (c *Client) DeleteDatabase(ctx context.Context, address, id string) error {
	return c.do(ctx, http.MethodDelete, address+"/v1/databases/"+id, nil, nil)
}

func (c *Client) do(ctx context.Context, method, url string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("agent request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent returned %d: %s", resp.StatusCode, msg)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
