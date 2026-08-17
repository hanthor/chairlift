# ChairLift walkthrough

Every screen in ChairLift, in the order you meet them. The screenshots are
captured from the real application by `make screenshots` — see
[How these are made](#how-these-are-made) — so what you see below is what the
app draws, not a mockup.

ChairLift covers Snow Linux and the Bluefin family (Bluefin, Bluefin LTS, and
Dakota) from one application. Where a feature came from
[bluefinctl](https://github.com/projectbluefin/bluefinctl) or
[finupdate](https://github.com/tuna-os/finupdate), that is noted — including
where ChairLift deliberately presents it with fewer controls than the tool it
came from.

---

## Keep the system up to date

![Updates](screenshots/3-updates.png)

**Update All** is one action for the whole system: it stages the next OS
image, updates Flatpak applications, and upgrades Homebrew packages, showing
each phase's status as it goes. A phase that fails does not stop the others —
your applications still update even if the image server is unreachable.

Only the providers actually installed on the machine appear. The screenshot
above is from a host with no bootc, so Update All lists two phases; on
Bluefin it lists three, with **System Image** first.

A restart prompt appears under the phase rows after a run that stages an OS
image — and only then. The staging script is idempotent, so a system that was
already current finishes without asking you to reboot for nothing.

**Automatic Updates** is one switch: does this system update itself in the
background, or only when you ask. bluefinctl expresses the same capability as
an update-strategy picker plus a schedule chooser, three per-layer switches,
and a focus mode. Those are five controls answering one question, so ChairLift
asks the question. The switch is hidden entirely on a system without the
unattended-update timer.

Below Update All, the per-provider groups remain for anything it does not
cover: expanding **Flatpak Updates** or **Homebrew Updates** shows the
individual pending packages, so you can update one thing without updating
everything.

> **Roll Back** lives in the **System Updates** group on this page, which
> appears only on a bootc host that ships the update-stage script — so it is
> absent from the screenshot above, which was captured on a workstation.
> It is a single row naming the deployment you would return to, and it takes
> effect at the next restart. It is one row rather than bluefinctl's rollback
> calendar because `bootc rollback` has exactly one destination: the
> deployment the host already records. Offering a calendar would imply a
> choice the operation does not have.

---

## Choose a release channel, developer tools, and gaming

![Features](screenshots/5-features.png)

This page is where the Bluefin-family features live. Each group hides itself
on a system without `/usr/share/ublue-os/image-info.json`, so none of it
appears on Snow Linux or any other non-Bluefin host.

**Release Channel** shows which image and stream you are running — "Dakota ·
latest" above — with one switch to move between the stable and testing
streams. The switch stages a `bootc switch`; you restart when you're ready.

The switch is deliberately unavailable on some systems, and that is worth
understanding. ChairLift resolves the target from a per-image table rather
than by appending a suffix to your current tag, because the mapping genuinely
differs per image. Verified against GHCR on 2026-08-17:

| Image | Stable streams | Testing streams |
| --- | --- | --- |
| `ghcr.io/ublue-os/bluefin` | `latest`, `stable`, `stable-daily`, `gts`, `beta`, `lts`, `lts-hwe` | `lts-testing`, `lts-hwe-testing` |
| `ghcr.io/projectbluefin/bluefin-lts` | `lts`, `stable` | `testing` |
| `ghcr.io/projectbluefin/dakota` | `latest`, `stable` | `testing` |

So Bluefin Stable on `latest` or `stable` has **no testing counterpart** —
`ghcr.io/ublue-os/bluefin:testing` does not exist — and the switch is shown
inactive with the reason in its subtitle, rather than staging a switch to an
image that would fail to pull. Downstream images add themselves through a
[`channels.yml`](../channels.example.yml) file; no code change needed.

**Graphics Driver** shows the hardware ChairLift detected and which driver
image you are running. Universal Blue ships NVIDIA's driver as a *separate
image* rather than layering it, so getting it is a `bootc switch` — which is
why this sits with the release channel: both change which image boots, one by
tag and one by name.

The row only offers an action when it helps: an NVIDIA card running the
standard image is offered the NVIDIA one, as above. It never pushes a machine
*off* a driver image, and it never chooses between the proprietary driver and
the open kernel modules, because that depends on the GPU generation and
choosing wrong leaves an unbootable desktop.

Like the channel switch, availability is verified rather than assumed. The
driver images are published for the `latest`, `stable`, `gts`, and `beta`
streams only — `ghcr.io/ublue-os/bluefin-nvidia:lts` is a 404 — so an LTS host
with an NVIDIA card is told what it has and offered nothing, rather than being
sent to an image that would fail to pull. Detection reads PCI vendor IDs from
sysfs rather than asking `nvidia-smi`, which only exists once the driver is
already installed and would therefore answer "no NVIDIA card" on exactly the
machines that need the offer.

**Developer Mode** adds your account to the container, VM, and serial-device
groups (`docker`, `incus-admin`, `libvirt`, `dialout`), which takes effect at
your next login. The subtitle says plainly that it does *not* rebase you to a
`-dx` image, because a Bluefin user who has seen those images would reasonably
expect it to. Group membership is the only form of developer mode available on
all three supported systems: Dakota publishes no `-dx` variant at all.

**Gaming Mode** installs Steam, ProtonUp-Qt, Protontricks, MangoHud, GOverlay,
and Flatseal. They are all user-scope Flatpaks — nothing is layered onto the
system image, so a system update never has to reconcile them, and the whole
feature needs no administrator password. Turning it off removes what ChairLift
installed and leaves anything your image shipped system-wide alone.

**Features** at the bottom is Snow Linux's updex feature manager, which is
empty on a non-updex host.

---

## Install and manage applications

![Applications](screenshots/1-applications.png)

Installed Flatpak applications and Homebrew formulae and casks, each
expandable, with search across both Homebrew namespaces. Homebrew rows offer
uninstall, and formulae can be pinned. **Bundles** installs a curated set of
packages in one action.

Homebrew 6's per-tap trust model hides packages installed from an untrusted
tap; ChairLift detects those and offers to trust the tap — per-user, with no
administrator password, because tap trust is a user-scope decision.

---

## Check on the system

![System](screenshots/4-system.png)

Image and deployment status, parsed `os-release` fields, and a link into
Mission Center for live resource monitoring.

---

## Clean up

![Maintenance](screenshots/2-maintenance.png)

Homebrew and Flatpak cleanup, plus any maintenance actions the image
maintainer configured in `config.yml`.

---

## Get help

![Help](screenshots/6-help.png)

Links to the distribution's website, issue tracker, and discussions, all
configurable per image.

---

## How these are made

```bash
make screenshots
```

That builds the application, runs it under a private Xvfb and D-Bus session,
drives it through every page with its own `Alt+<number>` accelerators, and
writes one cropped PNG per page to `docs/screenshots/`. It is the same harness
`make e2e` runs as `TestWalkthroughScreenshots`, which additionally asserts
that the app launched, every accelerator navigated somewhere, no two pages
rendered identically, each frame is non-blank, and the Bluefin-family and
Update All groups were really built.

The application always runs with `--dry-run`, so no screenshot can be produced
by a session that changed anything on the machine.

Three things the capture host cannot have are supplied by stubs compiled only
into the `chairlift_e2e` build — a Dakota image descriptor, an
unattended-update timer state, and an NVIDIA GPU — so the Bluefin-family rows,
the Automatic Updates switch, and the Graphics Driver offer render on an
ordinary workstation. No released binary contains a code path that reads them,
which `make ci` asserts.

Screenshots are **not** regenerated on every commit: font hinting and GTK
point releases move pixels, so per-push regeneration would churn the
repository for no signal. Run `make screenshots` when a feature's appearance
actually changes. `make ci` separately checks that every page has a screenshot, that this
document references it, that every supported image is named, and that every
configurable group has an entry here — so a feature cannot land undocumented,
including one added to a page that already exists.
