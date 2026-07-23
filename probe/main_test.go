package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeNetworkProvider struct {
	samples []RTTSample
	pingErr error
}

func (f fakeNetworkProvider) DefaultGateway() (string, error) {
	return "192.168.1.1", nil
}

func (f fakeNetworkProvider) Current() (NetworkContext, error) {
	return NetworkContext{Interface: "en0"}, nil
}

func (f fakeNetworkProvider) Ping(context.Context, string, int) ([]RTTSample, error) {
	return f.samples, f.pingErr
}

func TestRunPingFormatsSamples(t *testing.T) {
	network := fakeNetworkProvider{
		samples: []RTTSample{
			{IcmpSeq: 0, RTT: 3125 * time.Microsecond},
		},
	}

	got := runPing(network, "192.168.1.1", TargetGateway)

	if got.Target != "192.168.1.1" || got.TargetType != TargetGateway {
		t.Errorf("runPing() target = %#v", got)
	}
	if len(got.Samples) != 1 {
		t.Fatalf("runPing() samples = %#v, want one sample", got.Samples)
	}
	if got.Samples[0].ICMPSequence != 0 || got.Samples[0].RTT != "3.125ms" {
		t.Errorf("runPing() sample = %#v", got.Samples[0])
	}
	if got.Error != "" {
		t.Errorf("runPing() error = %q, want empty", got.Error)
	}
}

func TestRunPingRecordsError(t *testing.T) {
	network := fakeNetworkProvider{pingErr: errors.New("ping timed out")}

	got := runPing(network, "1.1.1.1", TargetInternet)

	if got.Error != "ping timed out" {
		t.Errorf("runPing() error = %q, want ping timed out", got.Error)
	}
	if got.Samples == nil || len(got.Samples) != 0 {
		t.Errorf("runPing() samples = %#v, want empty non-nil slice", got.Samples)
	}
}
