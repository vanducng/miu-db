"""Query editor focused state."""

from __future__ import annotations

from miu_db.core.input_context import InputContext
from miu_db.core.state_base import State


class QueryFocusedState(State):
    """Base state when query editor has focus."""

    def _setup_actions(self) -> None:
        pass

    def is_active(self, app: InputContext) -> bool:
        return app.focus == "query"
