// Package kvm is a client for the GLKVM (PiKVM-derived kvmd) HTTP API.
//
// Endpoints (see docs/API.md in the repository):
//
//	POST /api/auth/login               form: user=admin&passwd=...  -> {token}
//	GET  /api/switch                   header token: ...            -> switch state
//	POST /api/switch/set_active?port=N (N is zero-based)
//	GET  /api/system/get_config        -> config.port_names (names set in the web UI)
package kvm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/settings"
)

const (
	adminUserName  = "admin"
	requestTimeout = 8 * time.Second
	maximumReply   = 4 << 20
)

// SwitchSettleDelay is how long to wait after switching before reading the state back.
// Tests set it to zero.
var SwitchSettleDelay = 400 * time.Millisecond

// Messages shown to the user. They match the macOS and Windows apps.
const (
	MessageNotConfigured      = "Set the KVM address and password in Settings."
	MessageLoginRejected      = "Login rejected. Check the password in Settings."
	MessageTwoFactorRequired  = "2FA is enabled on the KVM; this app does not support it."
	MessageCertificateChanged = "The KVM's certificate changed. Reset the pinned certificate in Settings if expected."
	MessageUnexpectedResponse = "Unexpected response from the KVM."
	MessageTimedOut           = "The KVM did not respond in time."
)

// Error is an error whose message can be shown directly to the user.
type Error struct {
	Message string
}

func (kvmError *Error) Error() string {
	return kvmError.Message
}

func userError(message string) error {
	return &Error{Message: message}
}

// Port is one downstream port on the KVM's built-in switch.
type Port struct {
	Index        int
	ID           string
	DeviceName   string
	HasVideoLink bool
}

// DisplayName picks the name to show: the KVM's name, then the local name, then "Port N".
func (port Port) DisplayName(current settings.Settings) string {
	if port.DeviceName != "" {
		return port.DeviceName
	}
	if localName := current.LocalName(port.Index); localName != "" {
		return localName
	}
	return fmt.Sprintf("Port %d", port.Index+1)
}

// SwitchState is a snapshot of the switch: which port is active and which ports exist.
type SwitchState struct {
	ActivePortIndex int
	Ports           []Port
}

// ActivePort returns the active port, if it is in the port list.
func (state SwitchState) ActivePort() (Port, bool) {
	if state.ActivePortIndex >= 0 && state.ActivePortIndex < len(state.Ports) {
		return state.Ports[state.ActivePortIndex], true
	}
	return Port{}, false
}

// CertificateFingerprint is the SHA-256 of a DER certificate as lowercase hex, matching the
// macOS and Windows apps.
func CertificateFingerprint(certificateDER []byte) string {
	digest := sha256.Sum256(certificateDER)
	return hex.EncodeToString(digest[:])
}

// ParseSwitchState builds a SwitchState from the "result" of GET /api/switch.
func ParseSwitchState(switchResult json.RawMessage, webUIPortNames []string) (SwitchState, error) {
	var parsed struct {
		Summary struct {
			ActivePort json.RawMessage `json:"active_port"`
		} `json:"summary"`
		Model struct {
			Ports []map[string]any `json:"ports"`
		} `json:"model"`
		Video struct {
			Links []any `json:"links"`
		} `json:"video"`
	}
	if err := json.Unmarshal(switchResult, &parsed); err != nil || parsed.Model.Ports == nil {
		return SwitchState{}, userError(MessageUnexpectedResponse)
	}
	activePortIndex, err := parseInteger(parsed.Summary.ActivePort)
	if err != nil {
		return SwitchState{}, userError(MessageUnexpectedResponse)
	}

	ports := make([]Port, 0, len(parsed.Model.Ports))
	for portIndex, rawPort := range parsed.Model.Ports {
		switchPortName := ""
		if portName, isString := rawPort["name"].(string); isString {
			switchPortName = strings.TrimSpace(portName)
		}
		webUIPortName := ""
		if portIndex < len(webUIPortNames) {
			webUIPortName = strings.TrimSpace(webUIPortNames[portIndex])
		}
		portID := strconv.Itoa(portIndex + 1)
		if rawID, present := rawPort["id"]; present && rawID != nil {
			portID = fmt.Sprint(rawID)
		}
		hasVideoLink := true
		if portIndex < len(parsed.Video.Links) {
			hasVideoLink = truthy(parsed.Video.Links[portIndex])
		}
		deviceName := switchPortName
		if deviceName == "" {
			deviceName = webUIPortName
		}
		ports = append(ports, Port{
			Index:        portIndex,
			ID:           portID,
			DeviceName:   deviceName,
			HasVideoLink: hasVideoLink,
		})
	}
	return SwitchState{ActivePortIndex: activePortIndex, Ports: ports}, nil
}

