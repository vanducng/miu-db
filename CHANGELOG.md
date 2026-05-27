# Changelog

All notable user-facing changes to miu-db.

## 0.1.0 - 2026-05-27

### Changed

- First standalone `miu-db` release under the new package, command, config path,
  keyring service, docs, assets, and release flow.
- **Breaking:** `Enter` in the query editor now runs only the statement under the
  cursor (split by `;`), matching DataGrip / DBeaver / VS Code SQL Tools. Use
  `<space>ga` (or the existing `<space>gr`) to run all statements in the buffer.
  `Ctrl+Enter` in INSERT mode follows the same rule and keeps the cursor in
  INSERT mode after running.

### Added

- `<space>ga` leader alias for "run all statements".
- SSH tab now discovers aliases from `~/.ssh/config` with ProxyJump support.
- Subtle background tint on the lines of the statement under the cursor when
  the buffer contains two or more statements, so you can see what `Enter` will
  execute before pressing it.

### Removed

- Removed legacy package, config, keyring, environment-variable, and theme
  compatibility paths. Brand-new installs use only `miu-db` names.
