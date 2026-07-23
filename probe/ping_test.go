package main

import (
	"reflect"
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

	want := []RTTSample{
		{IcmpSeq: 0, RTT: 3125 * time.Microsecond},
		{IcmpSeq: 1, RTT: 4500 * time.Microsecond},
		{IcmpSeq: 3, RTT: 2875 * time.Microsecond},
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

	want := []RTTSample{
		{IcmpSeq: 1, RTT: 10200 * time.Microsecond},
		{IcmpSeq: 2, RTT: 9840 * time.Microsecond},
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
	if len(got) != 0 {
		t.Errorf("parsePingOutput() = %#v, want no successful samples", got)
	}
}

func TestParsePingOutputRejectsMalformedReplyRTT(t *testing.T) {
	output := []byte(
		"64 bytes from 192.168.1.1: icmp_seq=0 ttl=64 time=unknown ms\n",
	)

	if _, err := parsePingOutput(output); err == nil {
		t.Fatal("parsePingOutput() error = nil, want an error")
	}
}