func parseInteger(rawNumber json.RawMessage) (int, error) {
	var number json.Number
	if err := json.Unmarshal(rawNumber, &number); err != nil {
		var numberText string
		if json.Unmarshal(rawNumber, &numberText) != nil {
			return 0, err
		}
		number = json.Number(strings.TrimSpace(numberText))
	}
	value, err := number.Int64()
	return int(value), err
}

func truthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case string:
		return typed != ""
	case nil:
		return false
	default:
		return true
	}
}

// unwrapResult returns "result" from kvmd's {"ok": bool, "result": ...} envelope.
func unwrapResult(statusCode int, responseBody []byte) (json.RawMessage, error) {
	var envelope struct {
		OK     *bool           `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		if statusCode >= 400 {
			return nil, userError(fmt.Sprintf("KVM error: HTTP %d", statusCode))
		}
		return nil, userError(MessageUnexpectedResponse)
	}
	if envelope.OK == nil || !*envelope.OK {
		var failure struct {
			ErrorMessage string `json:"error_msg"`
		}
		_ = json.Unmarshal(envelope.Result, &failure)
		if failure.ErrorMessage == "" {
			failure.ErrorMessage = fmt.Sprintf("HTTP %d", statusCode)
		}
		return nil, userError("KVM error: " + failure.ErrorMessage)
	}
	if len(envelope.Result) == 0 {
		return json.RawMessage("{}"), nil
	}
	return envelope.Result, nil
}

// PasswordProvider returns the stored password, or "" when there is none.
type PasswordProvider func() string

// Client logs in, caches the session token and talks to one KVM.
//
// The KVM's self-signed certificate is pinned on first use: its SHA-256 is stored in the
// settings and any later certificate that differs is refused.
type Client struct {
	settingsStore    *settings.Store
	passwordProvider PasswordProvider
	mutex            sync.Mutex
	sessionToken     string
	httpClient       *http.Client
}

// NewClient returns a client that reads the host and pin from settingsStore.
func NewClient(settingsStore *settings.Store, passwordProvider PasswordProvider) *Client {
	client := &Client{settingsStore: settingsStore, passwordProvider: passwordProvider}
	client.httpClient = &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			// Verification is done by fingerprint pinning, not by a CA chain: the KVM's
			// certificate is self-signed.
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // pinned in verifyPinnedCertificate
				VerifyConnection:   client.verifyPinnedCertificate,
				MinVersion:         tls.VersionTLS12,
			},
			Proxy:               nil,
			TLSHandshakeTimeout: requestTimeout,
			IdleConnTimeout:     30 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return client
}

// ForgetSession drops the cached token and open connections so the next request logs in
// again (after the address, password or pin changed).
func (client *Client) ForgetSession() {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	client.sessionToken = ""
	client.httpClient.CloseIdleConnections()
}

// FetchSwitchState returns the current switch state, with port names from the web UI.
func (client *Client) FetchSwitchState() (SwitchState, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	switchResult, err := client.authorizedRequest(http.MethodGet, "/api/switch")
	if err != nil {
		return SwitchState{}, err
	}
	// Names are cosmetic, so a failed config read must not block switching.
	webUIPortNames, _ := client.fetchWebUIPortNames()
	return ParseSwitchState(switchResult, webUIPortNames)
}

// SetActivePort switches to a zero-based port and gives the switch a moment to settle.
func (client *Client) SetActivePort(portIndex int) error {
	client.mutex.Lock()
	_, err := client.authorizedRequest(http.MethodPost,
		"/api/switch/set_active?port="+strconv.Itoa(portIndex))
	client.mutex.Unlock()
	if err != nil {
		return err
	}
	time.Sleep(SwitchSettleDelay)
	return nil
}

func (client *Client) fetchWebUIPortNames() ([]string, error) {
	configResult, err := client.authorizedRequest(http.MethodGet, "/api/system/get_config")
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Config struct {
			PortNames []any `json:"port_names"`
		} `json:"config"`
	}
	if json.Unmarshal(configResult, &parsed) != nil {
		return nil, nil
	}
	portNames := make([]string, 0, len(parsed.Config.PortNames))
	for _, rawName := range parsed.Config.PortNames {
		if rawName == nil {
			portNames = append(portNames, "")
			continue
		}
		portNames = append(portNames, fmt.Sprint(rawName))
	}
	return portNames, nil
}

// authorizedRequest sends with the cached token, logging in first or again after a 401/403.
func (client *Client) authorizedRequest(method, pathAndQuery string) (json.RawMessage, error) {
	if client.sessionToken == "" {
		token, err := client.login()
		if err != nil {
			return nil, err
		}
		client.sessionToken = token
	}
	statusCode, responseBody, err := client.send(method, pathAndQuery, client.sessionToken, nil, "")
	if err != nil {
		return nil, err
	}
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		token, err := client.login()
		if err != nil {
			return nil, err
		}
		client.sessionToken = token
		statusCode, responseBody, err = client.send(method, pathAndQuery, token, nil, "")
		if err != nil {
			return nil, err
		}
	}
	return unwrapResult(statusCode, responseBody)
}

func (client *Client) login() (string, error) {
	password := client.passwordProvider()
	if password == "" || client.settingsStore.Get().Host == "" {
		return "", userError(MessageNotConfigured)
	}
	// Built by hand to keep the field order the web UI uses; QueryEscape gives form encoding.
	formBody := "user=" + url.QueryEscape(adminUserName) + "&passwd=" + url.QueryEscape(password)
	statusCode, responseBody, err := client.send(http.MethodPost, "/api/auth/login", "",
		[]byte(formBody), "application/x-www-form-urlencoded")
	if err != nil {
		return "", err
	}
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return "", userError(MessageLoginRejected)
	}
	loginResult, err := unwrapResult(statusCode, responseBody)
	if err != nil {
		return "", err
	}
	var loginFields map[string]any
	if json.Unmarshal(loginResult, &loginFields) == nil {
		if token, isString := loginFields["token"].(string); isString && token != "" {
			return token, nil
		}
		_, needsTwoFactor := loginFields["need_2fa"]
		_, hasTwoFactor := loginFields["2fa"]
		if needsTwoFactor || hasTwoFactor {
			return "", userError(MessageTwoFactorRequired)
		}
	}
	return "", userError(MessageUnexpectedResponse)
}

func (client *Client) send(method, pathAndQuery, token string, body []byte, contentType string) (int, []byte, error) {
	host := client.settingsStore.Get().Host
	if host == "" {
		return 0, nil, userError(MessageNotConfigured)
	}
	requestContext, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, method,
		"https://"+host+pathAndQuery, bytes.NewReader(body))
	if err != nil {
		return 0, nil, userError("Could not reach the KVM: " + err.Error())
	}
	request.Header.Set("Accept", "application/json")
	if token != "" {
		request.Header.Set("token", token)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return 0, nil, describeTransportError(err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maximumReply))
	if err != nil {
		return 0, nil, describeTransportError(err)
	}
	return response.StatusCode, responseBody, nil
}

var errCertificateChanged = errors.New("certificate changed")

func describeTransportError(transportError error) error {
	if errors.Is(transportError, errCertificateChanged) {
		return userError(MessageCertificateChanged)
	}
	var urlError *url.Error
	if errors.As(transportError, &urlError) {
		if urlError.Timeout() {
			return userError(MessageTimedOut)
		}
		transportError = urlError.Err
	}
	if errors.Is(transportError, context.DeadlineExceeded) {
		return userError(MessageTimedOut)
	}
	return userError("Could not reach the KVM: " + transportError.Error())
}

func (client *Client) verifyPinnedCertificate(connectionState tls.ConnectionState) error {
	if len(connectionState.PeerCertificates) == 0 {
		return errCertificateChanged
	}
	fingerprint := CertificateFingerprint(connectionState.PeerCertificates[0].Raw)
	pinnedFingerprint := client.settingsStore.Get().PinnedCertificateFingerprint
	if pinnedFingerprint == "" {
		return client.settingsStore.Update(func(updated *settings.Settings) {
			updated.PinnedCertificateFingerprint = fingerprint
		})
	}
	if !strings.EqualFold(pinnedFingerprint, fingerprint) {
		return errCertificateChanged
	}
	return nil
}
