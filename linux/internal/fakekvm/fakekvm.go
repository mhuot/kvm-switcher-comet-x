// Package fakekvm is an HTTPS server that answers like a GL.iNet KVM from the shared
// fixtures in the repository's fixtures/ folder. It is used only by tests.
package fakekvm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// FixturesDirectory is the repository's fixtures/ folder.
func FixturesDirectory() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "fixtures")
}

// FixtureText returns the contents of one fixture file.
func FixtureText(fileName string) string {
	fixtureBytes, err := os.ReadFile(filepath.Join(FixturesDirectory(), fileName))
	if err != nil {
		panic(err)
	}
	return string(fixtureBytes)
}

// RecordedRequest is one request the fake KVM received.
type RecordedRequest struct {
	Method string
	Path   string
	Query  string
	Token  string
	Body   string
}

// Route is a canned reply: an HTTP status and a fixture file name.
type Route struct {
	StatusCode  int
	FixtureFile string
}

// Server is a running fake KVM.
type Server struct {
	*httptest.Server
	mutex    sync.Mutex
	requests []RecordedRequest
	// Routes maps a path to its reply. Tests may change it between requests.
	Routes map[string]Route
	// Responder, when set, replaces Routes and returns a status and a response body.
	Responder func(RecordedRequest) (int, string)
}

// Start runs a fake KVM with a fresh self-signed certificate and the default routes.
func Start() *Server {
	fake := &Server{Routes: map[string]Route{
		"/api/auth/login":        {http.StatusOK, "login_success.json"},
		"/api/switch":            {http.StatusOK, "switch_state.json"},
		"/api/switch/set_active": {http.StatusOK, "set_active_success.json"},
		"/api/system/get_config": {http.StatusOK, "get_config.json"},
	}}
	// Each fake KVM gets its own certificate, like real KVMs, so pinning can be tested.
	fake.Server = httptest.NewUnstartedServer(http.HandlerFunc(fake.serve))
	fake.Server.TLS = &tls.Config{Certificates: []tls.Certificate{selfSignedCertificate()}}
	fake.Server.StartTLS()
	return fake
}

func selfSignedCertificate() tls.Certificate {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	serialNumber, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		panic(err)
	}
	certificateTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{CommonName: "glkvm.local"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, certificateTemplate,
		certificateTemplate, &privateKey.PublicKey, privateKey)
	if err != nil {
		panic(err)
	}
	return tls.Certificate{Certificate: [][]byte{certificateDER}, PrivateKey: privateKey}
}

// Host returns host:port for the settings.
func (fake *Server) Host() string {
	return strings.TrimPrefix(fake.URL, "https://")
}

// SetRoute replaces the reply for one path.
func (fake *Server) SetRoute(path string, statusCode int, fixtureFile string) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.Routes[path] = Route{statusCode, fixtureFile}
}

// Requests returns every request received so far.
func (fake *Server) Requests() []RecordedRequest {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return append([]RecordedRequest(nil), fake.requests...)
}

// RequestsTo returns the requests received for one path.
func (fake *Server) RequestsTo(path string) []RecordedRequest {
	var matching []RecordedRequest
	for _, recorded := range fake.Requests() {
		if recorded.Path == path {
			matching = append(matching, recorded)
		}
	}
	return matching
}

func (fake *Server) serve(writer http.ResponseWriter, request *http.Request) {
	requestBody, _ := io.ReadAll(request.Body)
	recorded := RecordedRequest{
		Method: request.Method,
		Path:   request.URL.Path,
		Query:  request.URL.RawQuery,
		Token:  request.Header.Get("token"),
		Body:   string(requestBody),
	}
	fake.mutex.Lock()
	fake.requests = append(fake.requests, recorded)
	responder := fake.Responder
	route, routeFound := fake.Routes[recorded.Path]
	fake.mutex.Unlock()

	statusCode, responseBody := http.StatusNotFound, ""
	switch {
	case responder != nil:
		statusCode, responseBody = responder(recorded)
	case routeFound:
		statusCode, responseBody = route.StatusCode, FixtureText(route.FixtureFile)
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	_, _ = io.WriteString(writer, responseBody)
}
