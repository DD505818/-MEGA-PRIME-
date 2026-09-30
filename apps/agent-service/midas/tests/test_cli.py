"""CLI smoke tests: status is read-only and safe; promote/demote round-trip."""

import json

import pytest

from midas import cli

from .conftest import DATASET_DIR, make_pass_report


def _run_db(monkeypatch, tmp_path):
    monkeypatch.setenv("MIDAS_DB_PATH", str(tmp_path / "midas.db"))
    monkeypatch.setenv("MIDAS_DATASET_DIR", DATASET_DIR)


def test_status_empty(monkeypatch, tmp_path, capsys):
    _run_db(monkeypatch, tmp_path)
    assert cli.main(["status"]) == 0
    assert json.loads(capsys.readouterr().out) == []


def test_status_unknown_strategy(monkeypatch, tmp_path, capsys):
    _run_db(monkeypatch, tmp_path)
    assert cli.main(["status", "--strategy", "Ghost"]) == 0
    out = json.loads(capsys.readouterr().out)
    assert "not promoted" in out["status"]


def test_promote_then_status(monkeypatch, tmp_path, capsys):
    _run_db(monkeypatch, tmp_path)
    rep = tmp_path / "report.json"
    rep.write_text(json.dumps(make_pass_report()))
    assert cli.main(["promote", "--strategy", "BoxTheory", "--report", str(rep)]) == 0
    capsys.readouterr()
    assert cli.main(["status", "--strategy", "BoxTheory"]) == 0
    rec = json.loads(capsys.readouterr().out)
    assert rec["status"] == "PROMOTED"


def test_promote_rejects_bad_report(monkeypatch, tmp_path, capsys):
    _run_db(monkeypatch, tmp_path)
    rep = tmp_path / "report.json"
    bad = make_pass_report(verdict="FAIL", failed_stage="costs")
    rep.write_text(json.dumps(bad))
    assert cli.main(["promote", "--strategy", "BoxTheory", "--report", str(rep)]) == 1


def test_demote_and_record_fill(monkeypatch, tmp_path, capsys):
    _run_db(monkeypatch, tmp_path)
    rep = tmp_path / "report.json"
    rep.write_text(json.dumps(make_pass_report()))
    assert cli.main(["promote", "--strategy", "Surge", "--report", str(rep)]) == 0
    capsys.readouterr()
    # 30 fills, no breach yet -> exit 0 (not demoted)
    for _ in range(29):
        assert cli.main(["record-fill", "--strategy", "Surge", "--pnl", "1.0"]) == 0
    capsys.readouterr()
    assert cli.main(["demote", "--strategy", "Surge", "--reason", "test"]) == 0
    assert "DEMOTED" in capsys.readouterr().out
