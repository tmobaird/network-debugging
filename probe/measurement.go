package main

import "time"

// TargetType describes the role a target plays in a measurement.
//
// It is deliberately separate from the target's IP address. An address tells us
// where packets were sent; the type tells us why that address is useful when we
// later compare measurements.
type TargetType string

const (
	// TargetGateway is the probe's default router. Measuring it exercises the
	// local path without intentionally traversing the ISP connection.
	TargetGateway TargetType = "gateway"

	// TargetInternet is a stable public IP. Measuring it exercises the local
	// path, router, ISP, and the route across the internet to that target.
	TargetInternet TargetType = "internet"
)

// NetworkContext records the network attachment in use when a measurement was
// taken. This context lets the collector distinguish a direct CE1000A path from
// a path through the TP-Link repeater.
//
// Some operating systems may not expose every Wi-Fi field. Pointer fields are
// nil when the value could not be observed; nil is different from a measured
// value of zero.
type NetworkContext struct {
	// Interface is the operating-system name of the network interface that
	// carried the measurement, for example "en0" on macOS or "wlan0" on Linux.
	Interface string

	// ConnectionLabel is user-provided experiment context, such as
	// "direct-verizon" or "tp-link-repeater". Unlike the remaining Wi-Fi
	// fields, this is configured rather than observed by the operating system.
	ConnectionLabel string

	// SSID is the human-readable Wi-Fi network name, such as
	// "Verizon_76Y7HS" or "Verizon_76Y7HS_EXT". Multiple radios can advertise
	// the same SSID, so this does not uniquely identify an access point.
	SSID *string

	// BSSID identifies the particular Wi-Fi radio the probe is associated
	// with. It is normally represented as a MAC address. This is more precise
	// than SSID when detecting roaming or comparing access points.
	BSSID *string

	// ChannelNumber is the primary Wi-Fi channel reported by the operating
	// system. A channel number only has meaning together with Band.
	ChannelNumber *int

	// Band identifies the Wi-Fi frequency band, for example "2.4GHz", "5GHz",
	// or "6GHz".
	Band *string

	// ChannelWidthMHz is the amount of spectrum occupied by the connection,
	// commonly 20, 40, 80, or 160 MHz.
	ChannelWidthMHz *int

	// SignalDBM is the received Wi-Fi signal strength in dBm as observed by
	// the probe. Values are normally negative; a value closer to zero means a
	// stronger received signal. Signal strength alone does not measure link
	// quality, interference, or the TP-Link's separate backhaul connection.
	SignalDBM *int

	// NoiseDBM is the background radio noise observed by the probe in dBm.
	// SignalDBM - NoiseDBM gives the signal-to-noise ratio in decibels.
	NoiseDBM *int
}

// Measurement contains the result of sending a batch of probes to one target.
// It records observations, not a diagnosis: for example, packet loss to an
// internet target does not by itself prove that the ISP is at fault.
type Measurement struct {
	// ProbeID is the stable identity of the device taking the measurement.
	// It should not change when the probe moves or joins a different SSID.
	ProbeID string

	// Timestamp is when this measurement batch began. Using time.Time keeps
	// the instant unambiguous; we will standardize its wire representation
	// when we define the collector API.
	Timestamp time.Time

	// Target is the IP address or hostname to which packets were sent.
	Target string

	// TargetType explains the target's diagnostic role, such as gateway or
	// internet. It is a label, not something inferred from the address.
	TargetType TargetType

	// Sent is the number of requests transmitted during this batch.
	Sent int

	// Received is the number of matching replies received before their
	// deadlines. Sent - Received is the observed loss count.
	Received int

	// MinRTT, AvgRTT, and MaxRTT summarize round-trip times for successful
	// replies. They do not include requests that timed out.
	MinRTT time.Duration
	AvgRTT time.Duration
	MaxRTT time.Duration

	// Jitter is the variation between successful RTT samples. We intend to
	// define it as the mean absolute difference between consecutive RTTs; the
	// calculation will be implemented and tested separately.
	Jitter time.Duration

	// Network describes the probe's active network attachment while this
	// batch was collected.
	Network NetworkContext
}
