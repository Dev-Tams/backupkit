package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

// EncryptAESGCM encrypts src into dst using AES-256-GCM.
// Password is hashed to a 32-byte key (MVP; we’ll improve later).
func EncryptAESGCM(dst io.Writer, src io.Reader, password string) (int64, error) {
	if password == "" {
		return 0, fmt.Errorf("encryption password is empty")
	}

	//ill ugrade to scrypt later
	key := sha256.Sum256([]byte(password))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return 0, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, fmt.Errorf("gcm: %w", err)
	}

	//unique per backup
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return 0, err
	}

	//writing nonce first for decryption
	if _, err := dst.Write(nonce); err != nil{
		return 0, err
	}
	buf := make([]byte, 32*1024)
	var total int64

	//keeps memory low for large dumps
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			ciphertext := gcm.Seal(nil, nonce, buf[:n], nil)

			_, err := dst.Write(ciphertext)
			if err != nil{
				return total, err
			}

			total += int64(n)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return total, readErr
		}
	}
	return total, nil
}
