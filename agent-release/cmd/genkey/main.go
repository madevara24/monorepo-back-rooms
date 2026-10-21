// Command genkey generates the one ed25519 keypair the mbr-node release
// pipeline needs: CI signs with the private key, and
// control-panel-service and node-agent-service verify with the public
// key. It is a human-run, one-time (or rotation-time) operation, never
// invoked by CI itself: CI only ever signs with a key that already lives
// in its secrets.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate key: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Add the private key as the GitHub Actions repo secret MBR_NODE_RELEASE_SIGNING_KEY:")
	fmt.Println(base64.StdEncoding.EncodeToString(priv))
	fmt.Println()
	fmt.Println("Configure the public key on the control plane (MBR_RELEASE_PUBLIC_KEY_B64) and on every")
	fmt.Println("node-agent-service host (AGENT_RELEASE_PUBLIC_KEY_B64):")
	fmt.Println(base64.StdEncoding.EncodeToString(pub))
}
