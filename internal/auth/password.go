// Package auth holds the two fixed accounts, password verification, the
// server-side sessions and the login rate limit.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2idParams are the cost settings of one hash. The floor is the OWASP
// minimum for argon2id: 19 MiB of memory, two passes, one lane.
type Argon2idParams struct {
	MemoryKiB uint32
	Passes    uint32
	Lanes     uint8
}

// MinimumArgon2idParams is the weakest setting a configured hash may use.
var MinimumArgon2idParams = Argon2idParams{MemoryKiB: 19 * 1024, Passes: 2, Lanes: 1}

const (
	argon2idName    = "argon2id"
	minSaltBytes    = 8
	minKeyBytes     = 16
	hashedSaltBytes = 16
	hashedKeyBytes  = 32
)

// PasswordHash is a parsed PHC string:
// $argon2id$v=19$m=<KiB>,t=<passes>,p=<lanes>$<salt>$<key>, base64 unpadded.
type PasswordHash struct {
	Params Argon2idParams
	salt   []byte
	key    []byte
}

// ParsePasswordHash reads a PHC string and refuses anything that is not
// argon2id version 19 at or above MinimumArgon2idParams.
func ParsePasswordHash(encoded string) (PasswordHash, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != argon2idName {
		return PasswordHash{}, errors.New("not an argon2id PHC string")
	}
	if parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return PasswordHash{}, fmt.Errorf("argon2id version must be v=%d", argon2.Version)
	}
	var p Argon2idParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Passes, &p.Lanes); err != nil {
		return PasswordHash{}, fmt.Errorf("argon2id parameters: %w", err)
	}
	// Sscanf stops at the first character it cannot use; the round trip
	// rejects trailing text it would otherwise ignore.
	if parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", p.MemoryKiB, p.Passes, p.Lanes) {
		return PasswordHash{}, errors.New("argon2id parameters must be exactly m=<KiB>,t=<passes>,p=<lanes>")
	}
	if p.MemoryKiB < MinimumArgon2idParams.MemoryKiB || p.Passes < MinimumArgon2idParams.Passes || p.Lanes < MinimumArgon2idParams.Lanes {
		return PasswordHash{}, fmt.Errorf("argon2id parameters m=%d,t=%d,p=%d are below the minimum m=%d,t=%d,p=%d",
			p.MemoryKiB, p.Passes, p.Lanes, MinimumArgon2idParams.MemoryKiB, MinimumArgon2idParams.Passes, MinimumArgon2idParams.Lanes)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < minSaltBytes {
		return PasswordHash{}, fmt.Errorf("argon2id salt must be at least %d bytes of unpadded base64", minSaltBytes)
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) < minKeyBytes {
		return PasswordHash{}, fmt.Errorf("argon2id key must be at least %d bytes of unpadded base64", minKeyBytes)
	}
	return PasswordHash{Params: p, salt: salt, key: key}, nil
}

// HashPassword derives a new hash with a random salt.
func HashPassword(password string, p Argon2idParams) (string, error) {
	salt := make([]byte, hashedSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, p.Passes, p.MemoryKiB, p.Lanes, hashedKeyBytes)
	return fmt.Sprintf("$%s$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2idName, argon2.Version, p.MemoryKiB, p.Passes, p.Lanes,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// verifyArgon2idPassword recomputes the key and compares it in constant time,
// so how long the comparison takes says nothing about how much of it matched.
func verifyArgon2idPassword(h PasswordHash, password string) bool {
	derived := argon2.IDKey([]byte(password), h.salt, h.Params.Passes, h.Params.MemoryKiB, h.Params.Lanes, uint32(len(h.key)))
	return subtle.ConstantTimeCompare(derived, h.key) == 1
}

// newDummyPasswordHash is what an unknown identifier is verified against: a
// hash of a random password nobody knows, at the same cost as a real account,
// so a miss takes as long as a wrong password.
func newDummyPasswordHash(p Argon2idParams) (PasswordHash, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return PasswordHash{}, err
	}
	encoded, err := HashPassword(base64.RawStdEncoding.EncodeToString(secret), p)
	if err != nil {
		return PasswordHash{}, err
	}
	return ParsePasswordHash(encoded)
}
