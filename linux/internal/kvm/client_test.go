package kvm_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/fakekvm"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/kvm"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/settings"
)

const testPassword = "correct horse"

func init() {
	kvm.SwitchSettleDelay = 0
}

func newStore(t *testing.T, initial settings.Settings) *settings.Store {
	t.Helper()
	store := settings.Open(filepath.Join(t.TempDir(), "settings.json"))
	if err := store.Update(func(updated *settings.Settings) { *updated = initial }); err != nil {
		t.Fatal(err)
	}
	return store
}

func newClient(t *testing.T, host string) (*kvm.Client, *settings.Store) {
	t.Helper()
	store := newStore(t, settings.Settings{Host: host, LabelStyle: settings.LabelStylePortName})
	return kvm.NewClient(store, func() string { return testPassword }), store
}

func startFake(t *testing.T) *fakekvm.Server {
	t.Helper()
	fake := fakekvm.Start()
	t.Cleanup(fake.Close)
	return fake
}

func requireUserError(t *testing.T, err error, wantSubstring string) {
	t.Helper()
	var userError *kvm.Error
	if !errors.As(err, &userError) {
		t.Fatalf("got %v (%T), want a *kvm.Error containing %q", err, err, wantSubstring)
	}
	if !strings.Contains(userError.Message, wantSubstring) {
		t.Fatalf("got %q, want it to contain %q", userError.Message, wantSubstring)
	}
}

