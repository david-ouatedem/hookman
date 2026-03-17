package delivery

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Sign computes the HMAC-SHA256 signature for a payload using the given secret.
// Returns a string in the format "sha256=<hex>".
func Sign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
}

// Verify checks that the provided signature matches the expected HMAC-SHA256
// of the payload. Uses constant-time comparison to prevent timing attacks.
func Verify(payload []byte, signature string, secret string) bool {
	expected := Sign(payload, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}
