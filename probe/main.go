package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	defaultInternetTarget = "1.1.1.1"
	defaultPingCount      = 5
	pingBatchTimeout      = 10 * time.Second
)

type pingRunOutput struct {
	SchemaVersion int                `json:"schemaVersion"`
	Timestamp     time.Time          `json:"timestamp"`
	Target        string             `json:"target"`
	TargetType    TargetType         `json:"targetType"`
	Sent          int                `json:"sent"`
	Received      int                `json:"received"`
	Samples       []pingSampleOutput `json:"samples"`
	Network       NetworkContext     `json:"network"`
	Error         string             `json:"error,omitempty"`
}

type pingSampleOutput struct {
	ICMPSequence int     `json:"icmpSequence"`
	RTTMillis    float64 `json:"rttMs"`
}

func main() {
	interval := flag.Duration(
		"interval",
		0,
		"time between measurement cycle starts; zero runs one cycle",
	)
	duration := flag.Duration(
		"duration",
		0,
		"total experiment duration; zero runs until interrupted when interval is set",
	)
	flag.Parse()

	if err := validateSchedule(*interval, *duration); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)

	err := runExperiment(
		ctx,
		newPlatformNetworkProvider(),
		encoder,
		os.Stderr,
		*interval,
	)
	if err != nil && !errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) {
		fmt.Fprintf(os.Stderr, "probe failed: %v\n", err)
		os.Exit(1)
	}
}

func validateSchedule(interval, duration time.Duration) error {
	if interval < 0 {
		return errors.New("--interval must not be negative")
	}
	if duration < 0 {
		return errors.New("--duration must not be negative")
	}
	if duration > 0 && interval == 0 {
		return errors.New("--duration requires a positive --interval")
	}
	return nil
}

func runExperiment(
	ctx context.Context,
	network NetworkProvider,
	encoder *json.Encoder,
	stderr io.Writer,
	interval time.Duration,
) error {
	nextCycle := time.Now()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := collectCycle(ctx, network, encoder); err != nil {
			if interval == 0 {
				return err
			}
			fmt.Fprintf(stderr, "measurement cycle failed: %v\n", err)
		}

		if interval == 0 {
			return nil
		}

		nextCycle = nextCycle.Add(interval)
		now := time.Now()
		for !nextCycle.After(now) {
			nextCycle = nextCycle.Add(interval)
		}

		timer := time.NewTimer(time.Until(nextCycle))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func collectCycle(
	ctx context.Context,
	network NetworkProvider,
	encoder *json.Encoder,
) error {
	gateway, err := network.DefaultGateway()
	if err != nil {
		return fmt.Errorf("discover gateway: %w", err)
	}

	networkContext, err := network.Current()
	if err != nil {
		return fmt.Errorf("discover current network: %w", err)
	}

	targets := []struct {
		address    string
		targetType TargetType
	}{
		{address: gateway, targetType: TargetGateway},
		{address: defaultInternetTarget, targetType: TargetInternet},
	}

	for _, target := range targets {
		result := runPing(ctx, network, networkContext, target.address, target.targetType)
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("encode ping result: %w", err)
		}
	}

	return nil
}

func runPing(
	parent context.Context,
	network NetworkProvider,
	networkContext NetworkContext,
	target string,
	targetType TargetType,
) pingRunOutput {
	result := pingRunOutput{
		SchemaVersion: 1,
		Timestamp:     time.Now().UTC(),
		Target:        target,
		TargetType:    targetType,
		Samples:       []pingSampleOutput{},
		Network:       networkContext,
	}

	ctx, cancel := context.WithTimeout(parent, pingBatchTimeout)
	defer cancel()

	pingResult, err := network.Ping(ctx, target, defaultPingCount)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Sent = pingResult.Sent
	result.Received = pingResult.Received
	for _, sample := range pingResult.Samples {
		result.Samples = append(result.Samples, pingSampleOutput{
			ICMPSequence: sample.IcmpSeq,
			RTTMillis:    float64(sample.RTT) / float64(time.Millisecond),
		})
	}

	return result
}
