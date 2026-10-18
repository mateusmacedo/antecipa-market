package rpc_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
)

type material struct {
	caFile, certFile, keyFile string
}

func writePEM(t *testing.T, path, kind string, der []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func issue(t *testing.T) material {
	t.Helper()
	dir := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "bff"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() = %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey() = %v", err)
	}
	m := material{
		caFile:   filepath.Join(dir, "ca.pem"),
		certFile: filepath.Join(dir, "bff.crt"),
		keyFile:  filepath.Join(dir, "bff.key"),
	}
	writePEM(t, m.caFile, "CERTIFICATE", der)
	writePEM(t, m.certFile, "CERTIFICATE", der)
	writePEM(t, m.keyFile, "EC PRIVATE KEY", keyDER)
	return m
}

func TestClientTLSRefusesAnUnreadableAuthority(t *testing.T) {
	m := issue(t)

	if _, err := rpc.ClientTLS(filepath.Join(t.TempDir(), "absent.pem"), "identity", m.certFile, m.keyFile); err == nil {
		t.Fatal("ClientTLS() with an absent authority = nil, want an error")
	}
}

func TestClientTLSRefusesAnAuthorityWithoutPEM(t *testing.T) {
	m := issue(t)
	empty := filepath.Join(t.TempDir(), "empty.pem")
	if err := os.WriteFile(empty, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := rpc.ClientTLS(empty, "identity", m.certFile, m.keyFile); err == nil {
		t.Fatal("ClientTLS() with no PEM certificate = nil, want an error")
	}
}

func TestClientTLSRefusesAnInvalidPair(t *testing.T) {
	m := issue(t)
	other := issue(t)

	if _, err := rpc.ClientTLS(m.caFile, "identity", m.certFile, other.keyFile); err == nil {
		t.Fatal("ClientTLS() with a key that does not match the certificate = nil, want an error")
	}
	if _, err := rpc.ClientTLS(m.caFile, "identity", m.certFile, filepath.Join(t.TempDir(), "absent.key")); err == nil {
		t.Fatal("ClientTLS() with an absent key = nil, want an error")
	}
}

func TestClientTLSBuildsTheClientSide(t *testing.T) {
	m := issue(t)

	config, err := rpc.ClientTLS(m.caFile, "identity.internal", m.certFile, m.keyFile)

	if err != nil {
		t.Fatalf("ClientTLS() = %v", err)
	}
	if config.MinVersion != tls.VersionTLS12 || config.ServerName != "identity.internal" {
		t.Fatalf("config = MinVersion %#x, ServerName %q; want TLS 1.2 and identity.internal", config.MinVersion, config.ServerName)
	}
	if config.RootCAs == nil || len(config.Certificates) != 1 {
		t.Fatalf("config = %d certificates and RootCAs %v; want the authority and one client pair", len(config.Certificates), config.RootCAs)
	}
}
