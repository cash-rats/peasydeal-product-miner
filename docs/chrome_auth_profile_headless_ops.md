# Stable Chrome DevTools + Auth Sessions (Shopee/Taobao)

## Problem Summary

Your Docker crawler talks to Chrome running on the Ubuntu host via Chrome DevTools Protocol (CDP). When Chrome is left running in a long-lived VNC session, it eventually becomes unstable (e.g. shows “Aw, Snap!” / becomes unresponsive) and CDP requests time out (`Network.enable timed out`, `new_page/list_pages/evaluate_script` timeouts). You then have to reconnect via VNC and manually refresh/recover.

## Root Cause (Most Likely)

Keeping a GUI Chrome instance “open forever” is fragile:
- renderer crashes (“Aw, Snap!”) due to memory pressure/leaks or accumulated tabs/targets
- compositor/GPU issues in X11/VNC sessions over long idle periods
- general long-lived browser instability causing CDP to time out even though TCP is reachable

## Proposed Solution (High Level)

Use a **persistent Chrome profile** (cookies/session stored on disk) and run Chrome **headless for normal crawling**, but temporarily switch to **interactive mode via VNC** only when Shopee/Taobao requires manual login/CAPTCHA.

This avoids a fragile “forever-open VNC browser”, while preserving authenticated sessions across restarts.

## Design: Persistent Profile + Mode Switching

### Key idea

Run Chrome with a fixed `--user-data-dir` (profile directory). Cookies/session state live there.

Then:
- Normal operations: start Chrome **headless** with that profile.
- When auth expires/CAPTCHA: stop headless, start **interactive** with the same profile, solve it in VNC, stop, then restart headless.

### Important constraint

Do **not** run headless and interactive Chrome at the same time using the same `--user-data-dir`.

## One-Time Setup (Initial Login)

1) Choose a profile directory on the Ubuntu host (example):

- `/var/lib/peasydeal/chrome-profile`

2) Start Chrome **interactive** (VNC) using the persistent profile + CDP:

```sh
google-chrome \
  --remote-debugging-address=0.0.0.0 \
  --remote-debugging-port=9222 \
  --user-data-dir=/var/lib/peasydeal/chrome-profile \
  --profile-directory=Default
```

3) In VNC:
- open Shopee / Taobao
- log in normally
- ensure browsing works

4) Close Chrome.

Now the session cookies are saved in the profile directory.

## Normal Operation (Headless Crawling)

Start Chrome headless with the same profile:

```sh
google-chrome \
  --headless=new \
  --remote-debugging-address=0.0.0.0 \
  --remote-debugging-port=9222 \
  --user-data-dir=/var/lib/peasydeal/chrome-profile \
  --profile-directory=Default \
  --disable-gpu
```

Your Docker crawler continues talking to CDP on `:9222`, but Chrome is now:
- easier to keep stable long-term
- safe to restart automatically without losing auth

## Auth Expired / CAPTCHA Flow (Manual Intervention Procedure)

When crawling starts failing due to login/CAPTCHA:

1) Stop headless Chrome (whatever supervisor you use).
2) Start Chrome interactive (same `--user-data-dir`) via VNC:

```sh
google-chrome \
  --remote-debugging-address=0.0.0.0 \
  --remote-debugging-port=9222 \
  --user-data-dir=/var/lib/peasydeal/chrome-profile \
  --profile-directory=Default
```

3) In VNC:
- complete login / CAPTCHA

4) Close Chrome.
5) Restart headless Chrome (previous headless command).

## Reliability Enhancements (Recommended)

### 1) Watchdog / Restart Strategy

Add a small watchdog to detect CDP wedging and restart Chrome:
- CDP “looks up” checks like `/json/version` can still respond even when Chrome is unusable.
- A stronger check is to create a target:
  - `curl -fsS http://127.0.0.1:9222/json/new`

If that request times out/fails, restart Chrome.

### 2) Limit accumulation

Ensure the crawler:
- closes pages/tabs after use
- does not leak targets

### 3) Stabilizing flags (optional)

If you still see issues:
- `--disable-gpu --disable-software-rasterizer` (X11/VNC stability)
- Consider a periodic restart (e.g. nightly) even if healthy

## Operational Notes

- This approach keeps the “manual login ability” without requiring the browser to stay interactive all the time.
- Sites may invalidate sessions periodically; expect occasional VNC intervention.
- If you later want to formalize this, run Chrome under `systemd` with two unit modes:
  - `chrome-headless.service`
  - `chrome-interactive.service`
  - with mutual exclusion (only one can run at a time).

