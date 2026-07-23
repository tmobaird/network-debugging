# Experiment analysis

This directory contains reproducible analysis for JSONL files produced by the
network probe. Raw experiment files should be treated as immutable inputs.
Derived CSV files, summaries, and charts can always be regenerated.

## Start with a smoke test

Collect a short experiment before committing to a multi-hour run:

```sh
cd probe
./experiment.sh direct-verizon 5m 1m
```

Validate and summarize it from the repository root:

```sh
python3 analysis/analyze.py \
  "$HOME/network-observatory-data/direct-verizon_TIMESTAMP.jsonl"
```

Multiple scenarios can be analyzed together:

```sh
python3 analysis/analyze.py \
  "$HOME/network-observatory-data/direct-verizon_TIMESTAMP.jsonl" \
  "$HOME/network-observatory-data/tp-link-repeater_TIMESTAMP.jsonl" \
  --batches-csv /tmp/network-batches.csv \
  --summary-json /tmp/network-summary.json
```

The analyzer uses only the Python standard library. Run its tests with:

```sh
python3 -m unittest discover -s analysis -p 'test_*.py'
```

## Interactive dashboard

The Streamlit dashboard supports:

- Multiple JSONL files
- Connection-label, target, band, BSSID, and time-range filters
- Dynamic grouping by one or more dimensions
- Raw, 5-minute, 15-minute, and hourly time buckets
- RTT, jitter, loss, signal, noise, and SNR metrics
- Interactive time-series, distribution, and radio-context charts
- A filtered batch table

Install its dependencies into the isolated Python 3.10+ environment:

```sh
./analysis/setup.sh
```

Launch the local dashboard:

```sh
./analysis/dashboard.sh
```

The browser interface will prompt for one or more JSONL experiment files.
Changing the metric, grouping dimensions, bucket, or filters rebuilds the
chart from the validated raw records.

The CLI analyzer deliberately remains standard-library-only. Streamlit,
Pandas, and Plotly are isolated to the optional dashboard environment.

## Analysis dimensions

### Experimental identity

- Source file
- Connection label (`direct-verizon` or `tp-link-repeater`)
- Eventually: probe ID and experiment ID

The connection label is configured context. It is not observed proof of the
physical path.

### Time

- UTC batch timestamp
- Expected cadence
- Missing intervals and measurement gaps
- Time-of-day effects

Gateway and internet batches are sequential, so their timestamps will differ.
They can be correlated in time but are not simultaneous observations.

### Target role

- Gateway
- Internet (`1.1.1.1`)
- Concrete target address

Gateway behavior describes the complete local path to the router. Internet
behavior includes that local path plus the ISP and external route.

### Reliability

- Sent and received counts
- Aggregate observed loss
- Operational errors
- Consecutive loss events

Packet loss and operational measurement errors must remain separate.
Aggregate loss is calculated from total counts, not by averaging batch loss
percentages.

### Latency and variation

- Raw successful RTT samples
- Minimum, mean, median, maximum, and 95th percentile RTT
- Per-batch jitter

Jitter is the mean absolute difference between consecutive successful RTT
samples. It is unavailable when fewer than two successful replies exist.
RTT summaries exclude requests that received no reply.

### Wi-Fi attachment and radio context

- Interface
- SSID and BSSID
- Channel, band, and channel width
- Signal and noise in dBm
- Derived SNR in dB (`signalDBM - noiseDBM`)

These fields describe the client-facing Wi-Fi link. They do not directly
measure the TP-Link repeater's hidden wireless backhaul.

### Comparative questions

- Does gateway RTT or loss worsen through the TP-Link path?
- Does internet degradation occur while gateway measurements stay healthy?
- Do latency or loss changes correlate with signal, noise, or SNR?
- Do BSSID or channel changes coincide with discontinuities?
- Are differences persistent, intermittent, or limited to particular times?

These comparisons provide evidence. They do not by themselves establish root
cause.

## Current output

The script prints one grouped row for each connection-label/target combination.
It can additionally produce:

- A flattened per-batch CSV for inspection or plotting
- A machine-readable grouped JSON summary

The interactive dashboard is the initial plotting surface. Its flexible
grouping is intended to reveal which views remain useful across experiments;
those stable views can later be reproduced in Grafana.
