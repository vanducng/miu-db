"""Runtime configuration for miu-db."""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from miu_db.shared.migration.env_compat import read_env


@dataclass
class MockConfig:
    """Mock-related runtime configuration."""

    enabled: bool = False
    profile: Any | None = None
    missing_drivers: set[str] = field(default_factory=set)
    install_result: str | None = None
    pipx_mode: str | None = None
    query_delay: float = 0.0
    demo_rows: int = 0
    demo_long_text: bool = False
    cloud: bool = False
    docker_containers: list[Any] | None = None
    driver_error: bool = False


@dataclass
class RuntimeConfig:
    """Runtime configuration provided by CLI or tests."""

    settings_path: Path | None = None
    theme: str | None = None
    max_rows: int | None = None
    debug_mode: bool = False
    debug_idle_scheduler: bool = False
    profile_startup: bool = False
    startup_mark: float | None = None
    startup_log_path: Path | None = None
    startup_exit_after_refresh: bool = False
    startup_import_log_path: Path | None = None
    startup_import_min_ms: float = 1.0
    process_worker: bool = True
    process_worker_warm_on_idle: bool = True
    process_worker_auto_shutdown_s: float = 0.0
    ui_stall_watchdog_ms: float = 0.0
    query_alert_mode: int = 0
    mock: MockConfig = field(default_factory=MockConfig)

    @classmethod
    def from_env(cls) -> RuntimeConfig:
        def _parse_startup_mark(value: str | None) -> float | None:
            if not value:
                return None
            try:
                return float(value)
            except (TypeError, ValueError):
                return None

        def _parse_int(value: str | None) -> int | None:
            if not value:
                return None
            try:
                return int(value)
            except (TypeError, ValueError):
                return None

        def _parse_float(value: str | None) -> float:
            if not value:
                return 0.0
            try:
                return float(value)
            except (TypeError, ValueError):
                return 0.0

        def _parse_bool(value: str | None, default: bool) -> bool:
            if value is None or not value.strip():
                return default
            return value.strip().lower() in {"1", "true", "yes", "on"}

        settings_path = (read_env("MIU_DB_SETTINGS_PATH", "SQLIT_SETTINGS_PATH") or "").strip() or None
        startup_log_path = (read_env("MIU_DB_PROFILE_STARTUP_FILE", "SQLIT_PROFILE_STARTUP_FILE") or "").strip() or None
        startup_exit = read_env("MIU_DB_PROFILE_STARTUP_EXIT", "SQLIT_PROFILE_STARTUP_EXIT") == "1"
        import_log_path = (read_env("MIU_DB_PROFILE_STARTUP_IMPORTS_FILE", "SQLIT_PROFILE_STARTUP_IMPORTS_FILE") or "").strip() or None
        import_enabled = read_env("MIU_DB_PROFILE_STARTUP_IMPORTS", "SQLIT_PROFILE_STARTUP_IMPORTS") == "1" or bool(import_log_path)
        import_min_raw = (read_env("MIU_DB_PROFILE_STARTUP_IMPORTS_MIN_MS", "SQLIT_PROFILE_STARTUP_IMPORTS_MIN_MS") or "").strip()
        import_min_ms = _parse_float(import_min_raw) if import_min_raw else 1.0
        profile_startup = (
            read_env("MIU_DB_PROFILE_STARTUP", "SQLIT_PROFILE_STARTUP") == "1"
            or bool(startup_log_path)
            or startup_exit
            or import_enabled
        )
        default_startup_log = Path(".miu-db") / "startup.txt"
        startup_log = Path(startup_log_path).expanduser() if startup_log_path else (default_startup_log if profile_startup else None)
        startup_import_log = (
            Path(import_log_path).expanduser()
            if import_log_path
            else (Path(".miu-db") / "startup-imports.txt" if import_enabled else None)
        )
        max_rows = _parse_int(read_env("MIU_DB_MAX_ROWS", "SQLIT_MAX_ROWS"))
        worker_env = read_env("MIU_DB_PROCESS_WORKER", "SQLIT_PROCESS_WORKER")
        if worker_env is None or not worker_env.strip():
            process_worker = True
        else:
            process_worker = worker_env.strip().lower() in {"1", "true", "yes", "on"}
        warm_env = read_env("MIU_DB_PROCESS_WORKER_WARM_ON_IDLE", "SQLIT_PROCESS_WORKER_WARM_ON_IDLE")
        process_worker_warm_on_idle = _parse_bool(warm_env, True)
        shutdown_env = read_env("MIU_DB_PROCESS_WORKER_AUTO_SHUTDOWN_S", "SQLIT_PROCESS_WORKER_AUTO_SHUTDOWN_S")
        process_worker_auto_shutdown_s = _parse_float(shutdown_env)
        stall_env = read_env("MIU_DB_UI_STALL_WATCHDOG_MS", "SQLIT_UI_STALL_WATCHDOG_MS")
        ui_stall_watchdog_ms = _parse_float(stall_env)
        missing_drivers = read_env("MIU_DB_MOCK_MISSING_DRIVERS", "SQLIT_MOCK_MISSING_DRIVERS") or ""
        missing_driver_set = {item.strip() for item in missing_drivers.split(",") if item.strip()}

        mock_config = MockConfig(
            missing_drivers=missing_driver_set,
            install_result=(read_env("MIU_DB_MOCK_INSTALL_RESULT", "SQLIT_MOCK_INSTALL_RESULT") or "").strip().lower() or None,
            pipx_mode=(read_env("MIU_DB_MOCK_PIPX", "SQLIT_MOCK_PIPX") or "").strip().lower() or None,
            query_delay=_parse_float(read_env("MIU_DB_MOCK_QUERY_DELAY", "SQLIT_MOCK_QUERY_DELAY")),
            demo_rows=_parse_int(read_env("MIU_DB_DEMO_ROWS", "SQLIT_DEMO_ROWS")) or 0,
            demo_long_text=read_env("MIU_DB_DEMO_LONG_TEXT", "SQLIT_DEMO_LONG_TEXT") == "1",
            cloud=read_env("MIU_DB_MOCK_CLOUD", "SQLIT_MOCK_CLOUD") == "1",
            driver_error=read_env("MIU_DB_MOCK_DRIVER_ERROR", "SQLIT_MOCK_DRIVER_ERROR") == "1",
        )

        return cls(
            settings_path=Path(settings_path).expanduser() if settings_path else None,
            max_rows=max_rows,
            debug_mode=read_env("MIU_DB_DEBUG", "SQLIT_DEBUG") == "1",
            debug_idle_scheduler=read_env("MIU_DB_DEBUG_IDLE_SCHEDULER", "SQLIT_DEBUG_IDLE_SCHEDULER") == "1",
            profile_startup=profile_startup,
            startup_mark=_parse_startup_mark(read_env("MIU_DB_STARTUP_MARK", "SQLIT_STARTUP_MARK")),
            startup_log_path=startup_log,
            startup_exit_after_refresh=startup_exit,
            startup_import_log_path=startup_import_log,
            startup_import_min_ms=import_min_ms,
            process_worker=process_worker,
            process_worker_warm_on_idle=process_worker_warm_on_idle,
            process_worker_auto_shutdown_s=process_worker_auto_shutdown_s,
            ui_stall_watchdog_ms=ui_stall_watchdog_ms,
            mock=mock_config,
        )
