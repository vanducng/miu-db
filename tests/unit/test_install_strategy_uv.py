"""Tests for distinguishing `uv tool install` from `uvx` (issue #133).

Background: both install flavors land executables under the uv cache/data
tree, but `uv tool install` is persistent and `uvx` is ephemeral. They
require different commands to add an extra dependency:

- uv tool install → `uv tool install --reinstall --with X miu-db`
- uvx            → `uvx --from miu-db --with X sqlit`

The previous `is_uvx()` check (matching `/uv/tools/`) actually matched the
persistent path and would hand `uv tool install` users a broken uvx
command.
"""

from __future__ import annotations

from miu_db.domains.connections.app.install_strategy import (
    detect_install_method,
    get_install_options,
)
from miu_db.shared.core.system_probe import SystemProbe


def _probe_with_exe(path: str) -> SystemProbe:
    return SystemProbe(
        env={"_MIU_DB_TEST": "1"},
        executable=path,
        prefix=path,
        base_prefix=path,
        pip_available=True,
    )


def test_uv_tool_install_path_detected_as_uv_tool() -> None:
    probe = _probe_with_exe(
        "/home/alice/.local/share/uv/tools/miu-db/bin/python"
    )
    assert probe.is_uv_tool_install() is True
    assert probe.is_uvx() is False
    assert detect_install_method(probe=probe) == "uv-tool"


def test_uvx_ephemeral_path_detected_as_uvx() -> None:
    probe = _probe_with_exe(
        "/home/alice/.cache/uv/environments-v2/7b6a360d9162862b/845f3c5907156215/bin/python"
    )
    assert probe.is_uv_tool_install() is False
    assert probe.is_uvx() is True
    assert detect_install_method(probe=probe) == "uvx"


def test_uvx_legacy_archive_path_detected_as_uvx() -> None:
    """Older uv caches reference `archive-v0/` directly; still ephemeral."""
    probe = _probe_with_exe(
        "/home/alice/.cache/uv/cache/archive-v0/abcdef/bin/python"
    )
    assert probe.is_uvx() is True
    assert detect_install_method(probe=probe) == "uvx"


def test_uv_tool_install_option_uses_reinstall_with() -> None:
    """Issue #133: a `uv tool install` user needs a persistent reinstall,
    not `uvx --with ... miu-db` (which is ephemeral and wrong syntax)."""
    probe = _probe_with_exe(
        "/home/alice/.local/share/uv/tools/miu-db/bin/python"
    )
    options = get_install_options(
        package_name="PyMySQL",
        extra_name="mysql",
        probe=probe,
    )
    detected = [opt for opt in options if opt.label == "uv-tool"]
    assert detected, "uv-tool option must be present"
    assert (
        detected[0].command
        == "uv tool install --reinstall --with PyMySQL miu-db"
    )
    # First option is the detected one
    assert options[0].label == "uv-tool"


def test_uvx_option_uses_from_flag_and_correct_executable() -> None:
    """`uvx <package>` fails when the package and executable names differ.
    Must be `uvx --from miu-db --with ... miu-db` (package == binary)."""
    probe = _probe_with_exe(
        "/home/alice/.cache/uv/environments-v2/abc/def/bin/python"
    )
    options = get_install_options(
        package_name="PyMySQL",
        extra_name="mysql",
        probe=probe,
    )
    detected = [opt for opt in options if opt.label == "uvx"]
    assert detected
    cmd = detected[0].command
    assert cmd.startswith("uvx --from miu-db --with ")
    assert cmd.endswith(" miu-db")
    assert cmd.split()[-1] == "miu-db", "executable must be the miu-db CLI"


def test_previous_uvx_false_match_on_uv_tools_path_is_gone() -> None:
    """Regression: the old is_uvx() matched `/uv/tools/` and misclassified
    `uv tool install` users as uvx. Pin that exact case here so we don't
    reintroduce the conflation."""
    probe = _probe_with_exe(
        "/home/alice/.local/share/uv/tools/miu-db/bin/python"
    )
    assert probe.is_uvx() is False
    # And the detected method is uv-tool, not uvx
    assert detect_install_method(probe=probe) == "uv-tool"
