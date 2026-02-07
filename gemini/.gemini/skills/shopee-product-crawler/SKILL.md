---
name: shopee-product-crawler
description: Crawl a Shopee product page via Chrome DevTools MCP and return exactly one contract-compliant JSON object.
---

# Shopee Product Crawler

The runtime prompt provides the target URL. Use that URL as the navigation target.

You are controlling an already-running Google Chrome via Chrome DevTools.

Goal: extract the product title, sale price, and product description from a Shopee product page as fast as possible.

Chrome crash recovery (Aw, Snap! / renderer crash) — MUST follow when using Chrome DevTools MCP:
Goal: recover from Chrome "Aw, Snap!" / renderer crash using Chrome DevTools MCP.
Steps:
1) List all open pages/tabs (targets).
2) Select the target whose URL matches the intended site (or the active tab).
3) Run ONE evaluate_script in that tab to return:
   - href: location.href
   - title: document.title
   - ready: document.readyState
4) If href starts with "chrome-error://" OR title contains "Aw, Snap":
   a) Try Page.reload(ignoreCache=true) (DevTools: navigate_page type=reload ignoreCache=true).
   b) After reload, evaluate again.
   c) If still error page, Page.navigate to the desired URL (use stored last_good_url).
5) If reload/navigate fails or the tab becomes unresponsive:
   - open a new tab, navigate to last_good_url, then close the broken tab.
6) Return status ok / needs_manual / browser_dead (if cannot talk to CDP).

Performance rules (important):
- Minimize tool round-trips: prefer 1 navigation + 1 evaluate_script call total.
- Do NOT scroll the whole page, do NOT take screenshots unless blocked.
- Do NOT scan the entire DOM; use a few targeted selectors + meta tags.
- Do NOT call `fetch`, `XMLHttpRequest`, or any Shopee APIs. Only read already-present DOM/HTML.

Task:
(-1) Preflight: reload the first existing tab before doing anything else (to recover from stale/"Aw, Snap!" tabs):
   - Call DevTools `list_pages`.
   - If there is at least one page, select the FIRST page and reload it:
     - DevTools `select_page` on the first page
     - DevTools `navigate_page` with type=`reload`
   - If there are no pages, skip this preflight.
0) Open a new tab via DevTools (`new_page`). Record the returned page index; call it `pageIdx`.
1) Navigate to this Shopee product URL: <TARGET_URL_FROM_RUNTIME_PROMPT> (do this in the new tab you just opened).
2) Wait until the page is ready:
   - document.readyState === "complete", AND
   - EITHER JSON-LD Product is present (script[type="application/ld+json"] containing "@type":"Product") OR a price string (e.g. starting with $) is visible in the body.
   - Note: Shopee TW is an SPA and may inject JSON-LD several seconds after readyState is complete. If not found initially, wait up to 3-5 seconds.
3) Extract data in ONE evaluate_script (keep it simple and fast):
   - Detect blocking walls early:
     - If you see login / verification / CAPTCHA (e.g., "登入", "登錄", "驗證", "captcha", "robot", slider verification) AND the main product content (title/price) is NOT visible,
       return status="needs_manual" with notes describing what you saw.
   - Title (prefer stable sources, in this order):
     1) `meta[property="og:title"]`
     2) visible `main h1` (fallback `h1`)
     3) `document.title` (last resort)
     - Clean title: trim + collapse repeated whitespace.
   - Description (prefer stable sources, in this order):
     1) JSON-LD `script[type="application/ld+json"]` where `@type` includes `"Product"` -> `description`
     2) `meta[property="og:description"]` (fallback `meta[name="description"]`)
     3) A targeted DOM node for product description (e.g. `[class*="product-detail"]`, `[class*="description"]`)
     - Clean description: trim; keep newlines; collapse excessive whitespace.
   - Price (prefer stable sources, in this order):
     1) JSON-LD `script[type="application/ld+json"]` where `@type` includes `"Product"` -> `offers`
        - Handle both `Offer` and `AggregateOffer`
        - Prefer: `offers.price` (if present)
        - Fallback: `offers.lowPrice` / `offers.highPrice` (use min when range)
        - Normalize: strip currency symbols/commas/spaces; keep digits + optional decimal
     2) DOM selectors for the current sale price:
        - Search for elements containing "$" or "NT$" with font-size > 20px.
        - Try containers like `[class*="page-product"]`, `[class*="product-briefing"]`.
        - Avoid crossed-out prices: ignore text inside `<del>/<s>` and ignore elements with computed `textDecorationLine` containing `"line-through"`.
     3) `meta[property="product:price:amount"]` (fallback `meta[itemprop="price"]`, `meta[property="og:price:amount"]`)
   - If the page is not blocked and you still cannot get a non-empty `title` OR a valid `price` OR a non-empty `description`,
     return status="error" with a short reason (what was missing).
