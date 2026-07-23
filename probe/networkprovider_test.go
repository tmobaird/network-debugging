package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseDefaultRoute(t *testing.T) {
	output := `
   route to: default
destination: default
    gateway: 192.168.1.1
  interface: en0
`

	got, err := parseDefaultRoute(output)
	if err != nil {
		t.Fatalf("parseDefaultRoute() error = %v", err)
	}

	want := defaultRoute{Gateway: "192.168.1.1", Interface: "en0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDefaultRoute() = %#v, want %#v", got, want)
	}
}

func TestParseDefaultRouteRequiresGatewayAndInterface(t *testing.T) {
	for name, output := range map[string]string{
		"missing gateway":   "interface: en0\n",
		"missing interface": "gateway: 192.168.1.1\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseDefaultRoute(output); err == nil {
				t.Fatal("parseDefaultRoute() error = nil, want an error")
			}
		})
	}
}

func TestParseWiFiSnapshot(t *testing.T) {
	output := []byte(`{
  "band": "5GHz",
  "channelNumber": 64,
  "channelWidthMHz": 80,
  "interface": "en0",
  "noiseDBM": -89,
  "signalDBM": -36
}`)

	got, err := parseWiFiSnapshot(output)
	if err != nil {
		t.Fatalf("parseWiFiSnapshot() error = %v", err)
	}

	if got.Interface != "en0" || got.ChannelNumber == nil || *got.ChannelNumber != 64 {
		t.Errorf("parseWiFiSnapshot() = %#v", got)
	}
	if got.SignalDBM == nil || *got.SignalDBM != -36 {
		t.Errorf("SignalDBM = %v, want -36", got.SignalDBM)
	}
}

func TestParseWiFiSnapshotRejectsInvalidOutput(t *testing.T) {
	for name, output := range map[string][]byte{
		"invalid JSON":      []byte("not JSON"),
		"missing interface": []byte(`{"signalDBM":-36}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseWiFiSnapshot(output); err == nil {
				t.Fatal("parseWiFiSnapshot() error = nil, want an error")
			}
		})
	}
}

func TestParseIPConfigWiFiIdentity(t *testing.T) {
	output := `
interface_type : WiFi
SSID : ExampleNet_EXT
BSSID : aa:bb:cc:dd:ee:ff
`

	got, err := parseIPConfigWiFiIdentity(output)
	if err != nil {
		t.Fatalf("parseIPConfigWiFiIdentity() error = %v", err)
	}

	if got.SSID == nil || *got.SSID != "ExampleNet_EXT" {
		t.Errorf("SSID = %v, want ExampleNet_EXT", got.SSID)
	}
	if got.BSSID == nil || *got.BSSID != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("BSSID = %v, want aa:bb:cc:dd:ee:ff", got.BSSID)
	}
}

func TestCurrentCombinesCoreWLANAndIPConfig(t *testing.T) {
	runner := &fakeCommandRunner{responses: map[string]fakeCommandResponse{
		"route -n get default": {
			output: []byte("gateway: 192.168.1.1\ninterface: en0\n"),
		},
		"/tmp/wifi-snapshot en0": {
			output: []byte(`{"interface":"en0","channelNumber":64,"band":"5GHz","channelWidthMHz":80,"signalDBM":-36,"noiseDBM":-89}`),
		},
		"/usr/sbin/ipconfig getsummary en0": {
			output: []byte("SSID : ExampleNet_EXT\nBSSID : aa:bb:cc:dd:ee:ff\n"),
		},
	}}
	provider := &DarwinNetworkProvider{
		runner:          runner,
		wifiHelperPath:  "/tmp/wifi-snapshot",
		connectionLabel: "tp-link-repeater",
	}

	got, err := provider.Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	if got.Interface != "en0" || got.ConnectionLabel != "tp-link-repeater" {
		t.Errorf("Current() identity = %#v", got)
	}
	if got.SSID == nil || *got.SSID != "ExampleNet_EXT" {
		t.Errorf("Current().SSID = %v", got.SSID)
	}
	if got.SignalDBM == nil || *got.SignalDBM != -36 {
		t.Errorf("Current().SignalDBM = %v", got.SignalDBM)
	}

	wantCalls := []string{
		"route -n get default",
		"/tmp/wifi-snapshot en0",
		"/usr/sbin/ipconfig getsummary en0",
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Errorf("commands = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestCurrentRunsWiFiSourceWithSwift(t *testing.T) {
	runner := &fakeCommandRunner{responses: map[string]fakeCommandResponse{
		"route -n get default": {
			output: []byte("gateway: 192.168.1.1\ninterface: en0\n"),
		},
		"swift tools/wifi-snapshot/main.swift en0": {
			output: []byte(`{"interface":"en0","signalDBM":-36}`),
		},
		"/usr/sbin/ipconfig getsummary en0": {
			err: errors.New("Wi-Fi identity unavailable"),
		},
	}}
	provider := &DarwinNetworkProvider{
		runner:              runner,
		wifiHelperPath:      defaultWiFiHelperSource,
		wifiHelperUsesSwift: true,
	}

	got, err := provider.Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if got.SignalDBM == nil || *got.SignalDBM != -36 {
		t.Errorf("Current().SignalDBM = %v, want -36", got.SignalDBM)
	}

	wantCalls := []string{
		"route -n get default",
		"swift tools/wifi-snapshot/main.swift en0",
		"/usr/sbin/ipconfig getsummary en0",
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Errorf("commands = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestCurrentAllowsUnavailableWiFiEnrichment(t *testing.T) {
	runner := &fakeCommandRunner{responses: map[string]fakeCommandResponse{
		"route -n get default": {
			output: []byte("gateway: 192.168.1.1\ninterface: en0\n"),
		},
	}}
	provider := &DarwinNetworkProvider{
		runner:         runner,
		wifiHelperPath: "/missing/wifi-snapshot",
	}

	got, err := provider.Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if got.Interface != "en0" {
		t.Errorf("Current().Interface = %q, want en0", got.Interface)
	}
	if got.SSID != nil || got.SignalDBM != nil {
		t.Errorf("Current() Wi-Fi fields = %#v, want unavailable", got)
	}
}

type fakeCommandResponse struct {
	output []byte
	err    error
}

type fakeCommandRunner struct {
	responses map[string]fakeCommandResponse
	calls     []string
}

func (f *fakeCommandRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	f.calls = append(f.calls, key)

	response, ok := f.responses[key]
	if !ok {
		return nil, errors.New("unexpected command: " + key)
	}
	return response.output, response.err
}
