package proxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

func LoadOrCreateCA(storagePath string) (*x509.Certificate, *rsa.PrivateKey, error) {
	certPath := filepath.Join(storagePath, "ca.crt")
	keyPath := filepath.Join(storagePath, "ca.key")

	if _, err := os.Stat(certPath); err == nil {
		certBytes, _ := os.ReadFile(certPath)
		keyBytes, _ := os.ReadFile(keyPath)

		certBlock, _ := pem.Decode(certBytes)
		keyBlock, _ := pem.Decode(keyBytes)

		caCert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return nil, nil, err
		}
		caKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, nil, err
		}
		return caCert, caKey, nil
	}

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			Organization: []string{"GoProxy MITM"},
			CommonName:   "GoProxy Root CA",
		},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	savePEM(certPath, "CERTIFICATE", certBytes)
	savePEM(keyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(caKey))

	caCert, _ := x509.ParseCertificate(certBytes)
	return caCert, caKey, nil
}

func savePEM(path, pemType string, b []byte) {
	f, _ := os.Create(path)
	defer f.Close()
	pem.Encode(f, &pem.Block{Type: pemType, Bytes: b})
}
