"""Tests for miu-db config directory resolution."""

from __future__ import annotations

import importlib
from pathlib import Path

import pytest


@pytest.fixture(autouse=True)
def _restore_store_module_after_test() -> None:
    yield
    from miu_db.shared.core import store

    importlib.reload(store)


def _isolated_home(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> Path:
    """Point `Path.home()` at a tmpdir and clear env vars that matter."""
    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.delenv("MIU_DB_CONFIG_DIR", raising=False)
    monkeypatch.delenv("XDG_CONFIG_HOME", raising=False)
    return tmp_path


def _resolve() -> Path:
    # Re-import so _resolve_config_dir runs against the active env.
    from miu_db.shared.core import store

    importlib.reload(store)
    return store.CONFIG_DIR


def test_miu_db_config_dir_env_wins(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    _isolated_home(monkeypatch, tmp_path)
    override = tmp_path / "custom-config"
    monkeypatch.setenv("MIU_DB_CONFIG_DIR", str(override))

    resolved = _resolve()

    assert resolved == override


def test_legacy_config_dir_env_is_ignored(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    home = _isolated_home(monkeypatch, tmp_path)
    monkeypatch.setenv("SQLIT_CONFIG_DIR", str(tmp_path / "legacy-config"))

    resolved = _resolve()

    assert resolved == home / ".config" / "miu" / "db"


def test_xdg_config_home_is_respected(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    _isolated_home(monkeypatch, tmp_path)
    xdg = tmp_path / "xdg-conf"
    monkeypatch.setenv("XDG_CONFIG_HOME", str(xdg))

    resolved = _resolve()

    assert resolved == xdg / "miu" / "db"


def test_default_path_without_xdg(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    home = _isolated_home(monkeypatch, tmp_path)

    resolved = _resolve()

    assert resolved == home / ".config" / "miu" / "db"


def test_existing_default_path_is_not_modified(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    home = _isolated_home(monkeypatch, tmp_path)
    new_path = home / ".config" / "miu" / "db"
    new_path.mkdir(parents=True)
    (new_path / "settings.json").write_text('{"source": "new"}')

    resolved = _resolve()

    assert resolved == new_path
    assert (new_path / "settings.json").read_text() == '{"source": "new"}'


def test_resolution_is_pure_lookup(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    home = _isolated_home(monkeypatch, tmp_path)

    resolved = _resolve()

    assert resolved == home / ".config" / "miu" / "db"
    assert not resolved.exists()


def test_legacy_sqlit_config_files_are_copied_once(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    home = _isolated_home(monkeypatch, tmp_path)
    legacy_path = home / ".config" / "sqlit"
    new_path = home / ".config" / "miu" / "db"
    legacy_path.mkdir(parents=True)
    (legacy_path / "connections.json").write_text('{"source": "sqlit"}')
    (legacy_path / "nested").mkdir()
    (legacy_path / "nested" / "settings.json").write_text('{"theme": "old"}')

    resolved = _resolve()

    assert resolved == new_path
    assert (new_path / "connections.json").read_text() == '{"source": "sqlit"}'
    assert (new_path / "nested" / "settings.json").read_text() == '{"theme": "old"}'


def test_legacy_sqlit_migration_does_not_overwrite_existing_files(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    home = _isolated_home(monkeypatch, tmp_path)
    legacy_path = home / ".config" / "sqlit"
    new_path = home / ".config" / "miu" / "db"
    legacy_path.mkdir(parents=True)
    new_path.mkdir(parents=True)
    (legacy_path / "settings.json").write_text('{"source": "sqlit"}')
    (legacy_path / "connections.json").write_text('{"source": "sqlit"}')
    (new_path / "settings.json").write_text('{"source": "miu"}')

    resolved = _resolve()

    assert resolved == new_path
    assert (new_path / "settings.json").read_text() == '{"source": "miu"}'
    assert (new_path / "connections.json").read_text() == '{"source": "sqlit"}'
