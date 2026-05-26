"""Dual-read env vars during deprecation window. Remove SQLIT_* reads in v2.0.0."""

from __future__ import annotations

import os
import warnings


def read_env(new_var: str, old_var: str, default: str | None = None) -> str | None:
    new_val = os.environ.get(new_var)
    if new_val is not None:
        return new_val
    old_val = os.environ.get(old_var)
    if old_val is not None:
        warnings.warn(
            f"{old_var} is deprecated; use {new_var}. Will be removed in v2.0.",
            DeprecationWarning,
            stacklevel=2,
        )
        return old_val
    return default


def read_env_bool(new_var: str, old_var: str, default: bool = False) -> bool:
    val = read_env(new_var, old_var)
    if val is None:
        return default
    return val.lower() in ("1", "true", "yes", "on")