4) ALWAYS close the tab you opened in step (0) via DevTools (`close_page` with `pageIdx`), regardless of success or failure.
   - This is mandatory for `ok`, `needs_manual`, and `error` results.
   - Treat it as a finalization step: before returning output, close `pageIdx` if it still exists.

Output contract (JSON ONLY, no markdown):
{
  "url": "string",
  "status": "ok | needs_manual | error",
  "captured_at": "ISO-8601 UTC timestamp",
  "notes": "string (required when status=needs_manual)",
  "error": "string (required when status=error)",
  "title": "string",
  "description": "string",
  "currency": "string (e.g. TWD)",
  "price": "number or numeric string",
  "images": ["string"] (optional; empty array allowed),
  "variations": [
    {
      "title": "string",
      "position": "int",
      "image": "string"
    }
  ]
}

Rules:
- If status="ok": include title, description, currency, and price. images is optional (empty array allowed).
- If status="needs_manual": include notes describing the wall.
- If status="error": include error (string) with the failure reason.
- Output must be EXACTLY ONE JSON object and NOTHING ELSE (no prose, no markdown fences like ```json).
- Do NOT output placeholder type-descriptions like "string" as values; output real values with correct JSON types.
- Do NOT output empty strings for currency/price. If missing, omit those keys and set status="error" (or "needs_manual" if blocked).
- Normalize price:
  - Strip currency symbols, commas, spaces; keep only digits and an optional decimal point.
  - If the page shows a range, use the minimum.
 - Currency:
   - Prefer JSON-LD `offers.priceCurrency` (if present).
   - Fallback to `meta[property="product:price:currency"]` (or `meta[property="og:price:currency"]`).
   - If you still cannot find a currency but the site is Shopee TW, use `"TWD"` as a last resort.

Image extraction (overlay thumbnails):
- Do NOT call `fetch`, `XMLHttpRequest`, or any Shopee APIs; do NOT execute site-native functions/SDKs; only read already-present DOM/HTML.
- Fast path (preferred): use the OVERLAY as the source of truth. Do not try to “scroll the thumbnail strip” on the main page.
- Open the product image viewer overlay by clicking the main product image (DevTools `click` on the main image area).
  - Expected behavior: a modal appears with `role="dialog"` and page scrolling becomes disabled (`body` overflow becomes `hidden`).
  - Stale Element Recovery: If `click` fails or times out, take a fresh `take_snapshot` to get updated UIDs, or use `evaluate_script` to trigger `element.click()` via JavaScript.
- In ONE `evaluate_script`, extract ALL product images from the overlay dialog DOM (including off-screen/hidden thumbnails):
  - Locate the modal root: `const dialog = document.querySelector('[role=\"dialog\"]')` (fallback: `[aria-modal=\"true\"]` / `.shopee-modal`).
  - Collect candidate thumbnail images from within the dialog: `dialog.querySelectorAll('img')`.
  - Read `src` safely: `img.currentSrc || img.src || img.getAttribute('data-src') || img.getAttribute('data-lazy')`.
  - Keep only Shopee CDN images: `src` contains `susercontent.com/file/`.
  - Keep only thumbnail-sized images (overlay thumbs are typically ~80x80): bounding box width/height between ~50 and ~120.
  - Deduplicate by base URL: strip `@resize_...` suffix from `src` (treat those as the same image).
  - Filter out non-product duplicates/noise:
    - Prefer the DOMINANT gallery “series” among thumbnails (majority vote on the `/file/<prefix>` portion), and drop outliers.
  - If multiple series share the same numeric block (e.g. `sg-11134201` + `tw-11134201`), keep them together.
- Fallback validation (only if needed): if thumbnails look polluted (e.g. a thumbnail that doesn’t change the large image),
  click each thumb INSIDE THE OVERLAY (normal `.click()`) and collect the large displayed image URL after each click; keep the set of unique large images.
  - Avoid calling any network APIs; rely only on what the overlay already renders.

Variation extraction (fast DOM strategy, no massive DOM scan):
- Goal: identify product variation options (e.g., “款式/顏色/規格”) quickly using local, stable structures.
- Do NOT use `document.body.innerText` or scan thousands of nodes. Use narrow selectors.
- Preferred anchor: the product briefing area. Variation buttons are typically rendered in a small clustered row/column.

Selectors and heuristics (in order):
1) Find variation groups by searching for a heading or label with text like “規格”, “款式”, “顏色”, “樣式”.
2) Target variation button groups by class patterns:
   - Group containers:
     - `div.flex.items-center.j7HL5Q`
     - `[class*="product-briefing"] [class*="variation"]`
     - `[class*="product-variant"]`
     - `[class*="product-variation"]`
   - Buttons inside the group:
     - `button.sApkZm`
     - `button` or `[role="button"]`
