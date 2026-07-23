package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeNetworkProvider struct {
	result  PingResult
	pingErr error
}

func (f fakeNetworkProvider) DefaultGateway() (string, error) {
	return "192.168.1.1", nil
}

func (f fakeNetworkProvider) Current() (NetworkContext, error) {
	return NetworkContext{Interface: "en0"}, nil
}

func (f fakeNetworkProvider) Ping(context.Context, string, int) (PingResult, error) {
	return f.result, f.pingErr
}

func TestRunPingFormatsSamples(t *testing.T) {
	network := fakeNetworkProvider{
		result: PingResult{
			Sent:     1,
			Received: 1,
			Samples: []RTTSample{
				{IcmpSeq: 0, RTT: 3125 * time.Microsecond},
			},
		},
	}

	got := runPing(
		context.Background(),
		network,
		NetworkContext{Interface: "en0"},
		"192.168.1.1",
		TargetGateway,
	)

	if got.Target != "192.168.1.1" || got.TargetType != TargetGateway {
		t.Errorf("runPing() target = %#v", got)
	}
	if len(got.Samples) != 1 {
		t.Fatalf("runPing() samples = %#v, want one sample", got.Samples)
	}
	if got.Samples[0].ICMPSequence != 0 || got.Samples[0].RTTMillis != 3.125 {
		t.Errorf("runPing() sample = %#v", got.Samples[0])
	}
	if got.Sent != 1 || got.Received != 1 {
		t.Errorf("runPing() counts = sent %d, received %d", got.Sent, got.Received)
	}
	if got.Error != "" {
		t.Errorf("runPing() error = %q, want empty", got.Error)
	}
	if got.Timestamp.IsZero() || got.Timestamp.Location() != time.UTC {
		t.Errorf("runPing() timestamp = %v, want non-zero UTC", got.Timestamp)
	}
}

func TestRunPingRecordsError(t *testing.T) {
	network := fakeNetworkProvider{pingErr: errors.New("ping timed out")}

	got := runPing(
		context.Background(),
		network,
		NetworkContext{Interface: "en0"},
		"1.1.1.1",
		TargetInternet,
	)

	if got.Error != "ping timed out" {
		t.Errorf("runPing() error = %q, want ping timed out", got.Error)
	}
	if got.Samples == nil || len(got.Samples) != 0 {
		t.Errorf("runPing() samples = %#v, want empty non-nil slice", got.Samples)
	}
}

func TestValidateSchedule(t *testing.T) {
	for name, test := range map[string]struct {
		interval time.Duration
		duration time.Duration
		wantErr  bool
	}{
		"one shot":             {},
		"continuous":           {interval: time.Minute},
		"bounded experiment":   {interval: time.Minute, duration: 3 * time.Hour},
		"negative interval":    {interval: -time.Second, wantErr: true},
		"negative duration":    {duration: -time.Second, wantErr: true},
		"duration no interval": {duration: time.Hour, wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			err := validateSchedule(test.interval, test.duration)
			if (err != nil) != test.wantErr {
				t.Errorf("validateSchedule() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestCollectCycleWritesOneJSONLinePerTarget(t *testing.T) {
	network := fakeNetworkProvider{
		result: PingResult{
			Sent:     1,
			Received: 1,
			Samples: []RTTSample{
				{IcmpSeq: 0, RTT: 2 * time.Millisecond},
			},
		},
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)

	if err := collectCycle(context.Background(), network, encoder); err != nil {
		t.Fatalf("collectCycle() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("collectCycle() wrote %d lines, want 2\n%s", len(lines), output.String())
	}

	var gateway pingRunOutput
	if err := json.Unmarshal([]byte(lines[0]), &gateway); err != nil {
		t.Fatalf("decode gateway JSONL record: %v", err)
	}
	if gateway.Target != "192.168.1.1" || gateway.TargetType != TargetGateway {
		t.Errorf("gateway record = %#v", gateway)
	}
	if gateway.Samples[0].RTTMillis != 2 {
		t.Errorf("gateway RTT = %v ms, want 2", gateway.Samples[0].RTTMillis)
	}

	var internet pingRunOutput
	if err := json.Unmarshal([]byte(lines[1]), &internet); err != nil {
		t.Fatalf("decode internet JSONL record: %v", err)
	}
	if internet.Target != defaultInternetTarget || internet.TargetType != TargetInternet {
		t.Errorf("internet record = %#v", internet)
	}
}
