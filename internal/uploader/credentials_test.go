package uploader

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"testing"

	"worker-download/internal/db/models"
)

func TestPrepareStorageCredentialsMatchesPlatformFormat(t *testing.T) {
	const secret = "storage-test-secret"
	storage := &models.Storage{
		Provider: "s3",
		S3: &models.StorageS3Config{
			CredentialsConfigured:    true,
			AccessKeyIDEncrypted:     encryptTestCredential(t, "access-key", secret),
			SecretAccessKeyEncrypted: encryptTestCredential(t, "secret-key", secret),
		},
	}
	if err := PrepareStorageCredentials(storage, secret); err != nil {
		t.Fatal(err)
	}
	if storage.S3.AccessKeyID != "access-key" || storage.S3.SecretAccessKey != "secret-key" {
		t.Fatalf("decrypted credentials do not match")
	}
}

func encryptTestCredential(t *testing.T, value, secret string) string {
	t.Helper()
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		t.Fatal(err)
	}
	sealed := gcm.Seal(nil, iv, []byte(value), nil)
	tagStart := len(sealed) - gcm.Overhead()
	encode := base64.RawURLEncoding.EncodeToString
	return fmt.Sprintf("v1.%s.%s.%s", encode(iv), encode(sealed[tagStart:]), encode(sealed[:tagStart]))
}
