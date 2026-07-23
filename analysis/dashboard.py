#!/usr/bin/env python3

"""Interactive Streamlit dashboard for network probe experiments."""

from __future__ import annotations

import statistics
import sys
from pathlib import Path
from typing import Any

import pandas as pd
import plotly.express as px
import streamlit as st

SCRIPT_DIRECTORY = Path(__file__).resolve().parent
if str(SCRIPT_DIRECTORY) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIRECTORY))

from analyze import AnalysisError, load_batches_from_text
from dashboard_data import (
    GROUP_DIMENSIONS,
    METRICS,
    TIME_BUCKETS,
    apply_filters,
    build_time_series,
    display_group_value,
    nearest_rank,
    raw_rtt_rows,
)


st.set_page_config(
    page_title="Home Network Observatory",
    page_icon="📡",
    layout="wide",
)


def decode_upload(upload: Any) -> str:
    try:
        return upload.getvalue().decode("utf-8")
    except UnicodeDecodeError as error:
        raise AnalysisError(f"{upload.name}: file is not UTF-8") from error


def load_uploads(uploads: list[Any]) -> list[dict[str, Any]]:
    batches = []
    for upload in uploads:
        batches.extend(load_batches_from_text(decode_upload(upload), upload.name))
    return batches


def unique_values(batches: list[dict[str, Any]], field: str) -> list[Any]:
    return sorted(
        {batch.get(field) for batch in batches},
        key=lambda value: display_group_value(value),
    )


def filter_control(
    label: str,
    field: str,
    batches: list[dict[str, Any]],
) -> set[Any]:
    options = unique_values(batches, field)
    selected = st.sidebar.multiselect(
        label,
        options,
        default=options,
        format_func=display_group_value,
    )
    return set(selected)


def format_metric(value: float | None, unit: str = "") -> str:
    if value is None:
        return "—"
    return f"{value:.3f}{unit}"


def batch_table(batches: list[dict[str, Any]]) -> pd.DataFrame:
    hidden = {"_rtts"}
    rows = [
        {key: value for key, value in batch.items() if key not in hidden}
        for batch in batches
    ]
    return pd.DataFrame(rows)


st.title("Home Network Observatory")
st.caption(
    "Explore measured RTT, packet loss, and client-side Wi-Fi context. "
    "Connection labels are configured experimental context; charts show "
    "observations, not diagnoses."
)

uploads = st.sidebar.file_uploader(
    "Experiment JSONL files",
    type=["jsonl"],
    accept_multiple_files=True,
    help="Select one or more files produced by probe/experiment.sh.",
)

if not uploads:
    st.info("Choose one or more experiment JSONL files in the sidebar.")
    st.stop()

try:
    all_batches = load_uploads(uploads)
except (AnalysisError, OSError) as error:
    st.error(f"Could not load experiment data: {error}")
    st.stop()

if not all_batches:
    st.warning("The selected files contain no measurement records.")
    st.stop()

st.sidebar.header("Filters")
selected_filters = {
    "connection_label": filter_control(
        "Connection label",
        "connection_label",
        all_batches,
    ),
    "target_type": filter_control("Target type", "target_type", all_batches),
    "band": filter_control("Band", "band", all_batches),
    "bssid": filter_control("BSSID", "bssid", all_batches),
}

filtered_batches = apply_filters(all_batches, selected_filters)
if not filtered_batches:
    st.warning("No records match the current filters.")
    st.stop()

timestamps = sorted(
    pd.Timestamp(batch["timestamp"]).to_pydatetime()
    for batch in filtered_batches
)
if timestamps[0] != timestamps[-1]:
    selected_time_range = st.sidebar.slider(
        "Time range",
        min_value=timestamps[0],
        max_value=timestamps[-1],
        value=(timestamps[0], timestamps[-1]),
        format="YYYY-MM-DD HH:mm",
    )
    filtered_batches = [
        batch
        for batch in filtered_batches
        if selected_time_range[0]
        <= pd.Timestamp(batch["timestamp"]).to_pydatetime()
        <= selected_time_range[1]
    ]

st.sidebar.header("Chart")
metric_name = st.sidebar.selectbox("Metric", list(METRICS), index=0)
group_dimensions = st.sidebar.multiselect(
    "Group by",
    list(GROUP_DIMENSIONS),
    default=["Target type"],
    help="Each unique combination becomes a separate chart trace.",
)
bucket_name = st.sidebar.selectbox("Time bucket", list(TIME_BUCKETS), index=0)

