package passwordauth

// YuriID password hashing.
//
// Native format: PHC-style argon2id, "$argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>" (raw std base64, no padding).
// Legacy formats accepted for verification only (imported from Authentik / Django), rehashed to native on the next successful login:
//   - Django PBKDF2:          pbkdf2_sha256$<iterations>$<salt>$<b64 hash>   (also pbkdf2_sha1)
//   - Django Argon2:          argon2$argon2id$v=19$m=..,t=..,p=..$<salt>$<hash>  (also argon2i)
//   - Django bcrypt_sha256:   bcrypt_sha256$$2b$12$...
//   - Django bcrypt:          bcrypt$$2b$12$...
//   - plain bcrypt:           $2a$ / $2b$ / $2y$
// Unusable Django hashes ("!..." / empty) are rejected on import.

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // only used to verify legacy Django pbkdf2_sha1 hashes
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
)

const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 2
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16

	// MinPasswordLength is the minimum length of a new password (in characters)
	MinPasswordLength = 10
	// MaxPasswordLength guards against hashing huge inputs
	MaxPasswordLength = 256
)

var (
	ErrUnsupportedHash = errors.New("unsupported or unusable password hash format")
	b64                = base64.RawStdEncoding
)

// HashPassword returns a native argon2id hash
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword checks a password against any supported format
// needsRehash is true when the hash is valid but not in the current native format/parameters
func VerifyPassword(password, encoded string) (ok bool, needsRehash bool, err error) {
	switch {
	case strings.HasPrefix(encoded, "$argon2"):
		ok, params, err := verifyArgon2PHC(password, encoded)
		if err != nil || !ok {
			return false, false, err
		}
		native := params.variant == "argon2id" && params.m == argonMemory && params.t == argonTime && params.p == argonThreads && params.keyLen == argonKeyLen
		return true, !native, nil
	case strings.HasPrefix(encoded, "argon2$"):
		ok, _, err := verifyArgon2PHC(password, strings.TrimPrefix(encoded, "argon2"))
		return ok, ok, err
	case strings.HasPrefix(encoded, "pbkdf2_sha256$"):
		ok, err := verifyDjangoPBKDF2(password, encoded, sha256.New, 32)
		return ok, ok, err
	case strings.HasPrefix(encoded, "pbkdf2_sha1$"):
		ok, err := verifyDjangoPBKDF2(password, encoded, sha1.New, 20)
		return ok, ok, err
	case strings.HasPrefix(encoded, "bcrypt_sha256$"):
		// Django pre-hashes with SHA-256 (hex) to avoid bcrypt's 72-byte limit
		sum := sha256.Sum256([]byte(password))
		ok, err := verifyBcrypt(hex.EncodeToString(sum[:]), strings.TrimPrefix(encoded, "bcrypt_sha256$"))
		return ok, ok, err
	case strings.HasPrefix(encoded, "bcrypt$"):
		ok, err := verifyBcrypt(password, strings.TrimPrefix(encoded, "bcrypt$"))
		return ok, ok, err
	case strings.HasPrefix(encoded, "$2a$"), strings.HasPrefix(encoded, "$2b$"), strings.HasPrefix(encoded, "$2y$"):
		ok, err := verifyBcrypt(password, encoded)
		return ok, ok, err
	}
	return false, false, ErrUnsupportedHash
}

// ValidateImportableHash checks that a hash is in a supported, usable format (without verifying any password)
func ValidateImportableHash(encoded string) error {
	if encoded == "" || strings.HasPrefix(encoded, "!") {
		return ErrUnsupportedHash
	}
	switch {
	case strings.HasPrefix(encoded, "$argon2"), strings.HasPrefix(encoded, "argon2$"):
		_, err := parseArgon2PHC(strings.TrimPrefix(encoded, "argon2"))
		return err
	case strings.HasPrefix(encoded, "pbkdf2_sha256$"), strings.HasPrefix(encoded, "pbkdf2_sha1$"):
		parts := strings.Split(encoded, "$")
		if len(parts) != 4 {
			return ErrUnsupportedHash
		}
		if it, err := strconv.Atoi(parts[1]); err != nil || it < 1 {
			return ErrUnsupportedHash
		}
		if _, err := base64.StdEncoding.DecodeString(parts[3]); err != nil {
			return ErrUnsupportedHash
		}
		return nil
	case strings.HasPrefix(encoded, "bcrypt_sha256$"), strings.HasPrefix(encoded, "bcrypt$"),
		strings.HasPrefix(encoded, "$2a$"), strings.HasPrefix(encoded, "$2b$"), strings.HasPrefix(encoded, "$2y$"):
		i := strings.Index(encoded, "$2")
		if i < 0 {
			// e.g. "bcrypt$" without an embedded $2x$ hash: malformed, and must not panic
			return ErrUnsupportedHash
		}
		if _, err := bcrypt.Cost([]byte(encoded[i:])); err != nil {
			return ErrUnsupportedHash
		}
		return nil
	}
	return ErrUnsupportedHash
}

