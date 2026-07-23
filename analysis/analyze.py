#!/usr/bin/env python3

"""Validate and summarize Home Network Observatory JSONL experiments."""

from __future__ import annotations

import argparse
import csv
import io
import json
import math
import re
import statistics
import sys
from collections import defaultdict
from datetime import datetime
from pathlib import Path
from typing import Any, Iterable, Optional, Union


class AnalysisError(ValueError):
    """Raised when an experiment record violates the expected schema."""


RFC3339_PATTERN = re.compile(
    r"^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})"
    r"(?:\.(\d{1,9}))?"
    r"(Z|[+-]\d{2}:\d{2})$"
)


def require(condition: bool, source: str, message: str) -> None:
    if not condition:
        raise AnalysisError(f"{source}: {message}")


def parse_timestamp(value: Any, source: str) -> datetime:
    require(isinstance(value, str), source, "timestamp must be a string")
    match = RFC3339_PATTERN.fullmatch(value)
    require(match is not None, source, f"invalid timestamp {value!r}")

    base, fraction, offset = match.groups()
    normalized_offset = "+00:00" if offset == "Z" else offset

    # Go's RFC3339Nano formatter removes trailing zeros and may therefore emit
    # any fractional precision from one through nine digits. Apple Python 3.9's
    # fromisoformat accepts only three or six digits. Python datetime stores
    # microseconds, so pad short fractions and truncate sub-microsecond digits.
    normalized_fraction = ""
    if fraction is not None:
        normalized_fraction = "." + fraction[:6].ljust(6, "0")

    normalized = base + normalized_fraction + normalized_offset
    try:
        parsed = datetime.fromisoformat(normalized)
    except ValueError as error:
        raise AnalysisError(f"{source}: invalid timestamp {value!r}") from error
    require(parsed.tzinfo is not None, source, "timestamp must include a UTC offset")
    return parsed


def numeric_values(values: Iterable[Any], source: str, field: str) -> list[float]:
    result: list[float] = []
    for value in values:
        require(
            isinstance(value, (int, float)) and not isinstance(value, bool),
            source,
            f"{field} values must be numbers",
        )
        number = float(value)
        require(math.isfinite(number) and number >= 0, source, f"invalid {field} value")
        result.append(number)
    return result


def mean_consecutive_difference(values: list[float]) -> Optional[float]:
    if len(values) < 2:
        return None
    differences = [
        abs(current - previous)
        for previous, current in zip(values, values[1:])
    ]
    return statistics.fmean(differences)


def derive_batch(record: dict[str, Any], source: str) -> dict[str, Any]:
    require(record.get("schemaVersion") == 1, source, "unsupported schemaVersion")

    timestamp = parse_timestamp(record.get("timestamp"), source)
    target = record.get("target")
    target_type = record.get("targetType")
    require(isinstance(target, str) and target, source, "target must be non-empty")
    require(
        target_type in {"gateway", "internet"},
        source,
        "targetType must be gateway or internet",
    )

    sent = record.get("sent")
    received = record.get("received")
    require(isinstance(sent, int) and not isinstance(sent, bool), source, "sent must be an integer")
    require(
        isinstance(received, int) and not isinstance(received, bool),
        source,
        "received must be an integer",
    )
    require(sent >= 0, source, "sent must not be negative")
    require(0 <= received <= sent, source, "received must be between zero and sent")

    samples = record.get("samples")
    require(isinstance(samples, list), source, "samples must be an array")
    rtts = numeric_values(
        (sample.get("rttMs") for sample in samples if isinstance(sample, dict)),
        source,
        "rttMs",
    )
    require(len(rtts) == len(samples), source, "every sample must be an object with rttMs")
    require(
        len(rtts) == received,
        source,
        f"received is {received}, but {len(rtts)} RTT samples were recorded",
    )

    network = record.get("network")
    require(isinstance(network, dict), source, "network must be an object")

    signal = network.get("signalDBM")
    noise = network.get("noiseDBM")
    snr = signal - noise if isinstance(signal, int) and isinstance(noise, int) else None
    error = record.get("error")
    require(error is None or isinstance(error, str), source, "error must be a string")

    return {
        "source": source,
        "timestamp": timestamp.isoformat(),
        "connection_label": network.get("connectionLabel", ""),
        "target": target,
        "target_type": target_type,
        "sent": sent,
        "received": received,
        "loss_ratio": (sent - received) / sent if sent else None,
        "rtt_sample_count": len(rtts),
        "min_rtt_ms": min(rtts) if rtts else None,
        "avg_rtt_ms": statistics.fmean(rtts) if rtts else None,
        "median_rtt_ms": statistics.median(rtts) if rtts else None,
        "p95_rtt_ms": nearest_rank_percentile(rtts, 0.95),
        "max_rtt_ms": max(rtts) if rtts else None,
        "jitter_ms": mean_consecutive_difference(rtts),
        "interface": network.get("interface"),
        "ssid": network.get("ssid"),
        "bssid": network.get("bssid"),
        "channel_number": network.get("channelNumber"),
        "band": network.get("band"),
        "channel_width_mhz": network.get("channelWidthMHz"),
        "signal_dbm": signal,
        "noise_dbm": noise,
        "snr_db": snr,
        "error": error,
        "_rtts": rtts,
    }


