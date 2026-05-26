"""Protocol for Vim mode state."""

from __future__ import annotations

from typing import TYPE_CHECKING, Protocol

if TYPE_CHECKING:
    from miu_db.core.vim import VimMode


class VimModeProtocol(Protocol):
    vim_mode: VimMode
