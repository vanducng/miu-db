"""Tests for the dual-read env var compatibility helper."""

from __future__ import annotations

import warnings

import pytest

from miu_db.shared.migration.env_compat import read_env, read_env_bool


def test_prefers_new_var_no_warning(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("MIU_DB_FOO", "new-value")
    monkeypatch.setenv("SQLIT_FOO", "old-value")
    with warnings.catch_warnings():
        warnings.simplefilter("error", DeprecationWarning)
        result = read_env("MIU_DB_FOO", "SQLIT_FOO")
    assert result == "new-value"


def test_falls_back_to_old_with_warning(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("MIU_DB_FOO", raising=False)
    monkeypatch.setenv("SQLIT_FOO", "old-value")
    with pytest.warns(DeprecationWarning, match="SQLIT_FOO is deprecated"):
        result = read_env("MIU_DB_FOO", "SQLIT_FOO")
    assert result == "old-value"


def test_returns_default_when_both_unset(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("MIU_DB_FOO", raising=False)
    monkeypatch.delenv("SQLIT_FOO", raising=False)
    assert read_env("MIU_DB_FOO", "SQLIT_FOO") is None
    assert read_env("MIU_DB_FOO", "SQLIT_FOO", "fallback") == "fallback"


def test_read_env_bool_new_var_true(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("MIU_DB_BAR", "1")
    assert read_env_bool("MIU_DB_BAR", "SQLIT_BAR") is True


def test_read_env_bool_old_var_true_with_warning(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("MIU_DB_BAR", raising=False)
    monkeypatch.setenv("SQLIT_BAR", "true")
    with pytest.warns(DeprecationWarning):
        result = read_env_bool("MIU_DB_BAR", "SQLIT_BAR")
    assert result is True


def test_read_env_bool_unset_returns_default(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("MIU_DB_BAR", raising=False)
    monkeypatch.delenv("SQLIT_BAR", raising=False)
    assert read_env_bool("MIU_DB_BAR", "SQLIT_BAR") is False
    assert read_env_bool("MIU_DB_BAR", "SQLIT_BAR", default=True) is True
