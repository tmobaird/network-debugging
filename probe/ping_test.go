package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParsePingOutput(t *testing.T) {
	output := []byte(`PING 192.168.1.1 (192.168.1.1): 56 data bytes
64 bytes from 192.168.1.1: icmp_seq=0 ttl=64 time=3.125 ms
64 bytes from 192.168.1.1: icmp_seq=1 ttl=64 time=4.5 ms
Request timeout for icmp_seq 2
64 bytes from 192.168.1.1: icmp_seq=3 ttl=64 time=2.875 ms

--- 192.168.1.1 ping statistics ---
4 packets transmitted, 3 packets received, 25.0% packet loss
round-trip min/avg/max/stddev = 2.875/3.500/4.500/0.677 ms
`)

	got, err := parsePingOutput(output)
	if err != nil {
		t.Fatalf("parsePingOutput() error = %v", err)
	}

	want := PingResult{
		Sent:     4,
		Received: 3,
		Samples: []RTTSample{
			{IcmpSeq: 0, RTT: 3125 * time.Microsecond},
			{IcmpSeq: 1, RTT: 4500 * time.Microsecond},
			{IcmpSeq: 3, RTT: 2875 * time.Microsecond},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsePingOutput() = %#v, want %#v", got, want)
	}
}

func TestParsePingOutputAcceptsLinuxReplyFormat(t *testing.T) {
	output := []byte(`PING 1.1.1.1 (1.1.1.1) 56(84) bytes of data.
64 bytes from 1.1.1.1: icmp_seq=1 ttl=57 time=10.2 ms
64 bytes from 1.1.1.1: icmp_seq=2 ttl=57 time=9.84 ms

--- 1.1.1.1 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1001ms
rtt min/avg/max/mdev = 9.840/10.020/10.200/0.180 ms
`)

	got, err := parsePingOutput(output)
	if err != nil {
		t.Fatalf("parsePingOutput() error = %v", err)
	}

	want := PingResult{
		Sent:     2,
		Received: 2,
		Samples: []RTTSample{
			{IcmpSeq: 1, RTT: 10200 * time.Microsecond},
			{IcmpSeq: 2, RTT: 9840 * time.Microsecond},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsePingOutput() = %#v, want %#v", got, want)
	}
}

func TestParsePingOutputWithNoReplies(t *testing.T) {
	output := []byte(`PING 192.168.1.1 (192.168.1.1): 56 data bytes
Request timeout for icmp_seq 0
Request timeout for icmp_seq 1

--- 192.168.1.1 ping statistics ---
2 packets transmitted, 0 packets received, 100.0% packet loss
`)

	got, err := parsePingOutput(output)
	if err != nil {
		t.Fatalf("parsePingOutput() error = %v", err)
	}
	if got.Sent != 2 || got.Received != 0 || len(got.Samples) != 0 {
		t.Errorf("parsePingOutput() = %#v, want no successful samples", got)
	}
}

func TestParsePingOutputRejectsMalformedReplyRTT(t *testing.T) {
	output := []byte(`64 bytes from 192.168.1.1: icmp_seq=0 ttl=64 time=unknown ms
1 packets transmitted, 1 packets received, 0.0% packet loss
`)

	if _, err := parsePingOutput(output); err == nil {
		t.Fatal("parsePingOutput() error = nil, want an error")
	}
}

func TestParsePingOutputRejectsInconsistentReplyCount(t *testing.T) {
	output := []byte(`64 bytes from 192.168.1.1: icmp_seq=0 ttl=64 time=1.5 ms
2 packets transmitted, 2 packets received, 0.0% packet loss
`)

	if _, err := parsePingOutput(output); err == nil {
		t.Fatal("parsePingOutput() error = nil, want an error")
	}
}

func TestParsePingOutputRejectsDuplicateSequence(t *testing.T) {
	output := []byte(`64 bytes from 192.168.1.1: icmp_seq=0 ttl=64 time=1.5 ms
64 bytes from 192.168.1.1: icmp_seq=0 ttl=64 time=1.7 ms (DUP!)
1 packets transmitted, 1 packets received, 0.0% packet loss
`)

	if _, err := parsePingOutput(output); err == nil {
		t.Fatal("parsePingOutput() error = nil, want an error")
	}
}

func TestParsePingOutputRequiresSummary(t *testing.T) {
	output := []byte("64 bytes from 192.168.1.1: icmp_seq=0 ttl=64 time=1.5 ms\n")

	if _, err := parsePingOutput(output); err == nil {
		t.Fatal("parsePingOutput() error = nil, want an error")
	}
}

func TestPingTreatsTotalLossAsMeasurement(t *testing.T) {
	runner := &fakeCommandRunner{responses: map[string]fakeCommandResponse{
		"ping -c 2 192.168.1.1": {
			output: []byte(`PING 192.168.1.1 (192.168.1.1): 56 data bytes
Request timeout for icmp_seq 0
Request timeout for icmp_seq 1

--- 192.168.1.1 ping statistics ---
2 packets transmitted, 0 packets received, 100.0% packet loss
`),
			err: errors.New("exit status 2"),
		},
	}}
	provider := &DarwinNetworkProvider{runner: runner}

	got, err := provider.Ping(context.Background(), "192.168.1.1", 2)
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if got.Sent != 2 || got.Received != 0 || len(got.Samples) != 0 {
		t.Errorf("Ping() = %#v, want two sent and no replies", got)
	}
}

func TestPingRejectsNonPositiveCount(t *testing.T) {
	provider := &DarwinNetworkProvider{runner: &fakeCommandRunner{}}

	for _, count := range []int{0, -1} {
		if _, err := provider.Ping(context.Background(), "192.168.1.1", count); err == nil {
			t.Errorf("Ping() with count %d error = nil, want an error", count)
		}
	}
}

func TestPingReturnsContextCancellation(t *testing.T) {
	runner := &fakeCommandRunner{responses: map[string]fakeCommandResponse{
		"ping -c 2 192.168.1.1": {
			err: errors.New("signal: killed"),
		},
	}}
	provider := &DarwinNetworkProvider{runner: runner}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.Ping(ctx, "192.168.1.1", 2)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Ping() error = %v, want context.Canceled", err)
	}
}

func TestPingReturnsExecutionErrorWithoutValidSummary(t *testing.T) {
	runner := &fakeCommandRunner{responses: map[string]fakeCommandResponse{
		"ping -c 2 invalid": {
			output: []byte("ping: cannot resolve invalid: Unknown host\n"),
			err:    errors.New("exit status 68"),
		},
	}}
	provider := &DarwinNetworkProvider{runner: runner}

	_, err := provider.Ping(context.Background(), "invalid", 2)
	if err == nil || !strings.Contains(err.Error(), "cannot resolve invalid") {
		t.Fatalf("Ping() error = %v, want command output", err)
	}
}

func TestPingRejectsIncompleteBatch(t *testing.T) {
	runner := &fakeCommandRunner{responses: map[string]fakeCommandResponse{
		"ping -c 5 192.168.1.1": {
			output: []byte(`1 packets transmitted, 0 packets received, 100.0% packet loss
`),
		},
	}}
	provider := &DarwinNetworkProvider{runner: runner}

	_, err := provider.Ping(context.Background(), "192.168.1.1", 5)
	if err == nil || !strings.Contains(err.Error(), "transmitted 1 packets, want 5") {
		t.Fatalf("Ping() error = %v, want incomplete batch error", err)
	}
}
