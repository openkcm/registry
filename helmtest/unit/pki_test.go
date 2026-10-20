//go:build helmtest

package unit_test

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The file tests generate their certificates and keys at test time: the repository holds no
// key material.

// PEM block types of private key files.
const (
	pkcs8Key = "PRIVATE KEY"     // PKCS #8: RSA, ECDSA or Ed25519 key
	pkcs1Key = "RSA PRIVATE KEY" // PKCS #1: RSA key
	sec1Key  = "EC PRIVATE KEY"  // SEC 1: ECDSA key
)

// keyFormat is a format of private key files.
type keyFormat struct {
	name      string
	blockType string
	newKey    func(t *testing.T) crypto.Signer
}

// keyFormats are the formats of private key files that values.yaml documents (tls.key at
// extraVolumes).
var keyFormats = []keyFormat{
	{name: "PKCS #8 RSA", blockType: pkcs8Key, newKey: newRSAKey},
	{name: "PKCS #8 ECDSA", blockType: pkcs8Key, newKey: newECDSAKey},
	{name: "PKCS #8 Ed25519", blockType: pkcs8Key, newKey: newEd25519Key},
	{name: "PKCS #1 RSA", blockType: pkcs1Key, newKey: newRSAKey},
	{name: "SEC 1 ECDSA", blockType: sec1Key, newKey: newECDSAKey},
}

func newRSAKey(t *testing.T) crypto.Signer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	return key
}

func newECDSAKey(t *testing.T) crypto.Signer {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	return key
}

func newEd25519Key(t *testing.T) crypto.Signer {
	t.Helper()

	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	return key
}

// testCA is a certificate authority created for a test.
type testCA struct {
	cert *x509.Certificate
	key  crypto.Signer
}

// newRootCA creates a self-signed CA.
func newRootCA(t *testing.T, name string) *testCA {
	t.Helper()

	key := newECDSAKey(t)
	template := caTemplate(t, name)

	return &testCA{cert: createCertificate(t, template, template, key.Public(), key), key: key}
}

// newIntermediateCA creates a CA issued by ca.
func (ca *testCA) newIntermediateCA(t *testing.T, name string) *testCA {
	t.Helper()

	key := newECDSAKey(t)

	cert := createCertificate(t, caTemplate(t, name), ca.cert, key.Public(), ca.key)

	return &testCA{cert: cert, key: key}
}

