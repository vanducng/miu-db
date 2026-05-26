"""One-shot keyring migration from service='sqlit' to service='miu-db'. Remove in v2.0.0."""

from __future__ import annotations


LEGACY_KEYRING_SERVICE_NAME = "sqlit"  # sqlit-legacy: read by this migrator only


class KeyringMigrator:
    def __init__(self, connection_names: list[str]) -> None:
        self.connection_names = connection_names

    def migrate(self) -> int:
        """Copy entries from legacy service to new service. Return count copied."""
        try:
            import keyring
            import keyring.errors
        except ImportError:
            return 0

        from miu_db.domains.connections.app.credentials import KEYRING_SERVICE_NAME

        count = 0
        for name in self.connection_names:
            for suffix in (":db", ":ssh"):
                key = f"{name}{suffix}"
                try:
                    old = keyring.get_password(LEGACY_KEYRING_SERVICE_NAME, key)
                    if old is None:
                        continue
                    existing = keyring.get_password(KEYRING_SERVICE_NAME, key)
                    if existing is not None:
                        continue
                    keyring.set_password(KEYRING_SERVICE_NAME, key, old)
                    count += 1
                except keyring.errors.KeyringError:
                    continue
        return count