3) Accept a group only if it has at least 2 option buttons.
4) Option text constraints:
   - Trim, collapse whitespace.
   - Keep text length <= 50 characters.
5) Exclude non-option UI:
   - Skip elements inside `del/s` or with `text-decoration: line-through`.
   - Skip text matching generic UI words like “加入購物車”, “直接購買”, “聊聊”, pagination digits, or ratings.

Practical DOM signal (observed on Shopee TW):
- Variation buttons often use classes like:
  - Unselected: `button.sApkZm.SkhBL1.selection-box-unselected`
  - Selected: `button.sApkZm.rXJ8VU.uGKJch`
- The button group container can be `div.flex.items-center.j7HL5Q`.
- These are not guaranteed stable, but can be used as a fast path.

Group labeling:
- Try to find a nearby label for the group:
  - `group.querySelector('label')`
  - Or previous sibling text node/span with short text (<= 20 chars).
- If label is missing, return `null` for group name.

Minimal evaluate_script pattern (single call recommended):
- Find groups using the selectors above.
- For each group, extract `options = Array.from(group.querySelectorAll('button, [role="button"]'))`.
- Filter options to remove empty or too-long text, and UI noise.
- Return a list:
  - `[{ name: string|null, options: string[] }, ...]`

Output note:
- You do NOT need to capture variation stock or price deltas here—only the available option strings.

Variation image mapping (hover-based, main image updates):
- Goal: map each variation option to the main product image shown when hovering over that option.
- Key observation (Shopee TW): hovering a variation button (規格 options) updates the main image without opening a modal.
- Do NOT scan the whole DOM; reuse the variation buttons you already found.

Steps (minimal, deterministic):
1) Locate the main product image element once:
   - Prefer: `img[alt^="Product image"]`
   - Fallback: `img[alt*="Product image"]`, or `[class*="product-briefing"] img`
2) Locate the variation buttons under the "規格" heading:
   - Find heading with text `規格`
   - Use its parent as a scope; collect `button` nodes with non-empty text.
3) For each variation button (in DOM order):
   - Hover the button (DevTools `hover` on the element).
   - If `hover` does not update the main image, use `click()` on the button (DevTools `click` or JS `click()`).
   - Immediately read `mainImage.currentSrc || mainImage.src || mainImage.getAttribute('data-src')`.
   - Record `{ title: buttonText, position: index, image: mainImageSrc }`.
   - Note: It is normal for different variations to share the same image (sellers often reuse them); record whatever the UI displays.
4) If the image still does not change after multiple attempts, report that variation image mapping is unavailable for this page.

Implementation notes:
- Prefer DevTools `hover` tool for real hover behavior (works when JS relies on pointer events).
- Use a small delay only if necessary (avoid long waits).
- Keep `position` as 0-based index in the buttons list.
- Deduplicate only if two different options produce the same image; keep both unless instructed otherwise.
