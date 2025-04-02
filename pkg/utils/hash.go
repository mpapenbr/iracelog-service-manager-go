package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// We don't want to store apiKeys as plain text in the database.
// Since we use the output of this function to identify the tenant in the database
// we cannot use salts here.
// In general just hashing is not enough, but since the apiKeys are
// generated random strings this seems to be reasonable solution.
func HashAPIKey(arg string) string {
	hasher := sha256.New()
	hasher.Write([]byte(arg))
	return hex.EncodeToString(hasher.Sum(nil))
}
