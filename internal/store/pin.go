package store

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters tuned for a low-power shop PC: fast enough for a
// PIN login, strong enough that a stolen hash cannot be reversed cheaply.
const (
	pinTime    = 1
	pinMemory  = 64 * 1024
	pinThreads = 2
	pinKeyLen  = 32
	pinSaltLen = 16
)

func HashPIN(pin string) (string, error) {
	salt := make([]byte, pinSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pin), salt, pinTime, pinMemory, pinThreads, pinKeyLen)
	return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		pinMemory, pinTime, pinThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

func VerifyPIN(encoded, pin string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(pin), salt, t, m, p, uint32(len(want)))
	if len(got) != len(want) {
		return false
	}
	var diff byte
	for i := range got {
		diff |= got[i] ^ want[i]
	}
	return diff == 0
}
