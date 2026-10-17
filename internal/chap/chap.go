// SPDX-License-Identifier: GPL-3.0-or-later
package chap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"crypto/pbkdf2"
)

// Response derives the hexadecimal CHAP response expected by RunWisp.
func Response(password, nonce string) string {
	key, _ := pbkdf2.Key(sha256.New, password, []byte(nonce), 600_000, 32)
	return hex.EncodeToString(key)
}

func DecodeNonce(nonce string) ([]byte, error) {
	value, err := hex.DecodeString(nonce)
	if err != nil {
		return nil, fmt.Errorf("decode challenge nonce: %w", err)
	}
	return value, nil
}
