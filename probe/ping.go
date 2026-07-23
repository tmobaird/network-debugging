package main

import (
	"bufio"
	"bytes"
	"context"
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

var pingReplyPattern = regexp.MustCompile(
	`\bicmp_seq=(\d+)\b.*\btime=([0-9]+(?:\.[0-9]+)?)\s*ms\b`,
)

func (n *DarwinNetworkProvider) Ping(ctx context.Context, target string, count int) ([]RTTSample, error) {
	output, err := n.runner.Output(ctx, "ping", "-c", strconv.Itoa(count), target)
	if err != nil {
		return nil, commandError("ping "+target, output, err)
	}
	return parsePingOutput(output)
}

func parsePingOutput(output []byte) ([]RTTSample, error) {
	var samples []RTTSample

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		match := pingReplyPattern.FindStringSubmatch(line)
		if match == nil {
			// ICMP error lines can contain a sequence number without an RTT.
			// They represent unsuccessful requests, not RTT samples.
			if strings.Contains(line, "icmp_seq=") && strings.Contains(line, "time=") {
				return nil, fmt.Errorf("parse ping reply line %q", line)
			}
			continue
		}

		sequence, err := strconv.Atoi(match[1])
		if err != nil {
			return nil, fmt.Errorf("parse ping sequence %q: %w", match[1], err)
		}

		rtt, err := time.ParseDuration(match[2] + "ms")
		if err != nil {
			return nil, fmt.Errorf("parse ping RTT %q: %w", match[2], err)
		}

		samples = append(samples, RTTSample{
			IcmpSeq: sequence,
			RTT:     rtt,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read ping output: %w", err)
	}

	return samples, nil
}
