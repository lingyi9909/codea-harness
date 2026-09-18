#!/usr/bin/env python3
"""Permanent regressions for Codea 1.8 real-model report identity checks."""

import importlib.util
from pathlib import Path
import tempfile

HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location(
    "review180_real_model_smoke",
    HERE / "review180-real-model-smoke.py",
)
driver = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(driver)


def expect_failure(label, fn):
    try:
        fn()
    except RuntimeError:
        return
    raise AssertionError(f"{label}: expected validation failure")


def main():
    with tempfile.TemporaryDirectory(prefix="Codea 180 Report Identity & ") as raw:
        project = Path(raw) / "project"
        report = project / ".code-harness" / "runs" / "review-x" / "review.md"
        report.parent.mkdir(parents=True)
        report.write_text("durable review\n", encoding="utf-8")
        report_sha = driver.sha256_file(report)

        # Case 1: Runtime returns the normal relative path.
        driver.validate_report_identity(
            project,
            report,
            {
                "reportPath": ".code-harness/runs/review-x/review.md",
                "reportSha256": report_sha,
            },
        )

        # Case 2: Runtime returns the absolute Windows path for the same file.
        driver.validate_report_identity(
            project,
            report,
            {
                "reportPath": str(report.resolve()),
                "reportSha256": report_sha,
            },
        )

        other = project / ".code-harness" / "runs" / "review-y" / "review.md"
        other.parent.mkdir(parents=True)
        other.write_text("different review\n", encoding="utf-8")

        # Case 3: A different report path must never be accepted.
        expect_failure(
            "different report path",
            lambda: driver.validate_report_identity(
                project,
                report,
                {
                    "reportPath": str(other.resolve()),
                    "reportSha256": report_sha,
                },
            ),
        )

        # Case 4: Same path with a different SHA must never be accepted.
        expect_failure(
            "different report sha",
            lambda: driver.validate_report_identity(
                project,
                report,
                {
                    "reportPath": str(report.resolve()),
                    "reportSha256": "0" * 64,
                },
            ),
        )

    print("REVIEW180_REPORT_IDENTITY_REGRESSION PASS", flush=True)


if __name__ == "__main__":
    main()
