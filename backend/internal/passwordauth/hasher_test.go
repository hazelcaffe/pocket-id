package passwordauth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
)

func TestNativeRoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	ok, rehash, err := VerifyPassword("correct horse battery staple", h)
	if err != nil || !ok || rehash {
		t.Fatalf("native verify: ok=%v rehash=%v err=%v", ok, rehash, err)
	}
	if ok, _, _ := VerifyPassword("wrong", h); ok {
		t.Fatal("wrong password accepted")
	}
	if HashAlgorithm(h) != "argon2id" {
		t.Fatal(HashAlgorithm(h))
	}
}

// Django-format pbkdf2_sha256 hash (same construction Django uses: base64(PBKDF2-HMAC-SHA256(password, utf8 salt)))
func TestDjangoPBKDF2(t *testing.T) {
	salt := "yuriidsalt123456"
	key := pbkdf2.Key([]byte("hunter2hunter2"), []byte(salt), 1000, 32, sha256.New)
	enc := fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", 1000, salt, base64.StdEncoding.EncodeToString(key))
	if err := ValidateImportableHash(enc); err != nil {
		t.Fatal(err)
	}
	ok, rehash, err := VerifyPassword("hunter2hunter2", enc)
	if err != nil || !ok || !rehash {
		t.Fatalf("pbkdf2: ok=%v rehash=%v err=%v", ok, rehash, err)
	}
	if ok, _, _ := VerifyPassword("hunter2hunter3", enc); ok {
		t.Fatal("wrong password accepted")
	}
}

func TestDjangoArgon2(t *testing.T) {
	salt := []byte("0123456789abcdef")
	key := argon2.IDKey([]byte("s3cret-pass"), salt, 2, 102400, 8, 32)
	enc := fmt.Sprintf("argon2$argon2id$v=19$m=102400,t=2,p=8$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
	if err := ValidateImportableHash(enc); err != nil {
		t.Fatal(err)
	}
	ok, rehash, err := VerifyPassword("s3cret-pass", enc)
	if err != nil || !ok || !rehash {
		t.Fatalf("argon2: ok=%v rehash=%v err=%v", ok, rehash, err)
	}
}

func TestDjangoBcrypt(t *testing.T) {
	sum := sha256.Sum256([]byte("bcrypt-pass-123"))
	b, _ := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(sum[:])), 4)
	enc := "bcrypt_sha256$" + string(b)
	if err := ValidateImportableHash(enc); err != nil {
		t.Fatal(err)
	}
	if ok, rehash, err := VerifyPassword("bcrypt-pass-123", enc); err != nil || !ok || !rehash {
		t.Fatalf("bcrypt_sha256: ok=%v rehash=%v err=%v", ok, rehash, err)
	}
	b2, _ := bcrypt.GenerateFromPassword([]byte("plain-bcrypt-1"), 4)
	if ok, _, err := VerifyPassword("plain-bcrypt-1", "bcrypt$"+string(b2)); err != nil || !ok {
		t.Fatalf("bcrypt: ok=%v err=%v", ok, err)
	}
}

func TestUnusable(t *testing.T) {
	for _, h := range []string{"", "!abc", "md5$x$y", "pbkdf2_sha256$x$y$z"} {
		if ValidateImportableHash(h) == nil {
			t.Fatalf("accepted %q", h)
		}
	}
}

func TestMalformedBcryptPrefixDoesNotPanic(t *testing.T) {
	for _, h := range []string{"bcrypt$", "bcrypt_sha256$nothing-here"} {
		if err := ValidateImportableHash(h); err == nil {
			t.Fatalf("%q must be rejected", h)
		}
		if ok, _, _ := VerifyPassword("whatever-password", h); ok {
			t.Fatalf("%q must not verify", h)
		}
	}
}
