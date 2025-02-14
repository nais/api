package serviceaccount

import (
	"crypto/rand"
	"crypto/sha256"

	"github.com/btcsuite/btcutil/base58"
)

const tokenPrefix = "nais_console_"

func HashToken(token string) (string, error) {
	sha256Hash := sha256.New()
	if _, err := sha256Hash.Write([]byte(token)); err != nil {
		return "", err
	}

	return base58.Encode(sha256Hash.Sum(nil)), nil
}

func generateToken() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return tokenPrefix + base58.Encode(b), nil
}
