package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const (
	defaultInternetTarget = "1.1.1.1"
	defaultPingCount      = 5
	pingBatchTimeout      = 10 * time.Second
)

type probeSnapshot struct {
	Gateway string          `json:"gateway"`
	Network NetworkContext  `json:"network"`
	Pings   []pingRunOutput `json:"pings"`
}

type pingRunOutput struct {
	Target     string             `json:"target"`
	TargetType TargetType         `json:"targetType"`
	Samples    []pingSampleOutput `json:"samples"`
	Error      string             `json:"error,omitempty"`
}

type pingSampleOutput struct {
	ICMPSequence int    `json:"icmpSequence"`
	RTT          string `json:"rtt"`
}

func main() {
	network := newPlatformNetworkProvider()
	gateway, err := network.DefaultGateway()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to discover gateway: %v\n", err)
		os.Exit(1)
	}

	context, err := network.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to discover current network: %v\n", err)
		os.Exit(1)
	}

	snapshot := probeSnapshot{
		Gateway: gateway,
		Network: context,
		Pings: []pingRunOutput{
			runPing(network, gateway, TargetGateway),
			runPing(network, defaultInternetTarget, TargetInternet),
		},
	}

	snapshotJSON, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode probe snapshot: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stdout, string(snapshotJSON))
}

func runPing(network NetworkProvider, target string, targetType TargetType) pingRunOutput {
	result := pingRunOutput{
		Target:     target,
		TargetType: targetType,
		Samples:    []pingSampleOutput{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), pingBatchTimeout)
	defer cancel()

	samples, err := network.Ping(ctx, target, defaultPingCount)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	for _, sample := range samples {
		result.Samples = append(result.Samples, pingSampleOutput{
			ICMPSequence: sample.IcmpSeq,
			RTT:          sample.RTT.String(),
		})
	}

	return result
}