metric = METRICS[metric_name]
time_points = build_time_series(
    filtered_batches,
    metric["key"],
    group_dimensions,
    TIME_BUCKETS[bucket_name],
)

total_sent = sum(batch["sent"] for batch in filtered_batches)
total_received = sum(batch["received"] for batch in filtered_batches)
loss_percent = (
    ((total_sent - total_received) / total_sent) * 100
    if total_sent
    else None
)
operational_errors = sum(bool(batch["error"]) for batch in filtered_batches)
all_rtts = [
    float(rtt)
    for batch in filtered_batches
    for rtt in batch["_rtts"]
]

kpis = st.columns(5)
kpis[0].metric("Batches", f"{len(filtered_batches):,}")
kpis[1].metric("Operational errors", f"{operational_errors:,}")
kpis[2].metric("Received", f"{total_received:,}/{total_sent:,}")
kpis[3].metric("Observed loss", format_metric(loss_percent, "%"))
kpis[4].metric(
    "Median / P95 RTT",
    (
        f"{statistics.median(all_rtts):.3f} / "
        f"{nearest_rank(all_rtts, 0.95):.3f} ms"
        if all_rtts
        else "—"
    ),
)

overview_tab, distribution_tab, radio_tab, data_tab = st.tabs(
    ["Time series", "RTT distributions", "Radio relationship", "Batch data"]
)

with overview_tab:
    st.subheader(f"{metric_name} over time")
    if not time_points:
        st.info(f"No {metric_name.lower()} values are available for this selection.")
    else:
        time_frame = pd.DataFrame(time_points)
        figure = px.line(
            time_frame,
            x="timestamp",
            y="value",
            color="group",
            markers=TIME_BUCKETS[bucket_name] is None,
            custom_data=["batches"],
            labels={
                "timestamp": "Time",
                "value": metric["axis"],
                "group": "Group",
            },
        )
        figure.update_traces(
            connectgaps=False,
            hovertemplate=(
                "Time=%{x}<br>"
                f"{metric_name}=%{{y:.3f}} {metric['unit']}<br>"
                "Batches=%{customdata[0]}<extra>%{fullData.name}</extra>"
            ),
        )
        figure.update_layout(
            legend_title_text=" · ".join(group_dimensions) or "Series",
            hovermode="x unified",
        )
        st.plotly_chart(figure, width="stretch")

with distribution_tab:
    st.subheader("Successful RTT distributions")
    rtt_rows = raw_rtt_rows(filtered_batches, group_dimensions)
    if not rtt_rows:
        st.info("No successful RTT samples are available for this selection.")
    else:
        rtt_frame = pd.DataFrame(rtt_rows)
        distribution = px.box(
            rtt_frame,
            x="group",
            y="rtt_ms",
            color="group",
            points="outliers",
            labels={
                "group": "Group",
                "rtt_ms": "RTT (ms)",
            },
        )
        distribution.update_layout(showlegend=False)
        st.plotly_chart(distribution, width="stretch")

with radio_tab:
    st.subheader("RTT versus client-side radio context")
    radio_dimension = st.radio(
        "X-axis",
        ["Signal (dBm)", "SNR (dB)"],
        horizontal=True,
    )
    radio_key = "signal_dbm" if radio_dimension == "Signal (dBm)" else "snr_db"
    radio_rows = [
        row
        for row in raw_rtt_rows(filtered_batches, group_dimensions)
        if row[radio_key] is not None
    ]
    if not radio_rows:
        st.info(f"No {radio_dimension} observations are available.")
    else:
        radio_frame = pd.DataFrame(radio_rows)
        scatter = px.scatter(
            radio_frame,
            x=radio_key,
            y="rtt_ms",
            color="group",
            hover_data=[
                "timestamp",
                "target_type",
                "connection_label",
                "band",
            ],
            labels={
                radio_key: radio_dimension,
                "rtt_ms": "RTT (ms)",
                "group": "Group",
            },
        )
        st.plotly_chart(scatter, width="stretch")
        st.caption(
            "Signal and SNR describe the probe's client-facing Wi-Fi link. "
            "They do not directly observe a repeater's wireless backhaul."
        )

with data_tab:
    st.subheader("Filtered batch measurements")
    st.dataframe(
        batch_table(filtered_batches),
        width="stretch",
        hide_index=True,
    )
