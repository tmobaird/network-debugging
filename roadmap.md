# Roadmap

## Current milestone: trustworthy probe measurements

The project currently has a working macOS probe and a local experiment-analysis
workflow. The next objective is to finish the probe's stable measurement
contract, validate it through controlled experiments, and then implement the
Linux/Raspberry Pi provider before building the collector.

### Measurement model

- [x] Define gateway and internet target roles.
- [x] Separate configured connection labels from observed network context.
- [x] Represent unavailable Wi-Fi metadata without zero-valued measurements.
- [x] Record interface, SSID, BSSID, channel, band, channel width, signal, and
      noise when available.
- [x] Derive SNR as `signalDBM - noiseDBM` instead of storing it separately.
- [ ] Convert each raw ping result into a complete `Measurement`.
- [ ] Represent RTT summaries as unavailable when there are no successful
      replies.
- [ ] Represent jitter as unavailable when fewer than two replies succeed.
- [ ] Add stable probe ID, experiment ID, and physical location as separate
      configured dimensions.
- [ ] Define and document the collector-facing JSON contract and duration units.

### macOS network provider

- [x] Discover the default gateway and active interface.
- [x] Collect Wi-Fi context through the Swift Core WLAN helper.
- [x] Allow `ipconfig` as a best-effort SSID/BSSID development fallback.
- [x] Run the Swift source directly for zero-setup development.
- [x] Support a precompiled helper override for long-running experiments.
- [x] Bound external commands with timeouts.
- [x] Test command execution through an injected runner.
- [ ] Surface best-effort Wi-Fi enrichment failures without stopping
      measurements.
- [ ] Avoid repeating route discovery when collecting one measurement cycle.

### Ping measurement

- [x] Use the operating system's `ping` command for Phase 1.
- [x] Send counted batches to the gateway and `1.1.1.1`.
- [x] Parse individual successful RTT samples.
- [x] Parse macOS and Linux-style packet summaries.
- [x] Distinguish valid packet loss from command-execution failure.
- [x] Preserve 100% loss as a valid measurement despite `ping`'s nonzero exit.
- [x] Handle cancellation, malformed output, inconsistent counts, duplicate
      sequences, and incomplete batches.
- [x] Keep gateway and internet attempts independent.
- [x] Define jitter as the mean absolute difference between consecutive
      successful RTT samples.
- [ ] Calculate min, average, max, loss, and jitter inside the Go probe.
- [ ] Add platform-specific Linux command arguments and integration tests.

### Experiment collection

- [x] Emit compact JSONL with UTC timestamps and numeric RTT milliseconds.
- [x] Emit one record per target batch.
- [x] Add `--interval` and `--duration`.
- [x] Prevent overlapping measurement cycles and skip missed cadence slots.
- [x] Handle interrupt, termination, and experiment-duration cancellation.
- [x] Add `probe/experiment.sh` to compile the helper and probe, run under
      `caffeinate`, and create timestamped data and diagnostics files.
- [ ] Avoid recording expected experiment-boundary cancellation as an
      operational network error.
- [ ] Add experiment metadata fields to the runner and JSONL records.

## Current milestone: controlled home experiments

- [x] Capture an initial floor-3 dataset from a known unreliable location.
- [x] Confirm that the two observed BSSIDs belong to the Fios router's 2.4 GHz
      and 5 GHz radios.
- [x] Compare gateway and internet RTT, loss, jitter, signal, noise, and SNR.
- [ ] Capture a floor-2 baseline near the Verizon CE1000A.
- [ ] Identify the CE1000A BSSIDs during the floor-2 baseline.
- [ ] Install a TP-Link access point with Ethernet backhaul on floor 3 or 4.
- [ ] Verify the TP-Link is in access-point mode and the Verizon router remains
      the default gateway.
- [ ] Run controlled wired-TP-Link experiments with forced 5 GHz, forced
      2.4 GHz, and automatic band selection.
- [ ] Repeat comparisons from the same probe position and orientation.
- [ ] Add transition-centered analysis around band and BSSID changes.

The current working hypothesis is that floors 3 and 4 are near the usable edge
of the Verizon 5 GHz coverage. Weak-edge 5 GHz behavior, fallback to a more
contended 2.4 GHz band, or both may contribute to elevated local RTT. The
experiments must distinguish sustained band behavior from the brief transition
event itself.

## Experiment analysis

- [x] Validate JSONL schema and measurement invariants.
- [x] Calculate batch loss, RTT summaries, jitter, signal, noise, and SNR.
- [x] Produce grouped terminal summaries, flattened CSV, and summary JSON.
- [x] Build a local Streamlit and Plotly dashboard.
- [x] Support interactive filtering and grouping by connection label, target,
      band, BSSID, SSID, channel, and interface.
- [x] Support raw, 5-minute, 15-minute, and hourly aggregation.
- [x] Visualize time series, RTT distributions, and RTT versus signal/SNR.
- [ ] Mark band and BSSID transitions directly on time-series charts.
- [ ] Calculate before/after transition statistics.
- [ ] Detect cadence gaps and contiguous loss or latency incidents.
- [ ] Add configured physical location and access-point identity to dashboard
      filters.

## Linux and Raspberry Pi probe

- [ ] Select the provider at build time or runtime by operating system.
- [ ] Discover the Linux default gateway and active interface.
- [ ] Collect Linux Wi-Fi context without requiring privileged probe operation.
- [ ] Validate Linux `ping` arguments, timeouts, output, and exit behavior.
- [ ] Run the probe unattended as a service.
- [ ] Add a real configuration-file loader.

This milestone succeeds when the same measurement contract works on the
development Mac and the first Raspberry Pi probe.

## Collector and historical observability

- [ ] Define the versioned probe-to-collector API.
- [ ] Build the initial Go collector service.
- [ ] Add probe delivery timeouts, retries, and partial-failure behavior.
- [ ] Preserve probe timestamps while recording collector receive timestamps.
- [ ] Expose stable batch summaries to Prometheus.
- [ ] Build Grafana views for gateway latency, internet latency, loss, jitter,
      radio context, and probe availability.
- [ ] Run the collector, Prometheus, and Grafana through Docker Compose.

Phase 1 succeeds when one physical probe continuously produces trustworthy
measurements that can be viewed historically in Grafana.

## Later diagnostic capabilities

- [ ] DNS resolution and latency measurements.
- [ ] TCP or HTTPS reachability measurements.
- [ ] Interface statistics and connection events.
- [ ] Throughput testing with explicit scheduling and rate limits.
- [ ] Multi-probe correlation.
- [ ] Incident detection.
- [ ] Explainable health states and root-cause hypotheses.
- [ ] Additional probe hardware and home-observability domains.
