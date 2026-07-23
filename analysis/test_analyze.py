import json
import io
import tempfile
import unittest
from pathlib import Path

from analysis.analyze import (
    AnalysisError,
    load_batches,
    parse_timestamp,
    print_summaries,
    summarize,
)


def record(
    target_type: str = "gateway",
    target: str = "192.168.1.1",
    sent: int = 3,
    received: int = 2,
    rtts: tuple[float, ...] = (2.0, 4.0),
) -> dict:
    return {
        "schemaVersion": 1,
        "timestamp": "2026-07-23T18:00:00Z",
        "target": target,
        "targetType": target_type,
        "sent": sent,
        "received": received,
        "samples": [
            {"icmpSequence": sequence, "rttMs": rtt}
            for sequence, rtt in enumerate(rtts)
        ],
        "network": {
            "interface": "en0",
            "connectionLabel": "direct-verizon",
            "signalDBM": -40,
            "noiseDBM": -90,
        },
    }


class AnalysisTests(unittest.TestCase):
    def write_records(self, records: list[dict]) -> Path:
        temporary = tempfile.NamedTemporaryFile(
            mode="w",
            encoding="utf-8",
            suffix=".jsonl",
            delete=False,
        )
        with temporary:
            for item in records:
                temporary.write(json.dumps(item) + "\n")
        self.addCleanup(Path(temporary.name).unlink)
        return Path(temporary.name)

    def test_derives_batch_metrics(self) -> None:
        batch = load_batches([self.write_records([record()])])[0]

        self.assertAlmostEqual(batch["loss_ratio"], 1 / 3)
        self.assertEqual(batch["avg_rtt_ms"], 3.0)
        self.assertEqual(batch["median_rtt_ms"], 3.0)
        self.assertEqual(batch["jitter_ms"], 2.0)
        self.assertEqual(batch["snr_db"], 50)

    def test_parses_go_rfc3339_fractional_precision(self) -> None:
        five_digits = parse_timestamp(
            "2026-07-23T18:01:35.31261Z",
            "test",
        )
        nanoseconds = parse_timestamp(
            "2026-07-23T18:01:35.312610789Z",
            "test",
        )

        self.assertEqual(five_digits.microsecond, 312610)
        self.assertEqual(nanoseconds.microsecond, 312610)
        self.assertEqual(five_digits.utcoffset().total_seconds(), 0)

    def test_summarizes_by_connection_and_target(self) -> None:
        records = [
            record(rtts=(2.0, 4.0)),
            record(sent=2, received=2, rtts=(6.0, 8.0)),
            record(
                target_type="internet",
                target="1.1.1.1",
                sent=2,
                received=2,
                rtts=(10.0, 12.0),
            ),
        ]

        summaries = summarize(load_batches([self.write_records(records)]))

        self.assertEqual(len(summaries), 2)
        gateway = next(item for item in summaries if item["targetType"] == "gateway")
        self.assertEqual(gateway["sent"], 5)
        self.assertEqual(gateway["received"], 4)
        self.assertEqual(gateway["lossRatio"], 0.2)
        self.assertEqual(gateway["medianRttMs"], 5.0)
        self.assertEqual(gateway["medianSnrDb"], 50.0)

        output = io.StringIO()
        print_summaries(summaries, output)
        rendered = output.getvalue()
        self.assertIn("Connection: direct-verizon", rendered)
        self.assertIn("Received", rendered)
        self.assertIn("4/5", rendered)
        self.assertIn("20.00%", rendered)
        self.assertIn("5.000 ms", rendered)
        self.assertIn("50.000 dB", rendered)

    def test_rejects_sample_count_mismatch(self) -> None:
        malformed = record(received=2, rtts=(2.0,))

        with self.assertRaisesRegex(AnalysisError, "RTT samples"):
            load_batches([self.write_records([malformed])])

    def test_rejects_invalid_json(self) -> None:
        path = self.write_records([])
        path.write_text("not JSON\n", encoding="utf-8")

        with self.assertRaisesRegex(AnalysisError, "invalid JSON"):
            load_batches([path])


if __name__ == "__main__":
    unittest.main()
