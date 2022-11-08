package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

func getDomainList() (domainList []string) {
	domainList = configData.Gemini.DomainNames[:]
	if len(domainList) == 0 {
		domainList = append(domainList, "localhost")
	}
	if torAddress != "" {
		domainList = append(domainList, torAddress)
	}
	return
}

// Generate self-signed TLS certificate and key.  Uses ed25519 for the
// private key
func generateNewTLSCertAndKey() (cert, key []byte) {
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	handleErr(err, "Unable to generate TLS ed25519 private key")
	return generateNewTLSCertFromKey(privKey)
}

// Generate self-signed TLS certificate from private key.
func generateNewTLSCertFromKey(privKey crypto.PrivateKey) (cert, key []byte) {
	// Generate TLS ed25519 key
	pubKey := publicKeyFromPrivateKey(privKey)
	// Get random 128-bit integer (bigInt)
	serialNumber, err := rand.Int(rand.Reader,
		new(big.Int).Lsh(big.NewInt(1), 128))
	handleErr(err, "Unable to generate random 128-bit integer for TLS "+
		"certificate serial number")
	// Set NotBefore to January 1st, 2000 at midnight UTC
	notBefore, err := time.Parse(time.RFC3339, "2000-01-01T00:00:00Z")
	handleErr(err, "Unable to create NotBefore value for TLS certificate")
	// Set NotAfter to January 1st, 2200 at midnight UTC
	notAfter, err := time.Parse(time.RFC3339, "2200-01-01T00:00:00Z")
	handleErr(err, "Unable to create NotAfter value for TLS certificate")
	keyUsage := x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	if _, keyIsRSA := privKey.(*rsa.PrivateKey); keyIsRSA {
		// Set KeyEncipherment KeyUsage bit if privKey is RSA
		keyUsage |= x509.KeyUsageKeyEncipherment
	}
	tlsCertTemplate := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              getDomainList(),
	}
	// Create x509 certificate from tls certificate template and ed25519
	// public/private key
	certDERBytes, err := x509.CreateCertificate(rand.Reader, &tlsCertTemplate,
		&tlsCertTemplate, pubKey, privKey)
	handleErr(err, "Unable to generate TLS certificate")
	// Encode x509 certificate to pem encoding
	cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE",
		Bytes: certDERBytes})
	// Create x509 private key from ed25519 private key
	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	// Encode x509 private key to pem encoding
	key = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY",
		Bytes: privKeyBytes})
	return
}

func loadTLSCert() tls.Certificate {
	// tlsPrivKey will be the TLS private key if it exists and is valid,
	// otherwise will be nil
	tlsPrivKey := getTLSKey()
	// Attempt to read/load TLS certificate and key
	cert, err := tls.LoadX509KeyPair(configData.Gemini.TLS.CertPath,
		configData.Gemini.TLS.KeyPath)
	if err != nil {
		// Could not load TLS certificate and key
		var tlsCert []byte
		var tlsKey []byte
		if tlsPrivKey == nil {
			// No valid TLS key, so generate TLS certificate and key
			fmt.Println("- Generating new TLS certificate and TLS private key")
			tlsCert, tlsKey = generateNewTLSCertAndKey()
		} else {
			// Valid TLS key, so generate TLS certificate
			fmt.Println("- Generating new TLS certificate")
			tlsCert, tlsKey = generateNewTLSCertFromKey(tlsPrivKey)
		}
		// Write generated TLS certificate to cert path
		fmt.Printf("- Writing TLS certificate to %s\n", configData.Gemini.TLS.CertPath)
		writeTLSCert(tlsCert)
		if tlsPrivKey == nil {
			// Write generated TLS private key to key path if not valid TLS key
			fmt.Printf("- Writing TLS private key to %s\n", configData.Gemini.TLS.KeyPath)
			writeTLSKey(tlsKey)
		}
		// Load generated TLS certificate and key
		cert, err = tls.X509KeyPair(tlsCert, tlsKey)
		handleErr(err, "Unable to load generated TLS certificate and key")
		return cert
	}
	// Get x509 certificate data from TLS certificate
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	handleErr(err, "Unable to parse TLS certificate")
	// Check if all domain names for gemini capsule are in TLS certificate
	domainList := getDomainList()
	for _, domain := range domainList {
		domainInCert := false
		for _, certDomain := range x509Cert.DNSNames {
			if domain == certDomain {
				domainInCert = true
				break
			}
		}
		if !domainInCert || len(domainList) != len(x509Cert.DNSNames) {
			// Domain is not in cert or cert contains a domain not in the domain
			// list so generate and write new TLS certificate (but not key)
			fmt.Println("- Generating new TLS certificate from TLS private key")
			tlsCert, tlsKey := generateNewTLSCertFromKey(cert.PrivateKey)
			fmt.Printf("- Writing TLS certificate to %s\n", configData.Gemini.TLS.CertPath)
			writeTLSCert(tlsCert)
			cert, err = tls.X509KeyPair(tlsCert, tlsKey)
			handleErr(err, "Unable to load generated TLS certificate and key")
			return cert
		}
	}
	return cert
}

// Write TLS certificate to config TLS certificate path
func writeTLSCert(cert []byte) {
	handleErr(os.WriteFile(configData.Gemini.TLS.CertPath, cert, 0600),
		fmt.Sprintf("Unable to write TLS certificate file %s",
			configData.Gemini.TLS.CertPath))
}

// Write TLS private key to config TLS key path
func writeTLSKey(key []byte) {
	handleErr(os.WriteFile(configData.Gemini.TLS.KeyPath, key, 0600),
		fmt.Sprintf("Unable to write TLS key file %s",
			configData.Gemini.TLS.KeyPath))
}

func publicKeyFromPrivateKey(privKey any) any {
	switch key := privKey.(type) {
	case ed25519.PrivateKey:
		return key.Public()
	case *ecdsa.PrivateKey:
		return &key.PublicKey
	case *rsa.PrivateKey:
		return &key.PublicKey
	default:
		return nil
	}
}

// Attempt to read TLS private key.  Returns private key if it exists and
// is valid
func getTLSKey() crypto.PrivateKey {
	keyBytes, err := os.ReadFile(configData.Gemini.TLS.KeyPath)
	if err != nil {
		return nil
	}
	privKeyPEM, _ := pem.Decode(keyBytes)
	privKey, _ := x509.ParsePKCS8PrivateKey(privKeyPEM.Bytes)
	return privKey
}
