package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
)

//DecryptAESGCM just reverses EncryptAESGCM

func DecryptAESGCM(dst io.Writer, src io.Reader, password string) (int64, error) {
	if password == "" {
		return 0, fmt.Errorf("encryption password is empty")
	}

	key := sha256.Sum256([]byte(password))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return 0, fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, fmt.Errorf("gcm: %w", err)
	}
	if gcm.NonceSize() != 12 {
		return 0, fmt.Errorf("unexpected GCM nonce size: %d", gcm.NonceSize())
	}

	// Read header
	gotMagic := make([]byte, 8)
	if _, err := io.ReadFull(src, gotMagic); err != nil {
		return 0, err
	}
	for i := range magic {
		if gotMagic[i] != magic[i] {
			return 0, fmt.Errorf("invalid encrypted stream header")
		}
	}

	noncePrefix := make([]byte, 8)
	if _, err := io.ReadFull(src, noncePrefix); err != nil {
		return 0, err
	}

	var counter uint32
	nonce := make([]byte, 12)
	copy(nonce[:8], noncePrefix)

	var lenBuf [4]byte
	var total int64

	for {
		if _, err := io.ReadFull(src, lenBuf[:]); err != nil {
			return total, err
		}
		plainLen := binary.BigEndian.Uint32(lenBuf[:])
		if plainLen == 0 {
			break
		}

		// ciphertext length = plaintext + overhead
		cipherLen := int(plainLen) + gcm.Overhead()
		cipherBuf := make([]byte, cipherLen)
		if _, err := io.ReadFull(src, cipherBuf); err != nil {
			return total, err
		}
		binary.BigEndian.PutUint32(nonce[8:], counter)
		counter++

		plaintext, err := gcm.Open(nil, nonce, cipherBuf, nil)
		if err != nil {
			return total, fmt.Errorf("decrypt failed: %w", err)
		}

		if _, err := dst.Write(plaintext); err != nil {
			return total, err
		}
		total += int64(len(plaintext))
	}

	return total, nil

}