def load_batches_from_stream(
    input_file: Iterable[str],
    source_name: str,
) -> list[dict[str, Any]]:
    batches: list[dict[str, Any]] = []
    for line_number, line in enumerate(input_file, start=1):
        if not line.strip():
            continue
        source = f"{source_name}:{line_number}"
        try:
            record = json.loads(line)
        except json.JSONDecodeError as error:
            raise AnalysisError(f"{source}: invalid JSON: {error.msg}") from error
        require(isinstance(record, dict), source, "record must be a JSON object")
        batches.append(derive_batch(record, source))
    return batches


def load_batches_from_text(text: str, source_name: str) -> list[dict[str, Any]]:
    return load_batches_from_stream(io.StringIO(text), source_name)


def load_batches(paths: Iterable[Path]) -> list[dict[str, Any]]:
    batches: list[dict[str, Any]] = []
    for path in paths:
        with path.open(encoding="utf-8") as input_file:
            batches.extend(load_batches_from_stream(input_file, str(path)))
    return batches


def nearest_rank_percentile(
    values: list[float],
    percentile: float,
) -> Optional[float]:
    if not values:
        return None
    ordered = sorted(values)
    rank = max(1, math.ceil(percentile * len(ordered)))
    return ordered[rank - 1]


def optional_median(
    values: Iterable[Optional[Union[float, int]]],
) -> Optional[float]:
    present = [float(value) for value in values if value is not None]
    return statistics.median(present) if present else None


def summarize(batches: Iterable[dict[str, Any]]) -> list[dict[str, Any]]:
    groups: dict[tuple[str, str, str], list[dict[str, Any]]] = defaultdict(list)
    for batch in batches:
        key = (
            batch["connection_label"],
            batch["target_type"],
            batch["target"],
        )
        groups[key].append(batch)

    summaries: list[dict[str, Any]] = []
    for (label, target_type, target), group in sorted(groups.items()):
        all_rtts = [rtt for batch in group for rtt in batch["_rtts"]]
        total_sent = sum(batch["sent"] for batch in group)
        total_received = sum(batch["received"] for batch in group)
        operational_errors = sum(bool(batch["error"]) for batch in group)

        summaries.append(
            {
                "connectionLabel": label,
                "targetType": target_type,
                "target": target,
                "batches": len(group),
                "operationalErrors": operational_errors,
                "sent": total_sent,
                "received": total_received,
                "lossRatio": (
                    (total_sent - total_received) / total_sent
                    if total_sent
                    else None
                ),
                "rttSamples": len(all_rtts),
                "minRttMs": min(all_rtts) if all_rtts else None,
                "avgRttMs": statistics.fmean(all_rtts) if all_rtts else None,
                "medianRttMs": statistics.median(all_rtts) if all_rtts else None,
                "p95RttMs": nearest_rank_percentile(all_rtts, 0.95),
                "maxRttMs": max(all_rtts) if all_rtts else None,
                "medianBatchJitterMs": optional_median(
                    batch["jitter_ms"] for batch in group
                ),
                "medianSignalDbm": optional_median(
                    batch["signal_dbm"] for batch in group
                ),
                "medianNoiseDbm": optional_median(
                    batch["noise_dbm"] for batch in group
                ),
                "medianSnrDb": optional_median(batch["snr_db"] for batch in group),
            }
        )
    return summaries


