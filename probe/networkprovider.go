package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	defaultWiFiHelperSource = "tools/wifi-snapshot/main.swift"
	commandTimeout          = 2 * time.Second
	wifiSnapshotTimeout     = 10 * time.Second
)

type NetworkProvider interface {
	DefaultGateway() (string, error)
	Current() (NetworkContext, error)
	Ping(ctx context.Context, target string, count int) ([]RTTSample, error)
}

type commandRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execCommandRunner struct{}

func (execCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type DarwinNetworkProvider struct {
	runner              commandRunner
	wifiHelperPath      string
	wifiHelperUsesSwift bool
	connectionLabel     string
}

func newPlatformNetworkProvider() NetworkProvider {
	helperPath := os.Getenv("WIFI_SNAPSHOT_HELPER")
	usesSwift := false
	if helperPath == "" {
		helperPath = defaultWiFiHelperSource
		usesSwift = true
	}

	return &DarwinNetworkProvider{
		runner:              execCommandRunner{},
		wifiHelperPath:      helperPath,
		wifiHelperUsesSwift: usesSwift,
		connectionLabel:     os.Getenv("PROBE_CONNECTION_LABEL"),
	}
}

func (n *DarwinNetworkProvider) DefaultGateway() (string, error) {
	route, err := n.defaultRoute()
	if err != nil {
		return "", err
	}
	return route.Gateway, nil
}

func (n *DarwinNetworkProvider) Current() (NetworkContext, error) {
	route, err := n.defaultRoute()
	if err != nil {
		return NetworkContext{}, err
	}

	result := NetworkContext{
		Interface:       route.Interface,
		ConnectionLabel: n.connectionLabel,
	}

	// Wi-Fi enrichment is best effort. Route discovery is required for network
	// measurements, but missing helper binaries or privacy-redacted Wi-Fi fields
	// must not prevent gateway and internet checks from running.
	if snapshot, err := n.wifiSnapshot(route.Interface); err == nil {
		applyWiFiSnapshot(&result, snapshot)
	}

	if identity, err := n.ipconfigWiFiIdentity(route.Interface); err == nil {
		// Prefer Core WLAN if it supplied these fields. ipconfig is only a
		// development fallback because its verbose output is not a stable API.
		if result.SSID == nil {
			result.SSID = identity.SSID
		}
		if result.BSSID == nil {
			result.BSSID = identity.BSSID
		}
	}

	return result, nil
}

func (n *DarwinNetworkProvider) defaultRoute() (defaultRoute, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	output, err := n.runner.Output(ctx, "route", "-n", "get", "default")
	if err != nil {
		return defaultRoute{}, commandError("get default route", output, err)
	}
	return parseDefaultRoute(string(output))
}

func parseDefaultRoute(output string) (defaultRoute, error) {
	var result defaultRoute

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), ":")
		if !found {
			continue
		}

		switch strings.TrimSpace(key) {
		case "gateway":
			result.Gateway = strings.TrimSpace(value)
		case "interface":
			result.Interface = strings.TrimSpace(value)
		}
	}

	if err := scanner.Err(); err != nil {
		return defaultRoute{}, fmt.Errorf("read route output: %w", err)
	}

	if result.Gateway == "" {
		return defaultRoute{}, errors.New("default route contained no gateway")
	}

	if result.Interface == "" {
		return defaultRoute{}, errors.New("default route contained no interface")
	}

	return result, nil
}

type darwinWiFiSnapshot struct {
	Interface       string  `json:"interface"`
	SSID            *string `json:"ssid"`
	BSSID           *string `json:"bssid"`
	ChannelNumber   *int    `json:"channelNumber"`
	Band            *string `json:"band"`
	ChannelWidthMHz *int    `json:"channelWidthMHz"`
	SignalDBM       *int    `json:"signalDBM"`
	NoiseDBM        *int    `json:"noiseDBM"`
}

func (n *DarwinNetworkProvider) wifiSnapshot(interfaceName string) (darwinWiFiSnapshot, error) {
	if n.wifiHelperPath == "" {
		return darwinWiFiSnapshot{}, errors.New("Wi-Fi helper path is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), wifiSnapshotTimeout)
	defer cancel()

	command := n.wifiHelperPath
	args := []string{interfaceName}
	if n.wifiHelperUsesSwift {
		command = "swift"
		args = []string{n.wifiHelperPath, interfaceName}
	}

	output, err := n.runner.Output(ctx, command, args...)
	if err != nil {
		return darwinWiFiSnapshot{}, commandError("run Wi-Fi helper", output, err)
	}

	return parseWiFiSnapshot(output)
}

func parseWiFiSnapshot(output []byte) (darwinWiFiSnapshot, error) {
	var snapshot darwinWiFiSnapshot
	if err := json.Unmarshal(output, &snapshot); err != nil {
		return darwinWiFiSnapshot{}, fmt.Errorf("decode Wi-Fi helper output: %w", err)
	}
	if snapshot.Interface == "" {
		return darwinWiFiSnapshot{}, errors.New("Wi-Fi helper returned no interface")
	}
	return snapshot, nil
}

func applyWiFiSnapshot(context *NetworkContext, snapshot darwinWiFiSnapshot) {
	context.SSID = snapshot.SSID
	context.BSSID = snapshot.BSSID
	context.ChannelNumber = snapshot.ChannelNumber
	context.Band = snapshot.Band
	context.ChannelWidthMHz = snapshot.ChannelWidthMHz
	context.SignalDBM = snapshot.SignalDBM
	context.NoiseDBM = snapshot.NoiseDBM
}

type wifiIdentity struct {
	SSID  *string
	BSSID *string
}

func (n *DarwinNetworkProvider) ipconfigWiFiIdentity(interfaceName string) (wifiIdentity, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	output, err := n.runner.Output(ctx, "/usr/sbin/ipconfig", "getsummary", interfaceName)
	if err != nil {
		return wifiIdentity{}, commandError("get IP configuration summary", output, err)
	}

	return parseIPConfigWiFiIdentity(string(output))
}

func parseIPConfigWiFiIdentity(output string) (wifiIdentity, error) {
	var result wifiIdentity

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), ":")
		if !found {
			continue
		}

		value = strings.TrimSpace(value)
		if value == "" || strings.EqualFold(value, "<redacted>") {
			continue
		}

		switch strings.TrimSpace(key) {
		case "SSID":
			result.SSID = stringPointer(value)
		case "BSSID":
			result.BSSID = stringPointer(value)
		}
	}

	if err := scanner.Err(); err != nil {
		return wifiIdentity{}, fmt.Errorf("read IP configuration summary: %w", err)
	}
	if result.SSID == nil && result.BSSID == nil {
		return wifiIdentity{}, errors.New("IP configuration summary contained no Wi-Fi identity")
	}

	return result, nil
}

func stringPointer(value string) *string {
	return &value
}

func commandError(action string, output []byte, err error) error {
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w: %s", action, err, detail)
}

type defaultRoute struct {
	Gateway   string
	Interface string
}
