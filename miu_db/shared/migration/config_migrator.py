"""One-shot config migration from sqlit legacy paths to miu-db. Remove in v2.0.0."""

from __future__ import annotations

import json
import shutil
import time
from dataclasses import dataclass, field
from pathlib import Path


@dataclass
class MigrationResult:
    migrated: bool = False
    source: Path | None = None
    errors: list[str] = field(default_factory=list)


class ConfigMigrator:
    MARKER = ".migrated-from-sqlit"
    TEMP_MARKER = ".migrating-from-sqlit"

    @property
    def LEGACY_PATHS(self) -> list[Path]:  # noqa: N802
        return [
            Path.home() / ".config" / "sqlit",
            Path.home() / ".sqlit",
        ]

    @property
    def NEW_PATH(self) -> Path:  # noqa: N802
        return Path.home() / ".config" / "miu-db"

    def migrate(self) -> MigrationResult:
        new_path = self.NEW_PATH
        if (new_path / self.MARKER).exists():
            return MigrationResult()

        source = self._pick_source()
        if source is None:
            return MigrationResult()

        new_path.mkdir(parents=True, exist_ok=True)
        temp = new_path / self.TEMP_MARKER
        temp.write_text(str(time.time()))
        errors: list[str] = []
        try:
            for item in source.iterdir():
                dest = new_path / item.name
                if dest.exists():
                    continue
                try:
                    if item.is_dir():
                        shutil.copytree(item, dest)
                    else:
                        shutil.copy2(item, dest)
                except OSError as exc:
                    errors.append(f"Could not copy {item}: {exc}")

            self._rewrite_theme_names(new_path / "settings.json")
            self._migrate_keyring(source)

            (new_path / self.MARKER).write_text(
                f"migrated from {source} at {time.time()}\n"
            )
            return MigrationResult(migrated=True, source=source, errors=errors)
        except OSError as exc:
            errors.append(str(exc))
            return MigrationResult(migrated=False, source=source, errors=errors)
        finally:
            temp.unlink(missing_ok=True)

    def _pick_source(self) -> Path | None:
        for p in self.LEGACY_PATHS:
            if p.exists() and (p / "connections.json").exists():
                return p
        return None

    def _rewrite_theme_names(self, settings_json: Path) -> None:
        if not settings_json.exists():
            return
        try:
            data = json.loads(settings_json.read_text(encoding="utf-8"))
        except (json.JSONDecodeError, OSError):
            return
        if not isinstance(data, dict):
            return
        theme = data.get("theme")
        if theme == "sqlit":
            data["theme"] = "miu-db"
        elif theme == "sqlit-light":
            data["theme"] = "miu-db-light"
        else:
            return
        try:
            settings_json.write_text(json.dumps(data, indent=2), encoding="utf-8")
        except OSError:
            pass

    def _migrate_keyring(self, source: Path) -> None:
        connections_file = source / "connections.json"
        if not connections_file.exists():
            return
        try:
            data = json.loads(connections_file.read_text(encoding="utf-8"))
        except (json.JSONDecodeError, OSError):
            return
        if not isinstance(data, list):
            return
        names = [entry.get("name") for entry in data if isinstance(entry, dict) and entry.get("name")]
        if not names:
            return
        try:
            from miu_db.shared.migration.keyring_migrator import KeyringMigrator
            KeyringMigrator(names).migrate()
        except Exception:  # noqa: BLE001
            pass