// Every platform must parse the shared fixtures to the same result.
func TestSharedFixtureCases(t *testing.T) {
	casesBytes, err := os.ReadFile(filepath.Join(fakekvm.FixturesDirectory(), "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Cases []struct {
			Name     string `json:"name"`
			Switch   string `json:"switch"`
			Config   string `json:"config"`
			Expected struct {
				ActivePortIndex int `json:"active_port_index"`
				Ports           []struct {
					DisplayName  string `json:"display_name"`
					HasVideoLink bool   `json:"has_video_link"`
				} `json:"ports"`
			} `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(casesBytes, &contract); err != nil {
		t.Fatal(err)
	}
	if len(contract.Cases) == 0 {
		t.Fatal("no contract cases found")
	}
	for _, contractCase := range contract.Cases {
		t.Run(contractCase.Name, func(t *testing.T) {
			fake := startFake(t)
			fake.SetRoute("/api/switch", 200, contractCase.Switch)
			fake.SetRoute("/api/system/get_config", 200, contractCase.Config)
			client, store := newClient(t, fake.Host())

			switchState, err := client.FetchSwitchState()
			if err != nil {
				t.Fatal(err)
			}
			if switchState.ActivePortIndex != contractCase.Expected.ActivePortIndex {
				t.Errorf("active port %d, want %d", switchState.ActivePortIndex,
					contractCase.Expected.ActivePortIndex)
			}
			if len(switchState.Ports) != len(contractCase.Expected.Ports) {
				t.Fatalf("%d ports, want %d", len(switchState.Ports), len(contractCase.Expected.Ports))
			}
			for portIndex, expectedPort := range contractCase.Expected.Ports {
				gotPort := switchState.Ports[portIndex]
				if gotName := gotPort.DisplayName(store.Get()); gotName != expectedPort.DisplayName {
					t.Errorf("port %d name %q, want %q", portIndex, gotName, expectedPort.DisplayName)
				}
				if gotPort.HasVideoLink != expectedPort.HasVideoLink {
					t.Errorf("port %d video %v, want %v", portIndex, gotPort.HasVideoLink,
						expectedPort.HasVideoLink)
				}
			}
		})
	}
}

func TestLogsInOnceAndSendsTokenHeader(t *testing.T) {
	fake := startFake(t)
	client, _ := newClient(t, fake.Host())
	for range 2 {
		if _, err := client.FetchSwitchState(); err != nil {
			t.Fatal(err)
		}
	}
	if logins := fake.RequestsTo("/api/auth/login"); len(logins) != 1 {
		t.Fatalf("%d logins, want 1", len(logins))
	}
	switchRequests := fake.RequestsTo("/api/switch")
	if len(switchRequests) != 2 || switchRequests[0].Token != strings.Repeat("0", 64) {
		t.Fatalf("switch requests %+v", switchRequests)
	}
}

func TestLoginFormEncodesSpecialCharacters(t *testing.T) {
	fake := startFake(t)
	store := newStore(t, settings.Settings{Host: fake.Host()})
	client := kvm.NewClient(store, func() string { return "p&ss+word= ü" })
	if _, err := client.FetchSwitchState(); err != nil {
		t.Fatal(err)
	}
	logins := fake.RequestsTo("/api/auth/login")
	if len(logins) != 1 || logins[0].Method != "POST" {
		t.Fatalf("logins %+v", logins)
	}
	if want := "user=admin&passwd=p%26ss%2Bword%3D+%C3%BC"; logins[0].Body != want {
		t.Fatalf("body %q, want %q", logins[0].Body, want)
	}
}

func TestExpiredTokenTriggersOneReloginAndRetry(t *testing.T) {
	fake := startFake(t)
	switchCallCount := 0
	fake.Responder = func(recorded fakekvm.RecordedRequest) (int, string) {
		switch recorded.Path {
		case "/api/switch":
			switchCallCount++
			if switchCallCount == 1 {
				return 401, fakekvm.FixtureText("error_unauthorized.json")
			}
			return 200, fakekvm.FixtureText("switch_state.json")
		case "/api/auth/login":
			return 200, fakekvm.FixtureText("login_success.json")
		default:
			return 200, fakekvm.FixtureText("get_config.json")
		}
	}
	client, _ := newClient(t, fake.Host())
	switchState, err := client.FetchSwitchState()
	if err != nil {
		t.Fatal(err)
	}
	if switchState.ActivePortIndex != 2 {
		t.Fatalf("active port %d, want 2", switchState.ActivePortIndex)
	}
	if logins := fake.RequestsTo("/api/auth/login"); len(logins) != 2 {
		t.Fatalf("%d logins, want 2", len(logins))
	}
}

func TestWrongPasswordReportsLoginRejected(t *testing.T) {
	fake := startFake(t)
	fake.SetRoute("/api/auth/login", 403, "error_forbidden.json")
	client, _ := newClient(t, fake.Host())
	_, err := client.FetchSwitchState()
	requireUserError(t, err, "Login rejected")
}

func TestMissingPasswordReportsNotConfiguredWithoutNetwork(t *testing.T) {
	fake := startFake(t)
	store := newStore(t, settings.Settings{Host: fake.Host()})
	_, err := kvm.NewClient(store, func() string { return "" }).FetchSwitchState()
	requireUserError(t, err, "Set the KVM address")
	if requests := fake.Requests(); len(requests) != 0 {
		t.Fatalf("sent %d requests without a password", len(requests))
	}
}

func TestEmptyHostReportsNotConfigured(t *testing.T) {
	client, _ := newClient(t, "")
	_, err := client.FetchSwitchState()
	requireUserError(t, err, "Set the KVM address")
}

func TestSetActivePortSendsZeroBasedIndex(t *testing.T) {
	fake := startFake(t)
	client, _ := newClient(t, fake.Host())
	if err := client.SetActivePort(3); err != nil {
		t.Fatal(err)
	}
	setActiveRequests := fake.RequestsTo("/api/switch/set_active")
	if len(setActiveRequests) != 1 || setActiveRequests[0].Method != "POST" ||
		setActiveRequests[0].Query != "port=3" {
		t.Fatalf("set_active requests %+v", setActiveRequests)
	}
}

func TestConfigFailureFallsBackToDefaultNames(t *testing.T) {
	fake := startFake(t)
	fake.SetRoute("/api/system/get_config", 500, "error_forbidden.json")
	client, store := newClient(t, fake.Host())
	switchState, err := client.FetchSwitchState()
	if err != nil {
		t.Fatal(err)
	}
	var gotNames []string
	for _, port := range switchState.Ports {
		gotNames = append(gotNames, port.DisplayName(store.Get()))
	}
	if strings.Join(gotNames, ",") != "Port 1,Port 2,Port 3,Port 4" {
		t.Fatalf("names %q", gotNames)
	}
}

func TestLocalNamesFillGapsButKvmNamesWin(t *testing.T) {
	fake := startFake(t)
	fake.SetRoute("/api/system/get_config", 200, "get_config_without_port_names.json")
	store := newStore(t, settings.Settings{
		Host:           fake.Host(),
		LocalPortNames: []string{"Local One", "", "Local Three"},
	})
	client := kvm.NewClient(store, func() string { return testPassword })
	switchState, err := client.FetchSwitchState()
	if err != nil {
		t.Fatal(err)
	}
	var gotNames []string
	for _, port := range switchState.Ports {
		gotNames = append(gotNames, port.DisplayName(store.Get()))
	}
	if strings.Join(gotNames, ",") != "Local One,Port 2,Local Three,Port 4" {
		t.Fatalf("names %q", gotNames)
	}

	fake.SetRoute("/api/system/get_config", 200, "get_config.json")
	switchState, err = client.FetchSwitchState()
	if err != nil {
		t.Fatal(err)
	}
	if gotName := switchState.Ports[0].DisplayName(store.Get()); gotName != "Workstation" {
		t.Fatalf("name %q, want Workstation", gotName)
	}
}

func TestAPIErrorMessageIsSurfaced(t *testing.T) {
	fake := startFake(t)
	fake.SetRoute("/api/switch/set_active", 200, "error_forbidden.json")
	client, _ := newClient(t, fake.Host())
	requireUserError(t, client.SetActivePort(1), "KVM error: Forbidden")
}

func TestCertificateIsPinnedOnFirstUseAndSaved(t *testing.T) {
	fake := startFake(t)
	client, store := newClient(t, fake.Host())
	if _, err := client.FetchSwitchState(); err != nil {
		t.Fatal(err)
	}
	pinnedFingerprint := store.Get().PinnedCertificateFingerprint
	if len(pinnedFingerprint) != 64 {
		t.Fatalf("pinned fingerprint %q", pinnedFingerprint)
	}
	if want := kvm.CertificateFingerprint(fake.Certificate().Raw); pinnedFingerprint != want {
		t.Fatalf("pinned %s, want %s", pinnedFingerprint, want)
	}
	if reloaded := settings.Open(store.Path()).Get(); reloaded.PinnedCertificateFingerprint != pinnedFingerprint {
		t.Fatalf("reloaded pin %q", reloaded.PinnedCertificateFingerprint)
	}
}

func TestChangedCertificateIsRefusedWithoutSendingThePassword(t *testing.T) {
	fake := startFake(t)
	secondFake := startFake(t)
	client, store := newClient(t, fake.Host())
	if _, err := client.FetchSwitchState(); err != nil {
		t.Fatal(err)
	}

	// Same settings, a KVM at a new address with another certificate.
	if err := store.Update(func(updated *settings.Settings) { updated.Host = secondFake.Host() }); err != nil {
		t.Fatal(err)
	}
	client.ForgetSession()
	_, err := client.FetchSwitchState()
	requireUserError(t, err, "certificate changed")
	if requests := secondFake.Requests(); len(requests) != 0 {
		t.Fatalf("sent %d requests to a KVM with the wrong certificate", len(requests))
	}
}

func TestUnreachableKvmReportsConnectionError(t *testing.T) {
	client, _ := newClient(t, "127.0.0.1:9")
	_, err := client.FetchSwitchState()
	requireUserError(t, err, "Could not reach the KVM")
}

func TestParseRejectsMalformedState(t *testing.T) {
	for _, malformed := range []string{`[]`, `{}`, `{"summary":{"active_port":"x"},"model":{"ports":[]}}`} {
		if _, err := kvm.ParseSwitchState(json.RawMessage(malformed), nil); err == nil {
			t.Errorf("parsed %s without an error", malformed)
		}
	}
}