// HashAlgorithm returns a short, non-secret label for the hash format
func HashAlgorithm(encoded string) string {
	switch {
	case strings.HasPrefix(encoded, "$argon2id$"):
		return "argon2id"
	case strings.HasPrefix(encoded, "$"):
		return strings.SplitN(strings.TrimPrefix(encoded, "$"), "$", 2)[0]
	default:
		return "legacy:" + strings.SplitN(encoded, "$", 2)[0]
	}
}

type argonParams struct {
	variant   string
	m, t      uint32
	p         uint8
	keyLen    uint32
	salt, key []byte
}

func parseArgon2PHC(encoded string) (argonParams, error) {
	// $argon2id$v=19$m=65536,t=3,p=2$salt$hash
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" {
		return argonParams{}, ErrUnsupportedHash
	}
	var p argonParams
	p.variant = parts[1]
	if p.variant != "argon2id" && p.variant != "argon2i" {
		return argonParams{}, ErrUnsupportedHash
	}
	if parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return argonParams{}, ErrUnsupportedHash
	}
	var pp uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.m, &p.t, &pp); err != nil || pp == 0 || pp > 255 || p.t == 0 || p.m == 0 {
		return argonParams{}, ErrUnsupportedHash
	}
	p.p = uint8(pp)
	var err error
	if p.salt, err = decodeB64Loose(parts[4]); err != nil {
		return argonParams{}, ErrUnsupportedHash
	}
	if p.key, err = decodeB64Loose(parts[5]); err != nil || len(p.key) < 16 || len(p.key) > 1024 {
		return argonParams{}, ErrUnsupportedHash
	}
	p.keyLen = uint32(len(p.key)) //nolint:gosec // bounded to 16..1024 above
	return p, nil
}

func verifyArgon2PHC(password, encoded string) (bool, argonParams, error) {
	p, err := parseArgon2PHC(encoded)
	if err != nil {
		return false, p, err
	}
	var key []byte
	if p.variant == "argon2id" {
		key = argon2.IDKey([]byte(password), p.salt, p.t, p.m, p.p, p.keyLen)
	} else {
		key = argon2.Key([]byte(password), p.salt, p.t, p.m, p.p, p.keyLen)
	}
	return subtle.ConstantTimeCompare(key, p.key) == 1, p, nil
}

func verifyDjangoPBKDF2(password, encoded string, h func() hash.Hash, keyLen int) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 {
		return false, ErrUnsupportedHash
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 1 || iterations > 10_000_000 {
		return false, ErrUnsupportedHash
	}
	expected, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, ErrUnsupportedHash
	}
	if len(expected) != keyLen {
		keyLen = len(expected)
	}
	got := pbkdf2.Key([]byte(password), []byte(parts[2]), iterations, keyLen, h)
	return hmac.Equal(got, expected), nil
}

func verifyBcrypt(password, encoded string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(encoded), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	if err != nil {
		return false, ErrUnsupportedHash
	}
	return true, nil
}

func decodeB64Loose(s string) ([]byte, error) {
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// dummyHash is verified when an account doesn't exist, so failed logins take similar time either way
var dummyHash = func() string {
	h, _ := HashPassword("yuriid-timing-equalizer-not-a-real-password")
	return h
}()

// burnTime performs a comparable amount of hashing work without revealing anything
func burnTime(password string) {
	_, _, _ = VerifyPassword(password, dummyHash)
}
