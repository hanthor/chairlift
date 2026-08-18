# ChairLift walkthrough

Every screen in ChairLift, captured from the real app by `make screenshots`
(see [below](#how-these-are-made)) — not mockups.

One app for Snow Linux and the Bluefin family (Bluefin, Bluefin LTS, Dakota).
Features ported from [bluefinctl](https://github.com/projectbluefin/bluefinctl)
and [finupdate](https://github.com/tuna-os/finupdate) come down to one control
per decision — no strategy pickers, no schedule choosers, no feature grids.

---

## Updates

![Updates](screenshots/3-updates.png)

**Update All** updates the OS image, Flatpak apps, and Homebrew packages in
one click, with per-phase status and a notification when it finishes. It asks
for a restart only if an image was actually staged. **Automatic Updates** is
one switch for background updates. The per-provider Flatpak and Homebrew
groups below still work on their own.

**System Updates** adds **Roll Back**, naming the previous deployment, and
**What's Changing**, a package-by-package diff of the running and staged
images. It appears on bootc hosts only, so it is missing from the screenshot
above.

---

## System

![System](screenshots/4-system.png)

Image and deployment status, `os-release` details, and a Mission Center link.
Two switches change which image you run: **Release Channel** (stable/testing)
and **Graphics Driver** (the matching NVIDIA image, when your card needs it
and the image exists for your stream). Neither is offered when there is
nothing to switch to — see
[ADR-0011](adr/0011-chairlift-owns-bluefin-family-rebasing.md).

---

## Features

![Features](screenshots/5-features.png)

**Developer Mode** joins the container/VM/serial-device groups — it does not
rebase you to a `-dx` image. **Gaming Mode** installs Steam, Proton tooling,
and a few extras as user Flatpaks, no admin password needed. **Local AI**
serves a language model on `localhost:8080` from a rootless container, using
whichever GPU you have — the first start downloads several GB. **System
Features** is Snow Linux's updex manager, empty elsewhere.

---

## Applications

![Applications](screenshots/1-applications.png)

Installed Flatpaks and Homebrew formulae/casks, search across both, **Bundles**
for one-click package sets, and per-user tap trust for untrusted Homebrew
taps.

---

## Maintenance

![Maintenance](screenshots/2-maintenance.png)

Homebrew and Flatpak cleanup, plus any maintenance actions the image
maintainer configured. **Reset** holds the two irreversible ones, each behind
a confirmation dialog: Powerwash removes every user Flatpak and Distrobox
container, Factory Reset reinstalls the current image. It ships off, and is
switched on above so you can see it.

---

## Help

![Help](screenshots/6-help.png)

Links to the distribution's website, issues, and discussions.

---

## How these are made

```bash
make screenshots
```

Builds the app, runs it headless under Xvfb, navigates every page, and writes
one cropped PNG per page to `docs/screenshots/`. Always `--dry-run`, so
nothing on the capture machine changes. What the capture host can't have — a
Bluefin image, an NVIDIA GPU, an active update timer — comes from stubs
compiled only into the `chairlift_e2e` build, which `make ci` checks no
released binary can read.

Run it when a feature's appearance changes; it is not regenerated per commit,
since font and theme drift would churn the repo. `make ci` checks that every
page and configurable group has a screenshot and an entry here.
