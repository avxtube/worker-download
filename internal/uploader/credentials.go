package uploader

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"worker-download/internal/db/models"
)

func PrepareStorageCredentials(storage *models.Storage, secret string) error {
	if storage == nil || storage.Provider != "s3" {
		return nil
	}
	if storage.S3 == nil {
		return fmt.Errorf("S3 storage has no s3 config")
	}
	if !storage.S3.CredentialsConfigured {
		return fmt.Errorf("S3 credentials are not configured")
	}
	if strings.TrimSpace(secret) == "" {
		return fmt.Errorf("STORAGE_ENCRYPTION_KEY (or BETTER_AUTH_SECRET) is required for S3 storage")
	}
	accessKey, err := decryptStorageCredential(storage.S3.AccessKeyIDEncrypted, secret)
	if err != nil {
		return fmt.Errorf("decrypt access key: %w", err)
	}
	secretKey, err := decryptStorageCredential(storage.S3.SecretAccessKeyEncrypted, secret)
	if err != nil {
		return fmt.Errorf("decrypt secret key: %w", err)
	}
	storage.S3.AccessKeyID = accessKey
	storage.S3.SecretAccessKey = secretKey
	return nil
}

func decryptStorageCredential(value, secret string) (string, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 4 || parts[0] != "v1" {
		return "", fmt.Errorf("invalid credential format")
	}
	iv, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode IV: %w", err)
	}
	tag, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("decode auth tag: %w", err)
	}
	encrypted, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(iv) != gcm.NonceSize() || len(tag) != gcm.Overhead() {
		return "", fmt.Errorf("invalid IV or auth tag size")
	}
	plain, err := gcm.Open(nil, iv, append(encrypted, tag...), nil)
	if err != nil {
		return "", fmt.Errorf("authenticate credential: %w", err)
	}
	return string(plain), nil
}