// issue creates a certificate for TLS clients and servers with the common name and the DNS
// names.
func (ca *testCA) issue(t *testing.T, key crypto.Signer, commonName string,
	dnsNames ...string,
) *x509.Certificate {
	t.Helper()

	template := &x509.Certificate{
		SerialNumber: serialNumber(t),
		Subject:      pkix.Name{CommonName: commonName},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

	return createCertificate(t, template, ca.cert, key.Public(), ca.key)
}

func caTemplate(t *testing.T, name string) *x509.Certificate {
	t.Helper()

	return &x509.Certificate{
		SerialNumber:          serialNumber(t),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
}

func createCertificate(t *testing.T, template, parent *x509.Certificate, pub crypto.PublicKey,
	signer crypto.Signer,
) *x509.Certificate {
	t.Helper()

	der, err := x509.CreateCertificate(rand.Reader, template, parent, pub, signer)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return cert
}

func serialNumber(t *testing.T) *big.Int {
	t.Helper()

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
	require.NoError(t, err)

	return serial
}

// certPEM encodes the certificates as CERTIFICATE blocks, in the given order.
func certPEM(certs ...*x509.Certificate) []byte {
	var out bytes.Buffer
	for _, cert := range certs {
		_ = pem.Encode(&out, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	}

	return out.Bytes()
}

// keyPEM encodes the private key as a PEM block of the given type.
func keyPEM(t *testing.T, key crypto.Signer, blockType string) []byte {
	t.Helper()

	var (
		der []byte
		err error
	)

	switch blockType {
	case pkcs8Key:
		der, err = x509.MarshalPKCS8PrivateKey(key)
	case pkcs1Key:
		rsaKey, ok := key.(*rsa.PrivateKey)
		require.True(t, ok, "PKCS #1 needs an RSA key")

		der = x509.MarshalPKCS1PrivateKey(rsaKey)
	case sec1Key:
		ecKey, ok := key.(*ecdsa.PrivateKey)
		require.True(t, ok, "SEC 1 needs an ECDSA key")

		der, err = x509.MarshalECPrivateKey(ecKey)
	default:
		t.Fatalf("unknown private key block type %q", blockType)
	}

	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
}

// testPKI are the CAs of a file test: a root CA, an intermediate CA issued by it that issues
// the client and server certificates, and an unrelated CA.
type testPKI struct {
	root, intermediate, other *testCA
}

func newTestPKI(t *testing.T) *testPKI {
	t.Helper()

	root := newRootCA(t, "Test Root CA")

	return &testPKI{
		root:         root,
		intermediate: root.newIntermediateCA(t, "Test Intermediate CA"),
		other:        newRootCA(t, "Other CA"),
	}
}

// clientFiles returns the content of the files of the client "registry" in the layout that
// values.yaml documents: tls.crt with the client certificate and then the intermediate CA,
// tls.key with its private key in the given format, and ca.crt with two CA certificates, an
// unrelated one and the root CA of the server certificates.
func (p *testPKI) clientFiles(t *testing.T, format keyFormat) ([]byte, []byte, []byte) {
	t.Helper()

	key := format.newKey(t)
	cert := p.intermediate.issue(t, key, "registry")

	return certPEM(cert, p.intermediate.cert), keyPEM(t, key, format.blockType),
		certPEM(p.other.cert, p.root.cert)
}

// server returns the certificate chain of a TLS server for the DNS name, issued by the
// intermediate CA.
func (p *testPKI) server(t *testing.T, dnsName string) tls.Certificate {
	t.Helper()

	key := newECDSAKey(t)
	cert := p.intermediate.issue(t, key, dnsName, dnsName)

	return tls.Certificate{
		Certificate: [][]byte{cert.Raw, p.intermediate.cert.Raw},
		PrivateKey:  key,
		Leaf:        cert,
	}
}

// rootPool returns a pool with the root CA.
func (p *testPKI) rootPool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(p.root.cert)

	return pool
}

// handshake is what a test server learned from the TLS handshake of a client.
type handshake struct {
	err   error
	chain []*x509.Certificate // certificates the client sent, verified if err is nil
}

// serveTLS listens on 127.0.0.1 and runs a TLS server handshake with every client: first
// prelude on the plain connection, if not nil (for example the SSLRequest of a PostgreSQL
// client), then the handshake, which requires a client certificate issued by a CA in
// clientCAs. Then it closes the connection. It returns the port and the results of the
// handshakes.
func serveTLS(t *testing.T, cert tls.Certificate, clientCAs *x509.CertPool,
	prelude func(net.Conn) error,
) (string, <-chan handshake) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	results := make(chan handshake, 1)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			select {
			case results <- serverHandshake(conn, cert, clientCAs, prelude):
			default: // only the first result is read
			}
		}
	}()

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	return port, results
}

func serverHandshake(conn net.Conn, cert tls.Certificate, clientCAs *x509.CertPool,
	prelude func(net.Conn) error,
) handshake {
	defer func() { _ = conn.Close() }()

	err := conn.SetDeadline(time.Now().Add(10 * time.Second))
	if err != nil {
		return handshake{err: err}
	}

	if prelude != nil {
		err = prelude(conn)
		if err != nil {
			return handshake{err: err}
		}
	}

	tlsConn := tls.Server(conn, &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
		MinVersion:   tls.VersionTLS12,
	})
	err = tlsConn.Handshake()

	return handshake{err: err, chain: tlsConn.ConnectionState().PeerCertificates}
}

// firstHandshake returns the result of the first handshake of a server of serveTLS.
func firstHandshake(t *testing.T, results <-chan handshake) handshake {
	t.Helper()

	select {
	case result := <-results:
		return result
	case <-time.After(15 * time.Second):
		t.Fatal("no client connected to the test server")
		return handshake{}
	}
}
