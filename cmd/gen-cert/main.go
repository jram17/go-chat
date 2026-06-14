package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"time"
)

func main() {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),

		Subject: pkix.Name{
			CommonName: "localhost",
		},

		NotBefore: time.Now(),

		NotAfter: time.Now().Add(
			365 * 24 * time.Hour,
		),

		KeyUsage: x509.KeyUsageDigitalSignature |
			x509.KeyUsageKeyEncipherment,

		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},

		DNSNames: []string{
			"localhost",
		},
	}

	derBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&priv.PublicKey,
		priv,
	)

	if err != nil {
		panic(err)
	}

	os.MkdirAll("certs", 0755)

	certOut, err := os.Create(
		"certs/server.crt",
	)

	if err != nil {
		panic(err)
	}

	pem.Encode(certOut, &pem.Block{
		Type: "CERTIFICATE",
		Bytes: derBytes,
	})

	certOut.Close()

	keyBytes, err := x509.MarshalECPrivateKey(
		priv,
	)

	if err != nil {
		panic(err)
	}

	keyOut, err := os.Create(
		"certs/server.key",
	)

	if err != nil {
		panic(err)
	}

	pem.Encode(keyOut, &pem.Block{
		Type: "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	keyOut.Close()
}