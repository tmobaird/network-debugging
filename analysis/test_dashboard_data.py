import unittest
from datetime import datetime, timedelta, timezone

from analysis.dashboard_data import (
    aggregate_metric,
    apply_filters,
    bucket_timestamp,
    build_time_series,
    raw_rtt_rows,
)


def batch(
    timestamp: str,
    *,
    label: str = "direct-verizon",
    target_type: str = "gateway",
    band: str = "5GHz",
    sent: int = 5,
    received: int = 5,
    rtts: tuple[float, ...] = (2.0, 4.0),
    signal: int = -50,
    noise: int = -90,
) -> dict:
    return {
        "timestamp": timestamp,
        "connection_label": label,
        "target_type": target_type,
        "target": "192.168.1.1",
        "sent": sent,
        "received": received,
        "_rtts": list(rtts),
        "jitter_ms": abs(rtts[1] - rtts[0]) if len(rtts) >= 2 else None,
        "interface": "en0",
        "ssid": "ExampleNet",
        "bssid": "aa:bb:cc:dd:ee:ff",
        "channel_number": 64,
        "band": band,
        "signal_dbm": signal,
        "noise_dbm": noise,
        "snr_db": signal - noise,
    }


class DashboardDataTests(unittest.TestCase):
    def test_weighted_loss_uses_packet_counts(self) -> None:
        batches = [
            batch(
                "2026-07-23T18:00:00+00:00",
                sent=5,
                received=4,
            ),
            batch(
                "2026-07-23T18:01:00+00:00",
                sent=10,
                received=10,
            ),
        ]

        self.assertAlmostEqual(aggregate_metric("loss_percent", batches), 100 / 15)

    def test_time_bucket_groups_by_selected_dimension(self) -> None:
        batches = [
            batch("2026-07-23T18:01:00+00:00", band="5GHz", rtts=(2.0, 4.0)),
            batch("2026-07-23T18:04:00+00:00", band="5GHz", rtts=(6.0, 8.0)),
            batch("2026-07-23T18:02:00+00:00", band="2.4GHz", rtts=(10.0, 12.0)),
        ]

        points = build_time_series(
            batches,
            "median_rtt_ms",
            ["Band"],
            timedelta(minutes=5),
        )

        self.assertEqual(len(points), 2)
        by_group = {point["group"]: point for point in points}
        self.assertEqual(by_group["5GHz"]["value"], 5.0)
        self.assertEqual(by_group["5GHz"]["batches"], 2)
        self.assertEqual(by_group["2.4GHz"]["value"], 11.0)

    def test_bucket_timestamp_uses_utc_boundaries(self) -> None:
        timestamp = datetime(
            2026,
            7,
            23,
            18,
            7,
            31,
            tzinfo=timezone.utc,
        )

        got = bucket_timestamp(timestamp, timedelta(minutes=5))

        self.assertEqual(
            got,
            datetime(2026, 7, 23, 18, 5, tzinfo=timezone.utc),
        )

    def test_filters_and_flattens_raw_rtts(self) -> None:
        batches = [
            batch("2026-07-23T18:00:00+00:00", band="5GHz"),
            batch("2026-07-23T18:01:00+00:00", band="2.4GHz"),
        ]

        filtered = apply_filters(batches, {"band": {"5GHz"}})
        rows = raw_rtt_rows(filtered, ["Band", "Target type"])

        self.assertEqual(len(filtered), 1)
        self.assertEqual(len(rows), 2)
        self.assertEqual(rows[0]["group"], "5GHz · gateway")


if __name__ == "__main__":
    unittest.main()
