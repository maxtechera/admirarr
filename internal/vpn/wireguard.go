package vpn

import (
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/crypto/curve25519"
)

// GenerateKeyPair generates a WireGuard private/public keypair.
// Returns base64-encoded keys suitable for Mullvad device registration.
func GenerateKeyPair() (privateKey, publicKey string, err error) {
	// Generate 32 random bytes for the private key
	var privBytes [32]byte
	if _, err := rand.Read(privBytes[:]); err != nil {
		return "", "", err
	}

	// Clamp the private key per Curve25519 spec
	privBytes[0] &= 248
	privBytes[31] &= 127
	privBytes[31] |= 64

	// Derive the public key
	pubBytes, err := curve25519.X25519(privBytes[:], curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}

	return base64.StdEncoding.EncodeToString(privBytes[:]),
		base64.StdEncoding.EncodeToString(pubBytes), nil
}
