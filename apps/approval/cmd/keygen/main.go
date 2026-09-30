// keygen generates an Ed25519 keypair for the AEGIS→VULTURE approval
// boundary. Run: go run ./cmd/keygen
//
//   - AEGIS_APPROVAL_PRIVKEY (base64 seed) → risk-service env
//   - AEGIS_APPROVAL_PUBKEY  (base64)      → execution-service env
//
// Treat the private key like a password: never commit it, never log it.
package main

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/omega-prime-delta/approval"
)

func main() {
	pub, priv, err := approval.GenerateKeypair()
	if err != nil {
		fmt.Fprintln(os.Stderr, "keygen:", err)
		os.Exit(1)
	}
	seed := priv.Seed()
	fmt.Println("AEGIS_APPROVAL_PRIVKEY=" + base64.StdEncoding.EncodeToString(seed))
	fmt.Println("AEGIS_APPROVAL_PUBKEY=" + base64.StdEncoding.EncodeToString(pub))
}
