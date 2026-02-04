# Refine (CSR) + Clerk + Go Server Auth — Implementation Plan

## Goal
Add authentication to the Refine-based admin console (client-side rendered) with **Clerk** as the identity provider and a **Go backend** enforcing access to admin APIs.

Success criteria:
- Unauthenticated users are redirected to `/login`.
- All admin API calls require a valid Clerk session token.
- Refine can display current user identity (`getIdentity`) and (optionally) permissions/roles.

---

## Target Architecture (recommended)
**Browser (Refine CSR)**
→ user signs in via Clerk UI
→ frontend obtains **session token (JWT)**
→ sends API requests to Go backend with `Authorization: Bearer <token>`
**Go backend**
→ verifies JWT (Clerk) and attaches user claims to request context
→ serves `/me` and your admin APIs

Why this approach:
- Keeps auth enforcement on the backend.
- Avoids putting any sensitive DB access token in the frontend.
- Matches Clerk’s recommended pattern of using either `__session` cookie (same-origin) or `Authorization` header (cross-origin).  [oai_citation:0‡Clerk](https://clerk.com/docs/guides/sessions/verifying?utm_source=chatgpt.com)

---

## Phase 0 — Prep & Constraints (0.5 day)
1. **Decide request style**
   - Same-origin (admin UI + API under same site) → can use `__session` cookie approach
   - Cross-origin (different domains) → use `Authorization: Bearer <token>` (recommended default because it’s explicit)  [oai_citation:1‡Clerk](https://clerk.com/docs/guides/sessions/verifying?utm_source=chatgpt.com)

2. **Create Clerk project + keys**
   - Add env vars to admin frontend build:
     - `VITE_CLERK_PUBLISHABLE_KEY=...`
   - Add env vars to Go server:
     - `CLERK_SECRET_KEY=...` (if using Clerk Go SDK / middleware)
     - (If doing manual verification) `CLERK_JWKS_URL` or Clerk PEM public key depending on method  [oai_citation:2‡Clerk](https://clerk.com/docs/guides/sessions/manual-jwt-verification?utm_source=chatgpt.com)

---

## Phase 1 — Frontend (Refine CSR) (1 day)

### 1. Add Clerk provider to app root
Wrap your React app with Clerk’s React Router provider. (Use the package that matches your router integration.)  [oai_citation:3‡Clerk](https://clerk.com/docs/react-router/reference/components/clerk-provider?utm_source=chatgpt.com)

Deliverables:
- `src/main.tsx` or `src/App.tsx` updated to include `<ClerkProvider publishableKey={...}>`.

### 2. Add `/login` route using Clerk UI
Use Clerk’s `<SignIn />` component to render a sign-in UI fast.  [oai_citation:4‡Clerk](https://clerk.com/docs/react-router/reference/components/authentication/sign-in?utm_source=chatgpt.com)

Deliverables:
- `src/pages/login.tsx` (or route module) that renders `<SignIn />`
- Route config: `/login`

### 3. Implement Refine `authProvider`
Refine expects an `authProvider` object (async methods: `login`, `logout`, `check`, `getIdentity`, etc.).  [oai_citation:5‡Refine](https://refine.dev/core/docs/authentication/auth-provider/?utm_source=chatgpt.com)

Recommended behaviors:
- `login()` → `redirectTo: "/login"`
- `check()` → call backend `GET /api/me`:
  - 200 → `{ authenticated: true }`
  - 401/403 → `{ authenticated: false, redirectTo: "/login" }`
- `getIdentity()` → call backend `GET /api/me` and map to `{ id, name, avatar }`
- Optional: `getPermissions()` → call backend `GET /api/permissions` (or return roles from `/me`)

Deliverables:
- `src/authProvider.ts` implementing the above.

### 4. Protect routes with `<Authenticated />`
Wrap admin routes/layout with Refine’s `<Authenticated redirectOnFail="/login" />`.  [oai_citation:6‡Refine](https://refine.dev/core/tutorial/routing/authentication/react-router/?utm_source=chatgpt.com)

Deliverables:
- `src/App.tsx` (or router setup) uses `<Authenticated>` around protected routes.

### 5. Ensure API requests include token
Implement a single place to attach `Authorization` header:
- If you use `fetch`: create `apiClient.ts`
- If you use axios: add a request interceptor
- If you use Refine dataProvider: wrap its fetch client

Token retrieval:
- Get Clerk session token and add:
  - `Authorization: Bearer ${token}`

Deliverables:
- `src/apiClient.ts` that injects token into every API call (used by dataProvider and `authProvider.check()` calls).

---

## Phase 2 — Backend (Go server) (1 day)

### 1. Add auth middleware (Clerk verification)
Option A (fastest): Use Clerk Go SDK middleware that supports Bearer token verification and extracts session claims.  [oai_citation:7‡GitHub](https://github.com/clerk/clerk-sdk-go?utm_source=chatgpt.com)
Option B (manual): Verify JWT using Clerk guidance (get token from `Authorization` header / `__session` cookie).  [oai_citation:8‡Clerk](https://clerk.com/docs/guides/sessions/manual-jwt-verification?utm_source=chatgpt.com)

Deliverables:
- `middleware/auth.go`
  - Extract token
  - Verify with Clerk
  - On success: put `userID`, `sessionID`, `orgID/roles` (if any) into request context
  - On failure: `401 Unauthorized`

### 2. Add `GET /api/me`
Returns identity info for Refine `getIdentity()` and `check()`:
- `{ id, email, name, avatarUrl, roles?: [] }`

Deliverables:
- `handlers/me.go`

### 3. Protect all admin endpoints
Add middleware to admin route group:
- `/api/*` (except maybe `/api/health` if you want it public)
- Ensure 401/403 are consistent.

Deliverables:
- Route grouping with middleware applied.

### 4. Optional RBAC (role-based access control)
If “boss wants auth” also implies “only certain people can do certain actions”:
- Store role mapping in Turso (e.g., `admin_users` table: `clerk_user_id`, `role`)
- On each request: load role once (cache in memory/redis if needed) and enforce.

Deliverables:
- `db/admin_users` access layer
- `middleware/authorize.go` or per-handler checks

---

## Phase 3 — Refine Authorization (optional but common) (0.5–1 day)
If you implement roles:
- `authProvider.getPermissions()` returns `["admin"] | ["viewer"] | ...` for UI gating.
- Use Refine’s access control patterns to hide/disable resources based on permissions.

(Refine supports fetching permissions via `authProvider`—keep it backend-driven.)  [oai_citation:9‡Refine](https://refine.dev/core/docs/guides-concepts/authentication/auth-provider-interface/?utm_source=chatgpt.com)

---

## Security Checklist (must-do)
- Never expose Turso admin tokens to the browser.
- Always verify Clerk tokens on the Go backend (don’t trust frontend “logged in” state).
- Ensure HTTPS everywhere.
- Configure CORS if cross-origin; allow `Authorization` header.
- Consider Cloudflare Access as an extra perimeter layer (optional “belt & suspenders”).

---

## Testing Plan
### Frontend
- Unauthenticated visit `/` → redirects to `/login`
- After sign-in → can view protected routes
- `authProvider.check()` properly logs out / redirects on 401

### Backend
- `GET /api/me`:
  - no token → 401
  - invalid token → 401
  - valid token → 200 with correct identity
- Protected endpoints reject unauthenticated requests.

---

## Deployment Notes
- Add required env vars to:
  - Admin frontend hosting (Vercel / etc.)
  - Go server runtime (systemd / container / Vercel serverless, etc.)
- Confirm Clerk redirect URLs and allowed origins match your domains.
- Rollout strategy:
  1) Deploy backend auth middleware + `/api/me`
  2) Deploy frontend with `/login` + `<Authenticated>`
  3) Monitor 401 spikes and CORS errors

---

## Deliverables Summary (for code agent)
Frontend:
- `src/authProvider.ts`
- `src/pages/login.tsx`
- `src/apiClient.ts` (inject Bearer token)
- Router/App updates: `<ClerkProvider>` + `<Authenticated redirectOnFail="/login">`

Backend (Go):
- `middleware/auth.go` (Clerk verification)
- `handlers/me.go` (`GET /api/me`)
- Apply middleware to `/api/*`
- (Optional) `db/admin_users` + role checks + `/api/permissions`

---

## Reference Docs
- Refine authProvider interface & methods:  [oai_citation:10‡Refine](https://refine.dev/core/docs/guides-concepts/authentication/auth-provider-interface/?utm_source=chatgpt.com)
- Refine route protection with `<Authenticated />`:  [oai_citation:11‡Refine](https://refine.dev/core/tutorial/routing/authentication/react-router/?utm_source=chatgpt.com)
- Clerk React Router `<ClerkProvider>` and `<SignIn />`:  [oai_citation:12‡Clerk](https://clerk.com/docs/react-router/reference/components/clerk-provider?utm_source=chatgpt.com)
- Clerk session token verification (Go + header/cookie patterns):  [oai_citation:13‡Clerk](https://clerk.com/docs/guides/sessions/verifying?utm_source=chatgpt.com)


