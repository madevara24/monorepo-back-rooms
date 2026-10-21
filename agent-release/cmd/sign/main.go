// Command sign builds a release manifest for an already-built mbr-node
// binary: it checksums the binary, signs the version/commit/checksum
// with the ed25519 private key in MBR_NODE_RELEASE_SIGNING_KEY, and
// writes the resulting manifest as JSON. It never generates a key
// itself; see ../genkey for that one-time step.
package main

import (
	"crypto/ed25519"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	release "github.com/madevara24/monorepo-back-rooms/agent-release"
)

const signingKeyEnv = "MBR_NODE_RELEASE_SIGNING_KEY"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	binaryPath := flag.String("binary", "", "path to the built mbr-node binary")
	version := flag.Int("version", 0, "monotonic release version (e.g. the CI run number)")
	commitSHA := flag.String("commit", "", "git commit SHA the binary was built from")
	outPath := flag.String("out", "", "path to write manifest.json (defaults to stdout)")
	flag.Parse()

	if *binaryPath == "" || *version <= 0 || *commitSHA == "" {
		return fmt.Errorf("usage: sign -binary <path> -version <n> -commit <sha> [-out manifest.json]")
	}

	seedB64 := os.Getenv(signingKeyEnv)
	if seedB64 == "" {
		return fmt.Errorf(
			"%s is not set: this is the CI-held ed25519 private key for signed mbr-node releases; "+
				"generate one with `go run ./agent-release/cmd/genkey` and add it as a GitHub Actions repo secret named %s",
			signingKeyEnv, signingKeyEnv,
		)
	}
	priv, err := release.ParsePrivateKeyB64(seedB64)
	if err != nil {
		return fmt.Errorf("%s: %w", signingKeyEnv, err)
	}

	f, err := os.Open(*binaryPath)
	if err != nil {
		return fmt.Errorf("open binary: %w", err)
	}
	defer func() { _ = f.Close() }()

	checksum, err := release.ChecksumSHA256(f)
	if err != nil {
		return err
	}

	manifest := release.Manifest{
		Version:   *version,
		CommitSHA: *commitSHA,
		Checksum:  checksum,
		Signature: release.Sign(*version, *commitSHA, checksum, ed25519.PrivateKey(priv)),
	}

	out, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}

	if *outPath == "" {
		_, err = os.Stdout.Write(append(out, '\n'))
		return err
	}
	return os.WriteFile(*outPath, append(out, '\n'), 0o644)
}
