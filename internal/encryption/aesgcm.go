package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
)

var magic = []byte("BKENC001") // 8 bytes

// EncryptAESGCM encrypts src into dst using AES-256-GCM.
//uniqur nonce per chunk and framing
//end is plaintextLen = 0

func EncryptAESGCM(dst io.Writer, src io.Reader, password string) (int64, error) {
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

	//using 12-byte nonces: 8-byte random prefix + 4-byte counter.
	if gcm.NonceSize() != 12 {
		return 0, fmt.Errorf("unexpected GCM nonce size: %d", gcm.NonceSize())
	}

	//unique per backup
	noncePrefix := make([]byte, 8)
	if _, err := rand.Read(noncePrefix); err != nil {
		return 0, fmt.Errorf("nonce prefix: %w", err)
	}

	//writing header
	if _, err := dst.Write(magic); err != nil {
		return 0, err
	}

	if _, err := dst.Write(noncePrefix); err != nil {
		return 0, err
	}

	buf := make([]byte, 32*1024)
	var total int64
	var counter uint32

	var lenBuf [4]byte
	nonce := make([]byte, 12)
	copy(nonce[:8], noncePrefix)
	//keeps memory low for large dumps
	for {
		n, readErr := src.Read(buf)
		if n > 0 {

			// nonce = prefix + counter
			binary.BigEndian.PutUint32(nonce[8:], counter)
			counter++

			//frame length
			binary.BigEndian.PutUint32(lenBuf[:], uint32(n))
			if _, err := dst.Write(lenBuf[:]); err != nil {
				return total, err
			}

			//cipher text
			ciphertext := gcm.Seal(nil, nonce, buf[:n], nil)
			if _, err := dst.Write(ciphertext); err != nil {
				return total, err
			}

		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return total, readErr
		}
	}

	//end maker. length should be 0
	binary.BigEndian.PutUint32(lenBuf[:], 0)
	if _, err := dst.Write(lenBuf[:]); err != nil {
		return total, err
	}

	return total, nil
}