def write_batches_csv(path: Path, batches: list[dict[str, Any]]) -> None:
    fields = [field for field in batches[0] if not field.startswith("_")]
    with path.open("w", encoding="utf-8", newline="") as output_file:
        writer = csv.DictWriter(output_file, fieldnames=fields)
        writer.writeheader()
        for batch in batches:
            writer.writerow({field: batch[field] for field in fields})


def format_decimal(value: Any, unit: str = "") -> str:
    if value is None:
        return "-"
    return f"{float(value):.3f}{unit}"


def format_table(headers: list[str], rows: list[list[str]]) -> list[str]:
    widths = [
        max(len(headers[index]), *(len(row[index]) for row in rows))
        for index in range(len(headers))
    ]

    def render(row: list[str]) -> str:
        return "  ".join(
            value.ljust(widths[index])
            for index, value in enumerate(row)
        ).rstrip()

    return [
        render(headers),
        render(["-" * width for width in widths]),
        *(render(row) for row in rows),
    ]


def print_summaries(
    summaries: list[dict[str, Any]],
    output: Any = None,
) -> None:
    if output is None:
        output = sys.stdout

    by_label: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for summary in summaries:
        by_label[summary["connectionLabel"] or "(unlabeled)"].append(summary)

    headers = [
        "Target",
        "Address",
        "Batches",
        "Errors",
        "Received",
        "Loss",
        "Median RTT",
        "P95 RTT",
        "Jitter",
        "Signal",
        "SNR",
    ]

    for group_index, (label, group) in enumerate(sorted(by_label.items())):
        if group_index:
            print(file=output)
        print(f"Connection: {label}", file=output)

        rows = []
        for summary in group:
            loss = summary["lossRatio"]
            rows.append(
                [
                    summary["targetType"],
                    summary["target"],
                    str(summary["batches"]),
                    str(summary["operationalErrors"]),
                    f'{summary["received"]}/{summary["sent"]}',
                    f"{loss * 100:.2f}%" if loss is not None else "-",
                    format_decimal(summary["medianRttMs"], " ms"),
                    format_decimal(summary["p95RttMs"], " ms"),
                    format_decimal(summary["medianBatchJitterMs"], " ms"),
                    format_decimal(summary["medianSignalDbm"], " dBm"),
                    format_decimal(summary["medianSnrDb"], " dB"),
                ]
            )

        for line in format_table(headers, rows):
            print(line, file=output)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Validate and summarize network probe JSONL experiments."
    )
    parser.add_argument("jsonl", nargs="+", type=Path, help="JSONL experiment files")
    parser.add_argument(
        "--batches-csv",
        type=Path,
        help="write flattened per-batch metrics to this CSV file",
    )
    parser.add_argument(
        "--summary-json",
        type=Path,
        help="write grouped experiment summaries to this JSON file",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        batches = load_batches(args.jsonl)
        require(bool(batches), "input", "no JSONL records found")
        summaries = summarize(batches)

        if args.batches_csv:
            write_batches_csv(args.batches_csv, batches)
        if args.summary_json:
            args.summary_json.write_text(
                json.dumps(summaries, indent=2) + "\n",
                encoding="utf-8",
            )

        print_summaries(summaries)
        return 0
    except (AnalysisError, OSError) as error:
        print(f"analysis failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
