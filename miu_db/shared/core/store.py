"""Base store class with common JSON file operations."""

from __future__ import annotations

import json
import os
import shutil
import tempfile
from pathlib import Path
from typing import Any


CONFIG_HOME_SUBPATH = Path("miu") / "db"
LEGACY_CONFIG_DIR_NAME = "sqlit"


def _resolve_config_dir() -> Path:
    """Resolve the miu-db config directory.

    Precedence:
      1. $MIU_DB_CONFIG_DIR if set.
      2. $XDG_CONFIG_HOME/miu/db (falling back to ~/.config/miu/db).

    When using the default location, copy any missing files from the old
    sqlit config directory once so existing saved connections are available.
    """
    env_override = os.environ.get("MIU_DB_CONFIG_DIR")
    if env_override:
        return Path(env_override).expanduser()

    xdg_home = os.environ.get("XDG_CONFIG_HOME")
    base = Path(xdg_home).expanduser() if xdg_home else Path.home() / ".config"
    config_dir = base / CONFIG_HOME_SUBPATH
    _migrate_legacy_config_dir(base / LEGACY_CONFIG_DIR_NAME, config_dir)
    return config_dir


def _migrate_legacy_config_dir(source: Path, destination: Path) -> None:
    """Copy missing legacy sqlit config files into the miu/db config directory."""
    if not source.is_dir():
        return

    for source_path in source.rglob("*"):
        if not source_path.is_file():
            continue
        try:
            relative_path = source_path.relative_to(source)
        except ValueError:
            continue
        destination_path = destination / relative_path
        if destination_path.exists():
            continue
        try:
            destination_path.parent.mkdir(parents=True, exist_ok=True)
            os.chmod(destination_path.parent, 0o700)
            shutil.copy2(source_path, destination_path)
            os.chmod(destination_path, 0o600)
        except OSError:
            continue

    try:
        os.chmod(destination, 0o700)
    except OSError:
        pass


# Shared config directory. Resolved once at import time so module-level
# constants (e.g. CUSTOM_THEME_DIR) pick up the same value.
CONFIG_DIR = _resolve_config_dir()


class JSONFileStore:
    """Base class for JSON file-backed stores.

    Provides common file I/O operations with error handling.
    """

    def __init__(self, file_path: Path):
        self._file_path = file_path

    @property
    def file_path(self) -> Path:
        """Get the store's file path."""
        return self._file_path

    def _ensure_dir(self) -> None:
        """Ensure the config directory exists with secure permissions."""
        dir_path = self._file_path.parent
        dir_path.mkdir(parents=True, exist_ok=True)
        # Set directory to owner-only access (0700)
        try:
            os.chmod(dir_path, 0o700)
        except OSError:
            pass  # Best effort on platforms that don't support chmod

    def _read_json(self) -> Any:
        """Read and parse JSON from file.

        Returns:
            Parsed JSON data, or None if file doesn't exist or is invalid.
        """
        if not self._file_path.exists():
            return None
        try:
            with open(self._file_path, encoding="utf-8") as f:
                return json.load(f)
        except (json.JSONDecodeError, TypeError):
            return None

    def _write_json(self, data: Any) -> None:
        """Write data as JSON to file atomically with secure permissions.

        Uses temp file + rename for atomic writes to prevent data corruption
        on crash/power failure. Sets file permissions to owner-only (0600).

        Args:
            data: Data to serialize and write.
        """
        self._ensure_dir()
        # Create temp file in same directory (required for atomic rename)
        fd, tmp_path = tempfile.mkstemp(
            dir=self._file_path.parent,
            prefix=".tmp_",
            suffix=".json",
        )
        try:
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                json.dump(data, f, indent=2)
            # Set file to owner-only access (0600) before making visible
            os.chmod(tmp_path, 0o600)
            # Atomic rename (on POSIX systems)
            os.replace(tmp_path, self._file_path)
        except Exception:
            # Clean up temp file on failure
            try:
                os.unlink(tmp_path)
            except OSError:
                pass
            raise

    def exists(self) -> bool:
        """Check if the store file exists."""
        return self._file_path.exists()
