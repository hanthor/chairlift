# Plan: Bluefin suite parity

Brings the functionality of [bluefinctl](https://github.com/projectbluefin/bluefinctl)
(Textual TUI) and [finupdate](https://github.com/tuna-os/finupdate) (GTK4/Rust)
into ChairLift, so that one GTK4/Libadwaita application covers system
management on Bluefin, Bluefin LTS, and Dakota as well as on Snow Linux.

Two constraints shape every phase and are not negotiable per-item:

- **GNOME HIG, simple surface.** The goal is the *capability*, not the source
  application's control count. Where bluefinctl or finupdate expose a
  knob-per-behavior, ChairLift exposes the smallest control that gets the job
  done. The reference app is GNOME Software — hero status, list of pending
  changes, drill-down for detail — as identified by finupdate's own
  [HIG audit](https://github.com/tuna-os/finupdate/blob/main/docs/GNOME-HIG-AUDIT.md).
  A ported feature that arrives as six switches has not been ported correctly.
- **Every phase is proven by automated checks.** `make ci` for logic,
  `make e2e` for the installed boundary, and the screenshot walkthrough for
  the rendered surface. Per
  [acceptance-criteria-need-automated-checks-not-inspection](../agents/skills/acceptance-criteria-need-automated-checks-not-inspection.md),
  "verified by inspection" does not close a phase.

## Feature matrix

Source of each capability, and where it stands. "Already" means ChairLift
covered it before this plan.

| Capability | bluefinctl | finupdate | Status |
| --- | :---: | :---: | --- |
| OS image staging with live progress + log | ✅ | ✅ | Already (`internal/bootc`, `internal/stageexec`) |
| Native A/B (systemd-sysupdate) staging | — | — | Already (`internal/sysupdate`) |
| Flatpak list / update / uninstall | ✅ | ✅ | Already (`internal/flatpak`) |
| Homebrew install / search / upgrade / pin / bundles | ✅ | ✅ | Already (`internal/homebrew`) |
| Homebrew tap trust | — | — | Already (`internal/homebrew/trust.go`) |
| System info, os-release, health | ✅ | — | Already (`system_page.go`) |
| Updex features | — | — | Already (`internal/updex`) |
| Testing-channel switch | ✅ | ✅ | **Done** — `internal/imageinfo`, `internal/ublue` |
| Developer mode (dx groups) | ✅ | — | **Done** — `internal/ubluehelper` |
| Gaming mode | — | — | **Done** — `internal/gaming` (new; no upstream) |
| Per-image release-channel table + override | — | partial | **Done** — `channels.yml` |
| **Update All** (OS + Flatpak + Brew, one action) | ✅ `bctl update` | ✅ hero button | **Done** — `internal/updateall` |
| Restart to apply / reboot prompt | ✅ | ✅ | **Done** — `restart` helper subcommand |
| Rollback to previous deployment | ✅ calendar | ✅ | **Done** — `rollback` helper subcommand |
| Automatic background updates (uupd timer) | ✅ | ✅ | **Phase 3** |
| Image variant selection (`-nvidia`, `-dx`) | — | ✅ rebase dialog | **Phase 4** |
| Pin to dated tag / unpin to stream | — | ✅ | **Phase 4** |
| GPU detection | ✅ | ✅ | **Phase 4** (needed to offer the right variant) |
| Action journal of privileged operations | — | ✅ | **Phase 5** |
| Desktop notifications | ✅ `notify-send` | ✅ `GNotification` | **Phase 5** |
| Update strategy / focus mode / per-layer switches | ✅ | — | **Not porting** — the option sprawl the HIG constraint rules out. Phase 3's single switch replaces it. |
| Reboot-on-logout / scheduled reboot window | ✅ | ✅ "Restart Tonight" | **Deferred** — needs a systemd user unit contract; revisit after Phase 3 |
| Powerwash / Factory Reset | — | ✅ | **Approved 2026-08-17** — Phase 6. Irreversible, so both need an explicit typed/confirmed dialog and `bootc install reset` must stay behind its `--experimental` warning |
| AI / GPU container stacks | ✅ | — | **Approved 2026-08-17** — Phase 7. Must arrive as one "AI stack" choice per detected GPU, not a 30-entry catalog |
| Changelog / SBOM diff between images | — | ✅ | **Approved 2026-08-17** — Phase 8. Needs an offline-testable SPDX parser seam; the network call cannot be in the gated tests |
| D-Bus progress publishing | — | ✅ | **Not porting** — exists to feed finupdate's GNOME Shell extension; no consumer here |
| Dev tools installer (docker, Lima/WSL, editors, VMs) | ✅ | — | **Not porting as recipes** — ChairLift already has Brew search/install, Flatpak install, and configurable `brew_bundles_group` `bundles_paths`. The curated list belongs in a shipped bundle file, not ~500 lines of per-tool install code. |
| GNOME Control Center panel | — | ✅ | **Not porting** — finupdate ships a C panel + patches against gnome-control-center; out of scope for a standalone app |

## Phase 1 — Update All (medium) — ✅ landed

One button that brings the whole system up to date, matching both tools'
headline feature, with a single restart prompt at the end.

Composition, not new privilege: the OS phase calls the existing
`internal/bootc` staging path (`pkexec /usr/libexec/bootc-update-stage`,
action `org.frostyard.ChairLift.bootc.stage`), because
[AGENTS.md](../../AGENTS.md) fixes both "OS staging execution has one owner"
and the system-integration package's fixed-path contract. Flatpak and Homebrew
phases are already unprivileged. The only new privileged surface is
`systemctl reboot`.

- New pure `internal/updateall` sequencing the phases and owning the
  aggregate progress/outcome contract, with the per-provider work delegated
  to `internal/bootc`, `internal/flatpak`, and `internal/homebrew`.
- New `restart` subcommand on the existing `chairlift-ublue-helper`, with a
  matching action in `data/org.frostyard.ChairLift.ublue.policy` — not a
  third helper binary.
- One `updates_page` group: a single primary action plus per-phase status.
- **Done when:** the walkthrough captures the Updates page showing the Update
  All group with a non-empty plan, and `make ci` + `make e2e` are green.
  **Met.** The walkthrough asserts the `views: update all group built` marker
  names a non-zero phase count, and captures the rendered group. The run's
  own behavior — ordering, failure isolation, cancellation, and the restart
  decision — is covered by `internal/updateall`'s table tests rather than by
  screenshot, because a dry-run run completes faster than a frame can be
  captured and racing it would make the check flaky.

## Phase 2 — Rollback (small) — ✅ landed

- `bootc rollback` via a new `chairlift-ublue-helper` subcommand + action.
- Presented as a single row naming the deployment being returned to, mirroring
  `pageview.SysupdateRollbackSubtitle`'s existing grammar. Not bluefinctl's
  rollback calendar.
- **Done when:** the walkthrough captures the rollback row naming the previous
  deployment on a host with one, and its absence on a host without.
  **Met** for the absent case, which is what the CI runner is: the row stays
  hidden and the walkthrough's Updates frame shows the System Updates group
  without it. The present case is covered by `pageview`'s table test over the
  version/timestamp matrix and by the boundary test asserting the helper
  rejects any caller-supplied rollback target.

## Phase 3 — Automatic background updates (small)

- One switch: on/off, backed by `systemctl enable --now uupd.timer` /
  `disable` through a new helper subcommand + action.
- Hidden entirely when `uupd` is absent, the same availability pattern as
  `updex` and the Bluefin-family groups.
- **Done when:** the walkthrough captures the switch reflecting real timer
  state, and the switch is absent on a host without `uupd`.

## Phase 4 — Image variants (medium)

- Extend `internal/imageinfo`'s per-image table with variant suffixes
  (`-nvidia`, `-nvidia-open`, `-dx`), keeping the same discipline: **verify
  against the registry before mapping to a tag**. finupdate's `KNOWN_FAMILIES`
  was verified 2026-05-30 and records that `dakota-dx` does not exist; treat
  it as a starting hypothesis, not as fact.
- GPU detection to decide which variant to offer.
- Pin to a dated tag / unpin to the stream.
- **Done when:** a table test covers the variant matrix for all three images
  against registry-verified references, and the walkthrough captures the
  variant row.

## Phase 5 — Audit and notification (small)

- Action journal: append-only JSONL of every privileged action, mirroring
  finupdate's `action_journal.rs`. It also makes the walkthrough's
  privileged-path assertions machine-checkable.
- Desktop notification on completion of a long-running operation.
- **Done when:** the e2e boundary test asserts a journal entry per privileged
  invocation.

## Phase 6 — Powerwash and Factory Reset (medium)

Approved 2026-08-17. Both are irreversible, which sets the bar for how they
are presented and tested.

- Powerwash: `flatpak uninstall --user --all` plus `distrobox rm -f -a`. No
  privilege required — it is all user-scope, like gaming mode.
- Factory Reset: `bootc install reset --experimental --apply` through a new
  helper subcommand + action. The `--experimental` flag must be surfaced in
  the confirmation, not hidden.
- Both behind an `AdwAlertDialog` with a destructive-styled confirm, per the
  HIG's rule that destructive dialogs are reserved for non-undoable actions.
- **Done when:** the walkthrough captures both confirmation dialogs, and the
  e2e boundary test asserts the helper rejects a factory reset invocation
  carrying any argument.

## Phase 7 — AI stacks (medium)

Approved 2026-08-17, with the constraint that it arrives as one choice per
detected GPU rather than bluefinctl's full quadlet catalog.

- GPU detection selects the stack; the user sees a single switch, not a
  vendor/stack matrix.
- Quadlet units installed user-scope under `~/.config/containers/systemd/`.
- **Done when:** a table test covers stack selection for Nvidia, AMD, Intel,
  and no-GPU hosts, and the walkthrough captures the row on a stubbed GPU.

## Phase 8 — Changelog / SBOM diff (large)

Approved 2026-08-17.

- SPDX parsing and package diffing behind a pure package with the network
  fetch as a seam, so the gated tests never make a request.
- Presented as a drill-down from the staged-update row, not as a top-level
  page.
- **Done when:** the diff is table-tested against checked-in SPDX fixtures,
  and the walkthrough captures the drill-down.

## Later / ideas

- Reboot-on-logout and scheduled reboot windows, if the systemd user unit
  contract can be made as simple as one switch.
- A shipped Brew bundle expressing bluefinctl's curated developer tool list,
  replacing its per-tool install recipes.

## Open questions

- **Powerwash, Factory Reset, AI/GPU stacks, SBOM changelog:** port or drop?
  Each is listed above with the reason it is blocked. Decided by the
  maintainer, before Phase 5.
- **Is ChairLift the intended home for Bluefin-family system management, or is
  finupdate?** The two now overlap on updates and rebasing. Worth settling
  before Phase 4 adds a second rebase UI to the ecosystem.

## References

- Implements: [design/overview.md](../design/overview.md)
- Sources: [bluefinctl](https://github.com/projectbluefin/bluefinctl),
  [finupdate](https://github.com/tuna-os/finupdate)
