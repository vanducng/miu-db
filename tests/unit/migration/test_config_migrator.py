"""Tests for ConfigMigrator: copy-not-move, marker idempotency, theme rewrite, atomic temp file."""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from miu_db.shared.migration.config_migrator import ConfigMigrator


def _make_migrator(home: Path) -> ConfigMigrator:
    """Return a ConfigMigrator whose LEGACY_PATHS and NEW_PATH are rooted at home."""
    return ConfigMigrator()


def _seed_legacy_xdg(home: Path, files: dict[str, str] | None = None) -> Path:
    legacy = home / ".config" / "sqlit"
    legacy.mkdir(parents=True)
    (legacy / "connections.json").write_text("[]")
    for name, content in (files or {}).items():
        p = legacy / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(content)
    return legacy


def _seed_legacy_dot(home: Path, files: dict[str, str] | None = None) -> Path:
    legacy = home / ".sqlit"
    legacy.mkdir(parents=True)
    (legacy / "connections.json").write_text("[]")
    for name, content in (files or {}).items():
        p = legacy / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(content)
    return legacy


def test_copies_xdg_legacy(tmp_path: Path) -> None:
    _seed_legacy_xdg(tmp_path, {"settings.json": '{"theme": "default"}'})
    m = _make_migrator(tmp_path)
    result = m.migrate()
    assert result.migrated
    assert result.source == tmp_path / ".config" / "sqlit"
    new_path = tmp_path / ".config" / "miu-db"
    assert (new_path / "connections.json").exists()
    assert (new_path / "settings.json").exists()
    assert (new_path / ConfigMigrator.MARKER).exists()


def test_copies_dot_sqlit_legacy(tmp_path: Path) -> None:
    _seed_legacy_dot(tmp_path, {"settings.json": '{"theme": "default"}'})
    m = _make_migrator(tmp_path)
    result = m.migrate()
    assert result.migrated
    assert result.source == tmp_path / ".sqlit"
    assert (tmp_path / ".config" / "miu-db" / "connections.json").exists()


def test_prefers_xdg_when_both_exist(tmp_path: Path) -> None:
    _seed_legacy_xdg(tmp_path)
    _seed_legacy_dot(tmp_path)
    result = _make_migrator(tmp_path).migrate()
    assert result.source == tmp_path / ".config" / "sqlit"


def test_is_idempotent(tmp_path: Path) -> None:
    _seed_legacy_xdg(tmp_path)
    m = _make_migrator(tmp_path)
    r1 = m.migrate()
    r2 = m.migrate()
    assert r1.migrated
    assert not r2.migrated


def test_does_not_clobber_existing_miu_db_config(tmp_path: Path) -> None:
    _seed_legacy_xdg(tmp_path, {"settings.json": '{"theme": "legacy"}'})
    new_path = tmp_path / ".config" / "miu-db"
    new_path.mkdir(parents=True)
    (new_path / "settings.json").write_text('{"theme": "new"}')
    m = _make_migrator(tmp_path)
    result = m.migrate()
    assert result.migrated
    assert (new_path / "settings.json").read_text() == '{"theme": "new"}'


def test_handles_interrupted_run(tmp_path: Path) -> None:
    _seed_legacy_xdg(tmp_path, {"query_history.json": '[]'})
    new_path = tmp_path / ".config" / "miu-db"
    new_path.mkdir(parents=True)
    (new_path / ConfigMigrator.TEMP_MARKER).write_text("interrupted")
    result = _make_migrator(tmp_path).migrate()
    assert result.migrated
    assert (new_path / "connections.json").exists()
    assert not (new_path / ConfigMigrator.TEMP_MARKER).exists()


def test_no_op_when_no_legacy(tmp_path: Path) -> None:
    result = _make_migrator(tmp_path).migrate()
    assert not result.migrated
    assert result.source is None


def test_temp_marker_removed_on_success(tmp_path: Path) -> None:
    _seed_legacy_xdg(tmp_path)
    _make_migrator(tmp_path).migrate()
    new_path = tmp_path / ".config" / "miu-db"
    assert not (new_path / ConfigMigrator.TEMP_MARKER).exists()


def test_rewrites_sqlit_theme_to_miu_db(tmp_path: Path) -> None:
    _seed_legacy_xdg(tmp_path, {"settings.json": json.dumps({"theme": "sqlit"})})
    _make_migrator(tmp_path).migrate()
    new_settings = tmp_path / ".config" / "miu-db" / "settings.json"
    data = json.loads(new_settings.read_text())
    assert data["theme"] == "miu-db"


def test_rewrites_sqlit_light_theme(tmp_path: Path) -> None:
    _seed_legacy_xdg(tmp_path, {"settings.json": json.dumps({"theme": "sqlit-light"})})
    _make_migrator(tmp_path).migrate()
    data = json.loads((tmp_path / ".config" / "miu-db" / "settings.json").read_text())
    assert data["theme"] == "miu-db-light"


def test_leaves_other_themes_unchanged(tmp_path: Path) -> None:
    _seed_legacy_xdg(tmp_path, {"settings.json": json.dumps({"theme": "dracula"})})
    _make_migrator(tmp_path).migrate()
    data = json.loads((tmp_path / ".config" / "miu-db" / "settings.json").read_text())
    assert data["theme"] == "dracula"


def test_legacy_data_preserved_after_migration(tmp_path: Path) -> None:
    """Copy-not-move: legacy directory must still exist after migration."""
    _seed_legacy_xdg(tmp_path, {"settings.json": '{"x": 1}'})
    _make_migrator(tmp_path).migrate()
    assert (tmp_path / ".config" / "sqlit" / "connections.json").exists()
