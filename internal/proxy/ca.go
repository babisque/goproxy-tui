package proxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"time"
)

func LoadOrCreateCA() (caCert *x509.Certificate, caKey *rsa.PrivateKey, err error) {
	certFile := "ca.crt"
	keyFile := "ca.key"

	if _, err := os.Stat(certFile); err == nil {
		certBytes, _ := os.ReadFile(certFile)
		keyBytes, _ := os.ReadFile(keyFile)

		certBlock, _ := pem.Decode(certBytes)
		keyBlock, _ := pem.Decode(keyBytes)

		caCert, _ = x509.ParseCertificate(certBlock.Bytes)
		caKey, _ = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		return caCert, caKey, nil
	}

	caKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			Organization: []string{"Rodrigo GoProxy MITM"},
			CommonName:   "Rodrigo Proxy Root CA",
		},
		NotBefore:             time.Now(),
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

	savePEM(certFile, "CERTIFICATE", certBytes)
	savePEM(keyFile, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(caKey))

	caCert, _ = x509.ParseCertificate(certBytes)
	return caCert, caKey, nil
}

func savePEM(filename, pemType string, bytes []byte) {
	file, _ := os.Create(filename)
	defer file.Close()
	pem.Encode(file, &pem.Block{Type: pemType, Bytes: bytes})
}
