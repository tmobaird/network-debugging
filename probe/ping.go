package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type RTTSample struct {
	IcmpSeq int
	RTT     time.Duration
}

type PingResult struct {
	Sent     int
	Received int
	Samples  []RTTSample
}

var pingReplyPattern = regexp.MustCompile(
	`\bicmp_seq=(\d+)\b.*\btime=([0-9]+(?:\.[0-9]+)?)\s*ms\b`,
)

var pingSummaryPattern = regexp.MustCompile(
	`(?m)^(\d+)\s+packets transmitted,\s+(\d+)(?:\s+packets)?\s+received,`,
)

func (n *DarwinNetworkProvider) Ping(ctx context.Context, target string, count int) (PingResult, error) {
	if count <= 0 {
		return PingResult{}, errors.New("ping count must be positive")
	}

	output, err := n.runner.Output(ctx, "ping", "-c", strconv.Itoa(count), target)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return PingResult{}, fmt.Errorf("ping %s: %w", target, ctxErr)
	}

	result, parseErr := parsePingOutput(output)
	if parseErr == nil {
		if result.Sent != count {
			return PingResult{}, fmt.Errorf(
				"ping %s transmitted %d packets, want %d",
				target,
				result.Sent,
				count,
			)
		}

		// ping commonly exits nonzero when every request is lost. A complete,
		// internally consistent summary is still a valid network measurement.
		return result, nil
	}

	if err != nil {
		return PingResult{}, commandError("ping "+target, output, err)
	}

	return PingResult{}, parseErr
}

func parsePingOutput(output []byte) (PingResult, error) {
	result := PingResult{Samples: []RTTSample{}}
	sequences := make(map[int]struct{})

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		match := pingReplyPattern.FindStringSubmatch(line)
		if match == nil {
			// ICMP error lines can contain a sequence number without an RTT.
			// They represent unsuccessful requests, not RTT samples.
			if strings.Contains(line, "icmp_seq=") && strings.Contains(line, "time=") {
				return PingResult{}, fmt.Errorf("parse ping reply line %q", line)
			}
			continue
		}

		sequence, err := strconv.Atoi(match[1])
		if err != nil {
			return PingResult{}, fmt.Errorf("parse ping sequence %q: %w", match[1], err)
		}
		if _, duplicate := sequences[sequence]; duplicate {
			return PingResult{}, fmt.Errorf("duplicate ping sequence %d", sequence)
		}
		sequences[sequence] = struct{}{}

		rtt, err := time.ParseDuration(match[2] + "ms")
		if err != nil {
			return PingResult{}, fmt.Errorf("parse ping RTT %q: %w", match[2], err)
		}

		result.Samples = append(result.Samples, RTTSample{
			IcmpSeq: sequence,
			RTT:     rtt,
		})
	}

	if err := scanner.Err(); err != nil {
		return PingResult{}, fmt.Errorf("read ping output: %w", err)
	}

	summary := pingSummaryPattern.FindSubmatch(output)
	if summary == nil {
		return PingResult{}, errors.New("ping output contained no statistics summary")
	}

	sent, err := strconv.Atoi(string(summary[1]))
	if err != nil {
		return PingResult{}, fmt.Errorf("parse transmitted count %q: %w", summary[1], err)
	}
	received, err := strconv.Atoi(string(summary[2]))
	if err != nil {
		return PingResult{}, fmt.Errorf("parse received count %q: %w", summary[2], err)
	}
	if received > sent {
		return PingResult{}, fmt.Errorf(
			"ping summary received %d packets but transmitted %d",
			received,
			sent,
		)
	}
	if received != len(result.Samples) {
		return PingResult{}, fmt.Errorf(
			"ping summary reported %d replies but parsed %d RTT samples",
			received,
			len(result.Samples),
		)
	}

	result.Sent = sent
	result.Received = received
	return result, nil
}
