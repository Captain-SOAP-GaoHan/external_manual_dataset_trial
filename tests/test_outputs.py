#!/usr/bin/env python3
"""
Verifier for dbmate status dashboard feature.
Tests are organized as:
  - P*: Problem reproduction tests (should fail on init, pass on final)
  - F*: Feature acceptance tests (should fail on init, pass on final)
  - G*: Non-regression tests (should pass on both init and final)
"""

import subprocess
import json
import os
import sys
import re

ISSUE_DIR = os.environ.get(
    "ISSUE_DIR",
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
)
WORKSPACE = os.path.join(ISSUE_DIR, "workspace")
DBMATE = os.path.join(WORKSPACE, "dbmate")
MIGRATIONS_DIR = os.path.join(WORKSPACE, "testdata", "db", "migrations")
DB_URL = "sqlite:verify_test.db"
DB_PATH = os.path.join(WORKSPACE, "verify_test.db")


def run_dbmate(*args, expect_error=False):
    """Run dbmate command and return (stdout, stderr, returncode)."""
    cmd = [DBMATE, "--url", DB_URL, "--migrations-dir", MIGRATIONS_DIR] + list(args)
    result = subprocess.run(
        cmd, capture_output=True, text=True, cwd=WORKSPACE, timeout=60
    )
    if not expect_error and result.returncode not in (0, 1):
        print(f"Command failed: {' '.join(cmd)}")
        print(f"stderr: {result.stderr}")
    return result.stdout, result.stderr, result.returncode


def cleanup_db():
    """Remove test database file."""
    try:
        os.remove(DB_PATH)
    except FileNotFoundError:
        pass


# ──────────────────────────────────────────
# P*: Problem reproduction tests
# ──────────────────────────────────────────


def test_P1_status_no_counts():
    """P1: In init state, status command does not show Applied/Pending count summary."""
    cleanup_db()
    stdout, _, _ = run_dbmate("status")
    # In init (old status), there's no "Applied:" and "Pending:" summary line
    has_summary = bool(re.search(r"Applied:\s+\d", stdout))
    if has_summary:
        print("FAIL P1: Old status should not show Applied/Pending count summary")
        return False
    print("PASS P1: Old status does not show Applied/Pending count summary")
    return True


def test_P2_status_no_applied_at():
    """P2: In init state, status does not show applied_at timestamps."""
    cleanup_db()
    run_dbmate("up")
    stdout, _, _ = run_dbmate("status")
    has_applied_at = bool(re.search(r"Applied\s*At", stdout, re.IGNORECASE))
    if has_applied_at:
        print("FAIL P2: Old status should not show Applied At column")
        return False
    print("PASS P2: Old status does not show Applied At column")
    return True


def test_P3_status_no_table_format():
    """P3: In init state, --format table is not supported."""
    cleanup_db()
    stdout, stderr, rc = run_dbmate("status", "--format", "table")
    # Old status doesn't support --format flag or doesn't produce bordered table
    has_bordered_table = bool(re.search(r"\+[-]+\+", stdout))
    if has_bordered_table:
        print("FAIL P3: Old status should not produce bordered table format")
        return False
    print("PASS P3: Old status does not produce bordered table format")
    return True


def test_P4_status_no_json_format():
    """P4: In init state, --format json is not supported or doesn't produce valid JSON."""
    cleanup_db()
    stdout, stderr, rc = run_dbmate("status", "--format", "json")
    try:
        data = json.loads(stdout)
        has_required_fields = all(k in data for k in ["total", "applied", "pending", "migrations"])
        if has_required_fields:
            print("FAIL P4: Old status should not produce valid JSON with required fields")
            return False
    except (json.JSONDecodeError, ValueError):
        pass
    print("PASS P4: Old status does not produce valid JSON output")
    return True


# ──────────────────────────────────────────
# F*: Feature acceptance tests
# ──────────────────────────────────────────


def test_F1_dashboard_counts():
    """F1: Dashboard format shows Applied/Pending/Total counts and progress bar."""
    cleanup_db()
    # Apply migrations first
    run_dbmate("up")
    stdout, _, _ = run_dbmate("status")
    has_total = bool(re.search(r"Total\s+Migrations:\s+\d", stdout))
    has_applied = bool(re.search(r"Applied:\s+\d", stdout))
    has_pending = bool(re.search(r"Pending:\s+\d", stdout))
    has_progress = bool(re.search(r"Progress:\s+\[", stdout))
    if not (has_total and has_applied and has_pending and has_progress):
        print(f"FAIL F1: Dashboard missing counts/progress. total={has_total}, applied={has_applied}, pending={has_pending}, progress={has_progress}")
        return False
    print("PASS F1: Dashboard shows counts and progress bar")
    return True


def test_F2_migration_status_list():
    """F2: Status lists all migrations with applied/pending status markers."""
    cleanup_db()
    run_dbmate("up")
    stdout, _, _ = run_dbmate("status")
    # Should have applied marker
    has_applied_marker = bool(re.search(r"\[X\]", stdout)) or bool(re.search(r"applied", stdout, re.IGNORECASE))
    if not has_applied_marker:
        print("FAIL F2: Status does not show migration status markers")
        return False
    # Rollback one and check pending marker
    run_dbmate("rollback")
    stdout, _, _ = run_dbmate("status")
    has_pending_marker = bool(re.search(r"\[ \]", stdout)) or bool(re.search(r"pending", stdout, re.IGNORECASE))
    if not has_pending_marker:
        print("FAIL F2: Status does not show pending status markers after rollback")
        return False
    print("PASS F2: Status shows migration status markers (applied/pending)")
    return True


