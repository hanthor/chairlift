# 0011 — ChairLift owns Bluefin-family rebasing

- **Status:** Accepted
- **Date:** 2026-08-17

## Context

ChairLift, [bluefinctl](https://github.com/projectbluefin/bluefinctl), and
[finupdate](https://github.com/tuna-os/finupdate) all manage the same three
bootc images (Bluefin, Bluefin LTS, Dakota). Consolidating bluefinctl's and
finupdate's functionality into ChairLift removed the overlap on updates,
developer mode, and release channels — but rebasing was the one capability
where finupdate had a substantial existing implementation (a rebase dialog
with family, variant, and stream selection, backed by a `KNOWN_FAMILIES`
table) and ChairLift had none.

Building a second rebase UI without settling ownership would have left two
applications staging `bootc switch` against the same machine, each with its
own idea of which image references exist.

That idea is not a formality. Both existing implementations resolve targets
from tables that are wrong in ways verified against GHCR on 2026-08-17:

- bluefinctl's `toggle-testing` maps tags without regard to the image, and
  targets `ghcr.io/ublue-os/bluefin:testing` and
  `ghcr.io/projectbluefin/bluefin-lts:lts-testing`. Neither exists.
- finupdate's `KNOWN_FAMILIES` models driver variants per *family* rather than
  per *stream*, so it would offer `ghcr.io/ublue-os/bluefin-nvidia:lts`. The
  NVIDIA images are published for the latest/stable/gts/beta streams only.

A wrong target is not cosmetic: it is a staged transaction that fails to pull
on a user's machine.

## Decision

ChairLift owns Bluefin-family rebasing. It resolves every `bootc switch`
target — release channel and graphics driver alike — from tables in
`internal/imageinfo` that are keyed on **both** the registry path and the
stream, and whose entries are verified against the registry before being
added.

Rebasing targets never cross the pkexec boundary as arguments. The privileged
helper accepts a validated word (`stable`/`testing`, or
`standard`/`nvidia`/`nvidia-open`) and derives the reference itself from the
system image descriptor plus those tables.

Images beyond the three supported ones are added through the `channels.yml`
override rather than by editing Go.

## Consequences

Rebasing behaviour is now testable without a bootc host: the whole
image/stream/driver matrix is a table test, and the tables carry the observed
registry responses in their comments so a future editor can see what was
checked and when.

ChairLift will show a switch as **unavailable** in cases where the other tools
offer one — Bluefin Stable has no testing channel, and no LTS stream has an
NVIDIA image. This is correct but will read as a missing feature to anyone
comparing the three applications; the row states the reason rather than
leaving the user to guess.

The tables are a maintenance obligation. They encode facts about a registry
that changes without notice, and nothing in CI re-verifies them — the
verification is manual, recorded by date in the table comments. A stream that
gains or loses a driver image will be wrong until someone re-checks. The
mitigation is that being wrong fails closed: an unknown image or stream
resolves to no switch at all rather than to a guessed reference.

finupdate's rebase dialog is not ported. Its family/variant/stream matrix is
also a richer surface than ChairLift's simple-interface constraint allows —
ChairLift offers one driver switch, driven by detected hardware, rather than a
grid of feature chips.
