"""Protocol definitions for mixin type safety."""

from __future__ import annotations

from typing import Protocol

from miu_db.shared.ui.protocols.autocomplete import AutocompleteProtocol
from miu_db.shared.ui.protocols.connections import ConnectionsProtocol
from miu_db.shared.ui.protocols.core import TextualAppProtocol
from miu_db.shared.ui.protocols.explorer import ExplorerProtocol
from miu_db.shared.ui.protocols.lifecycle import LifecycleProtocol
from miu_db.shared.ui.protocols.metadata import MetadataHelpersProtocol
from miu_db.shared.ui.protocols.mixins import (
    AutocompleteMixinHost,
    ConnectionMixinHost,
    QueryMixinHost,
    ResultsFilterMixinHost,
    ResultsMixinHost,
    TreeFilterMixinHost,
    TreeMixinHost,
    UINavigationMixinHost,
)
from miu_db.shared.ui.protocols.query import QueryProtocol
from miu_db.shared.ui.protocols.results import ResultsProtocol
from miu_db.shared.ui.protocols.screens import ThemeScreenAppProtocol
from miu_db.shared.ui.protocols.startup import StartupProtocol
from miu_db.shared.ui.protocols.ui_navigation import UINavigationProtocol
from miu_db.shared.ui.protocols.vim import VimModeProtocol
from miu_db.shared.ui.protocols.widgets import WidgetAccessProtocol


class AppProtocol(
    TextualAppProtocol,
    WidgetAccessProtocol,
    MetadataHelpersProtocol,
    ConnectionsProtocol,
    VimModeProtocol,
    ExplorerProtocol,
    QueryProtocol,
    AutocompleteProtocol,
    ResultsProtocol,
    UINavigationProtocol,
    StartupProtocol,
    LifecycleProtocol,
    Protocol,
):
    """Composite protocol for the miu-db Textual App."""

    pass


__all__ = [
    "AppProtocol",
    "AutocompleteMixinHost",
    "AutocompleteProtocol",
    "ConnectionMixinHost",
    "ConnectionsProtocol",
    "ExplorerProtocol",
    "LifecycleProtocol",
    "MetadataHelpersProtocol",
    "QueryMixinHost",
    "QueryProtocol",
    "ResultsFilterMixinHost",
    "ResultsMixinHost",
    "ResultsProtocol",
    "StartupProtocol",
    "TextualAppProtocol",
    "ThemeScreenAppProtocol",
    "TreeFilterMixinHost",
    "TreeMixinHost",
    "UINavigationMixinHost",
    "UINavigationProtocol",
    "VimModeProtocol",
    "WidgetAccessProtocol",
]