def test_F3_applied_at_timestamp():
    """F3: Applied migrations show execution timestamps."""
    cleanup_db()
    run_dbmate("up")
    stdout, _, _ = run_dbmate("status")
    # Should contain a timestamp-like pattern (YYYY-MM-DD HH:MM:SS)
    has_timestamp = bool(re.search(r"\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}", stdout))
    if not has_timestamp:
        print("FAIL F3: Status does not show applied_at timestamps")
        return False
    print("PASS F3: Status shows applied_at timestamps for applied migrations")
    return True


def test_F4_table_format():
    """F4: --format table produces bordered table with alignment."""
    cleanup_db()
    run_dbmate("up")
    stdout, _, _ = run_dbmate("status", "--format", "table")
    has_borders = bool(re.search(r"\+[-]+\+", stdout))
    has_pipe_separator = bool(re.search(r"\|", stdout))
    has_summary = bool(re.search(r"Total:\s+\d", stdout))
    if not (has_borders and has_pipe_separator and has_summary):
        print(f"FAIL F4: Table format missing borders/separator/summary. borders={has_borders}, pipe={has_pipe_separator}, summary={has_summary}")
        return False
    print("PASS F4: --format table produces bordered table with summary")
    return True


def test_F5_json_format():
    """F5: --format json produces valid JSON with required fields."""
    cleanup_db()
    run_dbmate("up")
    stdout, _, _ = run_dbmate("status", "--format", "json")
    try:
        data = json.loads(stdout)
    except (json.JSONDecodeError, ValueError) as e:
        print(f"FAIL F5: --format json output is not valid JSON: {e}")
        return False
    required_fields = ["total", "applied", "pending", "migrations"]
    for field in required_fields:
        if field not in data:
            print(f"FAIL F5: JSON missing required field '{field}'")
            return False
    if not isinstance(data["migrations"], list):
        print("FAIL F5: JSON 'migrations' is not a list")
        return False
    if len(data["migrations"]) == 0:
        print("FAIL F5: JSON 'migrations' list is empty")
        return False
    # Check migration fields
    m = data["migrations"][0]
    for field in ["version", "file_name", "applied", "applied_at"]:
        if field not in m:
            print(f"FAIL F5: Migration object missing field '{field}'")
            return False
    # Applied migrations should have non-null applied_at
    if m.get("applied") and m.get("applied_at") is None:
        print("FAIL F5: Applied migration has null applied_at")
        return False
    print("PASS F5: --format json produces valid JSON with all required fields")
    return True


# ──────────────────────────────────────────
# G*: Non-regression tests
# ──────────────────────────────────────────


def test_G1_migrate_still_works():
    """G1: Basic migrate/rollback still works after changes."""
    cleanup_db()
    _, _, rc = run_dbmate("up")
    if rc not in (0, 1):
        print("FAIL G1: Migrate (up) failed")
        return False
    _, _, rc = run_dbmate("rollback")
    if rc not in (0, 1):
        print("FAIL G1: Rollback failed")
        return False
    print("PASS G1: Migrate and rollback still work")
    return True


def test_G2_status_exit_code():
    """G2: Status --exit-code returns 1 when pending migrations exist."""
    cleanup_db()
    _, _, rc = run_dbmate("status", "--exit-code")
    if rc != 1:
        print(f"FAIL G2: Expected exit code 1 with pending migrations, got {rc}")
        return False
    print("PASS G2: Status --exit-code returns 1 for pending migrations")
    return True


# ──────────────────────────────────────────
# Main
# ──────────────────────────────────────────

if __name__ == "__main__":
    # Check if we should run P* tests (init state) or F*/G* tests (final state)
    mode = os.environ.get("VERIFY_MODE", "final").lower()

    print(f"\n{'='*60}")
    print(f"Verification mode: {mode}")
    print(f"{'='*60}\n")

    results = {}

    if mode == "init":
        # Only run problem reproduction tests for init
        for name, func in [
            ("P1", test_P1_status_no_counts),
            ("P2", test_P2_status_no_applied_at),
            ("P3", test_P3_status_no_table_format),
            ("P4", test_P4_status_no_json_format),
        ]:
            try:
                results[name] = func()
            except Exception as e:
                print(f"ERROR {name}: {e}")
                results[name] = False
    else:
        # Run feature acceptance + non-regression tests for final
        for name, func in [
            ("F1", test_F1_dashboard_counts),
            ("F2", test_F2_migration_status_list),
            ("F3", test_F3_applied_at_timestamp),
            ("F4", test_F4_table_format),
            ("F5", test_F5_json_format),
            ("G1", test_G1_migrate_still_works),
            ("G2", test_G2_status_exit_code),
        ]:
            try:
                results[name] = func()
            except Exception as e:
                print(f"ERROR {name}: {e}")
                results[name] = False
        cleanup_db()

    print(f"\n{'='*60}")
    print("Summary:")
    for name, passed in results.items():
        status = "PASS" if passed else "FAIL"
        print(f"  {name}: {status}")
    print(f"{'='*60}\n")

    if not all(results.values()):
        sys.exit(1)
    sys.exit(0)
