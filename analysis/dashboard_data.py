"""Pure transformations used by the interactive experiment dashboard."""

from __future__ import annotations

import math
import statistics
from collections import defaultdict
from datetime import datetime, timedelta, timezone
from typing import Any, Callable, Iterable, Optional


GROUP_DIMENSIONS = {
    "Connection label": "connection_label",
    "Target type": "target_type",
    "Band": "band",
    "BSSID": "bssid",
    "SSID": "ssid",
    "Channel": "channel_number",
    "Interface": "interface",
}


METRICS = {
    "Median RTT": {
        "key": "median_rtt_ms",
        "unit": "ms",
        "axis": "RTT (ms)",
    },
    "Average RTT": {
        "key": "average_rtt_ms",
        "unit": "ms",
        "axis": "RTT (ms)",
    },
    "P95 RTT": {
        "key": "p95_rtt_ms",
        "unit": "ms",
        "axis": "RTT (ms)",
    },
    "Maximum RTT": {
        "key": "max_rtt_ms",
        "unit": "ms",
        "axis": "RTT (ms)",
    },
    "Jitter": {
        "key": "jitter_ms",
        "unit": "ms",
        "axis": "Jitter (ms)",
    },
    "Packet loss": {
        "key": "loss_percent",
        "unit": "%",
        "axis": "Observed loss (%)",
    },
    "Signal": {
        "key": "signal_dbm",
        "unit": "dBm",
        "axis": "Signal (dBm)",
    },
    "Noise": {
        "key": "noise_dbm",
        "unit": "dBm",
        "axis": "Noise (dBm)",
    },
    "SNR": {
        "key": "snr_db",
        "unit": "dB",
        "axis": "SNR (dB)",
    },
}


TIME_BUCKETS = {
    "Raw batches": None,
    "5 minutes": timedelta(minutes=5),
    "15 minutes": timedelta(minutes=15),
    "1 hour": timedelta(hours=1),
}


def parse_batch_timestamp(batch: dict[str, Any]) -> datetime:
    parsed = datetime.fromisoformat(batch["timestamp"])
    if parsed.tzinfo is None:
        raise ValueError("batch timestamp must include a UTC offset")
    return parsed


def bucket_timestamp(timestamp: datetime, bucket: Optional[timedelta]) -> datetime:
    if bucket is None:
        return timestamp
    bucket_seconds = int(bucket.total_seconds())
    epoch_seconds = int(timestamp.timestamp())
    floored = epoch_seconds - (epoch_seconds % bucket_seconds)
    return datetime.fromtimestamp(floored, tz=timezone.utc)


def display_group_value(value: Any) -> str:
    if value is None or value == "":
        return "(unknown)"
    return str(value)


def group_key(batch: dict[str, Any], dimensions: list[str]) -> tuple[str, ...]:
    if not dimensions:
        return ("All data",)
    return tuple(
        display_group_value(batch[GROUP_DIMENSIONS[dimension]])
        for dimension in dimensions
    )


def group_label(key: tuple[str, ...]) -> str:
    return " · ".join(key)


def nearest_rank(values: list[float], percentile: float) -> Optional[float]:
    if not values:
        return None
    ordered = sorted(values)
    rank = max(1, math.ceil(percentile * len(ordered)))
    return ordered[rank - 1]


def present_numbers(
    batches: Iterable[dict[str, Any]],
    field: str,
) -> list[float]:
    return [
        float(batch[field])
        for batch in batches
        if batch.get(field) is not None
    ]


def aggregate_metric(metric_key: str, batches: list[dict[str, Any]]) -> Optional[float]:
    if metric_key == "loss_percent":
        sent = sum(batch["sent"] for batch in batches)
        received = sum(batch["received"] for batch in batches)
        return ((sent - received) / sent) * 100 if sent else None

    if metric_key in {
        "median_rtt_ms",
        "average_rtt_ms",
        "p95_rtt_ms",
        "max_rtt_ms",
    }:
        rtts = [float(rtt) for batch in batches for rtt in batch["_rtts"]]
        if not rtts:
            return None
        aggregators: dict[str, Callable[[list[float]], float]] = {
            "median_rtt_ms": statistics.median,
            "average_rtt_ms": statistics.fmean,
            "p95_rtt_ms": lambda values: nearest_rank(values, 0.95),  # type: ignore[return-value]
            "max_rtt_ms": max,
        }
        return aggregators[metric_key](rtts)

    if metric_key == "jitter_ms":
        values = present_numbers(batches, "jitter_ms")
        return statistics.median(values) if values else None

    if metric_key in {"signal_dbm", "noise_dbm", "snr_db"}:
        values = present_numbers(batches, metric_key)
        return statistics.median(values) if values else None

    raise ValueError(f"unsupported metric: {metric_key}")


def build_time_series(
    batches: Iterable[dict[str, Any]],
    metric_key: str,
    dimensions: list[str],
    bucket: Optional[timedelta],
) -> list[dict[str, Any]]:
    grouped: dict[
        tuple[datetime, tuple[str, ...]],
        list[dict[str, Any]],
    ] = defaultdict(list)

    for batch in batches:
        timestamp = bucket_timestamp(parse_batch_timestamp(batch), bucket)
        key = group_key(batch, dimensions)
        grouped[(timestamp, key)].append(batch)

    points = []
    for (timestamp, key), group in sorted(grouped.items()):
        value = aggregate_metric(metric_key, group)
        if value is None:
            continue
        points.append(
            {
                "timestamp": timestamp,
                "group": group_label(key),
                "value": value,
                "batches": len(group),
            }
        )
    return points


def apply_filters(
    batches: Iterable[dict[str, Any]],
    selected: dict[str, set[Any]],
) -> list[dict[str, Any]]:
    filtered = []
    for batch in batches:
        if all(
            not values or batch.get(field) in values
            for field, values in selected.items()
        ):
            filtered.append(batch)
    return filtered


def raw_rtt_rows(
    batches: Iterable[dict[str, Any]],
    dimensions: list[str],
) -> list[dict[str, Any]]:
    rows = []
    for batch in batches:
        label = group_label(group_key(batch, dimensions))
        for rtt in batch["_rtts"]:
            rows.append(
                {
                    "timestamp": parse_batch_timestamp(batch),
                    "group": label,
                    "rtt_ms": float(rtt),
                    "target_type": batch["target_type"],
                    "connection_label": batch["connection_label"],
                    "band": display_group_value(batch.get("band")),
                    "signal_dbm": batch.get("signal_dbm"),
                    "snr_db": batch.get("snr_db"),
                }
            )
    return rows
