"""Tests for KeyringMigrator: copy entries from legacy service to new service."""

from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest

from miu_db.shared.migration.keyring_migrator import KeyringMigrator, LEGACY_KEYRING_SERVICE_NAME


_NEW_SERVICE = "miu-db"


def _make_keyring_mock(legacy_store: dict[str, str]) -> MagicMock:
    """Build a mock keyring module backed by two in-memory dicts."""
    new_store: dict[str, str] = {}

    mock_kr = MagicMock()

    def get_password(service: str, key: str) -> str | None:
        store = legacy_store if service == LEGACY_KEYRING_SERVICE_NAME else new_store
        return store.get(key)

    def set_password(service: str, key: str, value: str) -> None:
        if service == _NEW_SERVICE:
            new_store[key] = value

    mock_kr.get_password.side_effect = get_password
    mock_kr.set_password.side_effect = set_password
    mock_kr.errors = MagicMock()
    mock_kr.errors.KeyringError = Exception

    return mock_kr, new_store


def test_copies_known_entries() -> None:
    legacy = {"conn-a:db": "secret-db", "conn-a:ssh": "secret-ssh", "conn-b:db": "pw2"}
    mock_kr, new_store = _make_keyring_mock(legacy)

    with patch.dict("sys.modules", {"keyring": mock_kr, "keyring.errors": mock_kr.errors}):
        count = KeyringMigrator(["conn-a", "conn-b"]).migrate()

    assert count == 3
    assert new_store["conn-a:db"] == "secret-db"
    assert new_store["conn-a:ssh"] == "secret-ssh"
    assert new_store["conn-b:db"] == "pw2"


def test_skips_entries_already_in_new_service() -> None:
    legacy = {"conn-a:db": "old-pw"}
    mock_kr, new_store = _make_keyring_mock(legacy)
    new_store["conn-a:db"] = "existing-pw"

    with patch.dict("sys.modules", {"keyring": mock_kr, "keyring.errors": mock_kr.errors}):
        count = KeyringMigrator(["conn-a"]).migrate()

    assert count == 0
    assert new_store["conn-a:db"] == "existing-pw"


def test_skips_entries_not_in_legacy() -> None:
    mock_kr, new_store = _make_keyring_mock({})

    with patch.dict("sys.modules", {"keyring": mock_kr, "keyring.errors": mock_kr.errors}):
        count = KeyringMigrator(["ghost-conn"]).migrate()

    assert count == 0
    assert new_store == {}


def test_skips_when_backend_unavailable_per_entry() -> None:
    """KeyringError on get_password must be swallowed; other entries still processed."""
    mock_kr = MagicMock()
    new_store: dict[str, str] = {}

    call_count = {"n": 0}

    def get_password(service: str, key: str) -> str | None:
        call_count["n"] += 1
        if key == "bad-conn:db":
            raise Exception("backend unavailable")
        if service == LEGACY_KEYRING_SERVICE_NAME and key == "good-conn:db":
            return "good-pw"
        return None

    def set_password(service: str, key: str, value: str) -> None:
        new_store[key] = value

    mock_kr.get_password.side_effect = get_password
    mock_kr.set_password.side_effect = set_password
    mock_kr.errors = MagicMock()
    mock_kr.errors.KeyringError = Exception

    with patch.dict("sys.modules", {"keyring": mock_kr, "keyring.errors": mock_kr.errors}):
        count = KeyringMigrator(["bad-conn", "good-conn"]).migrate()

    assert count == 1
    assert new_store.get("good-conn:db") == "good-pw"


def test_returns_zero_when_keyring_import_fails() -> None:
    import sys
    saved = sys.modules.pop("keyring", None)
    try:
        count = KeyringMigrator(["conn-x"]).migrate()
    finally:
        if saved is not None:
            sys.modules["keyring"] = saved
    assert count == 0


def test_empty_connection_list() -> None:
    mock_kr = MagicMock()
    mock_kr.errors = MagicMock()
    mock_kr.errors.KeyringError = Exception

    with patch.dict("sys.modules", {"keyring": mock_kr, "keyring.errors": mock_kr.errors}):
        count = KeyringMigrator([]).migrate()

    assert count == 0
    mock_kr.get_password.assert_not_called()
