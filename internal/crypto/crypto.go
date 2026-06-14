package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

func GenerateKeyPair() (privateKey, publicKey []byte, err error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	pub := priv.PublicKey()
	return priv.Bytes(), pub.Bytes(), nil
}

func ComputeSharedSecret(myPrivate, theirPublic []byte) ([]byte, error) {
	curve := ecdh.X25519()

	priv, err := curve.NewPrivateKey(myPrivate)
	if err != nil {
		return nil, err
	}
	pub, err := curve.NewPublicKey(theirPublic)
	if err != nil {
		return nil, err
	}
	sharedSecret, err := priv.ECDH(pub)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(sharedSecret)

	return hash[:], nil
}

func Encrypt(key []byte, plaintext []byte) ([]byte, error) {
	//create a key first
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}
	nounce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nounce); err != nil {
		return nil, err
	}
	return gcm.Seal(nounce, nounce, plaintext, nil), nil
}

func Decrypt(key []byte, ciphertext []byte) ([]byte, error) {
	//create a key first
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return plaintext, nil

}
