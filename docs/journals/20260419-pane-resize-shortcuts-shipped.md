# Pane Resize Shortcuts Shipped

**Date**: 2026-04-19 22:15  
**Severity**: Low  
**Component**: Shell / Core Keymaps  
**Status**: Resolved  

## What Happened

Shipped the `pane-resize-shortcuts` feature (commit `9a178a1`, PR #8). Users can now enter resize mode via `<space>r` leader key, then use arrow keys to adjust sidebar width or query/results height by 2-cell increments. Any non-arrow key exits. Sizes persist to `settings.json` under the `layout` block.

## The Brutal Truth

This feature *should* have been simple — "just move a boundary, read/write JSON, bind some keys." Instead, it required wrestling with four competing design constraints:

1. **Keymap injection complexity**: The existing `ConfigurableKeymapProvider` only allowed rebinding actions that *already had* a default key. We needed to ship 4 new resize actions (`resize_pane_{left,right,up,down}`) with **no default binding** so users could opt into `Ctrl+arrow` themselves. Extending the provider to inject override-only `ActionKeyDef` entries felt like unplugging a tower of blocks to add one piece.

2. **Context-sensitive guards**: Resize arrows can't work in query `INSERT` mode (breaks TextArea word-nav with Ctrl+Arrows) or in tree/results filter inputs. Listing all these exceptions felt brittle — constantly afraid we'd miss one.

3. **Silent failures by design**: When the user mashes arrow keys at a boundary, we clamp (no resize) and stay in resize mode. This *feels* right ("silent no-op" matches the vision), but the first instinct was to either error or exit. Had to justify the design choice in review.

4. **Pane resolver mess**: We ended up with two helper functions — `_get_focus_pane()` and `_resolve_focused_pane()` — doing subtly different things (one returns "explorer/query/results", the other "sidebar/query/results"). The reviewer flagged this for cleanup but we deferred it. This is a **code smell** for next time.

## Technical Details

**New actions** (rebindable, no default keys):
```
action_resize_pane_left
action_resize_pane_right
action_resize_pane_up
action_resize_pane_down
```

**Constraints**:
- Sidebar width: 15–80 cells
- Query height: 20–80% of shell height
- Resize disabled: `query_insert`, `tree_filter`, `results_filter` contexts
- Modal open → clears stale resize-mode flag (defensive fix from review)

**Settings round-trip** (example):
```json
{
  "layout": {
    "sidebar_width": 25,
    "query_height": 50
  }
}
```

**Test coverage**: 51 tests added/modified (TDD: RED → GREEN per phase); all passing. Tests verify:
- Settings persistence (JSON round-trip)
- Clamp math (boundary conditions)
- Mode entry/exit lifecycle
- Override-only action injection in keymap provider
- `show-keymap` lists unbound rebindables

## What We Tried

1. **Initial design**: Per-action default keys for resize (`Ctrl+arrow`). Rejected because 4 new default bindings violate the "CEAFF" vision (Easy: no preference bloat; Fast: fewer keys to reach). Pivoted to leader-mode + rebindable pattern.

2. **Pane resolver unification**: Spent an hour trying to rationalize `_get_focus_pane` vs `_resolve_focused_pane` into one function. Too many call sites with different expectations. Punted — noted for cleanup.

3. **Exiting on boundary clamp**: First iteration threw an exception when resize hit a clamp. UX felt jarring. Changed to silent no-op (return bool for "did resize happen?"), way better.

## Root Cause Analysis

**Why did the keymap extension feel so messy?**  
The `ConfigurableKeymapProvider` was designed for "action **already has** a default key, user can override it." Extending it to "inject a fresh key def for keyless actions" required dipping into internals and threading a whitelist. This is the cost of bolting on a new pattern without refactoring the base design. **Should have** done a proper factory method or builder earlier, but that's a 2-hour refactoring for a P2 feature.

**Why the pane resolver split?**  
Tree/shell domains talk about "explorer" vs "sidebar" (same thing, different name), while the resize logic thinks in generic pane IDs. We coded around the impedance mismatch instead of bridging it. Small debt now, bigger refactor later.

## Lessons Learned

1. **Rebindable-without-default is a good pattern** — unlocks opt-in power-user features without cluttering the default keymap. But the infrastructure needs explicit support, not hacks.

2. **Silent no-ops at boundaries are acceptable UX** — if the user understands why (clamp). They felt natural in testing. Throw exceptions only for user errors, not for "you asked for something impossible."

3. **Context guards should live in `binding_contexts.py`** — we almost scattered them across three files. Centralizing the "when does this action apply?" logic makes future audits easier.

4. **Deferred cleanup is debt** — the pane resolver unification *sounds* future work, but the code confusion is happening **now** every time someone touches resize. Two small functions doing nearly the same thing is worse than one function that's slightly more complex.

## Next Steps

1. **Immediate**: Merge to main, wire PR #8.

2. **Short term (next sprint)**: Unify `_get_focus_pane` and `_resolve_focused_pane` in a single resolver function with clear semantics. This removes ambiguity for the next person who touches pane logic.

3. **Medium term**: Refactor `ConfigurableKeymapProvider` to have a proper "register keyless rebindable" API instead of the current override-only injection pattern. This makes it easier to add future actions.

4. **Monitor**: Watch for user reports about resize not working in unexpected contexts (there may be a filter input we missed).

---

## Unresolved Questions

- Should `layout` settings auto-populate defaults on first write? Currently relies on `LayoutState` defaults. Risk of stale settings if schema changes.
- Is `_resolve_focused_pane` actually needed, or is there a call site we could refactor to use `_get_focus_pane` only?
