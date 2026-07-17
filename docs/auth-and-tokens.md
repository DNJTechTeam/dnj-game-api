# Authentication & Tokens

DNJ Game API uses a single, deliberately minimal token: an **identity JWT**.
It proves *who* the user is and carries no tenant, roles or scopes. A richer
authorization model is meant to be layered on top later.

Authentication is **passwordless**. A `User` is only created the first time a
subscriber confirms the 6-digit verification code sent by email — there is no
register/login/forgot-password flow.

## Passwordless onboarding flow

1. The external event-subscription platform calls `POST /subscriptions/webhook`
   (protected by a shared secret, see below). The raw payload is stored and
   translated into one `subscription_webhook_verification_codes` row per
   participant — with a fresh 6-digit `verification_code` — keyed by
   `document` (CPF; `user_id` stays `null` at this point). Some participants
   arrive with no email at all: when an order has several participants
   sharing the buyer's email, only the buyer's row keeps it — companions are
   stored with `email` empty (see `OrderPayloadTranslator`).
2. The subscriber calls `POST /auth/onboarding` with **`document` required,
   `email` optional**. The record is looked up by `document`:
   - **Not found**: `400`, generic lookup-failed error (same shape as
     before — doesn't reveal whether the document truly doesn't exist).
   - **Found, already has an email on file**: the verification code is
     emailed to that address (ignoring any `email` sent in the request —
     the stored email always wins once set), and the response is `200`:
     `{"status": "CODE_SENT", "email": "c***a@hotmail.com"}` (obfuscated:
     first + last character of the local-part, full domain).
   - **Found, no email on file, request has no `email` either**: nothing is
     sent; response is `200 {"status": "EMAIL_REQUIRED"}`. The client is
     expected to collect the email from the user and call this same
     endpoint again with `document` + `email` filled in.
   - **Found, no email on file, request provides one**: the record is
     backfilled with that email (after a basic format check via
     `net/mail.ParseAddress`), the code is sent to it, and the response is
     `200 {"status": "CODE_SENT", "email": "<obfuscated>"}` — same as the
     "already has an email" case from then on.
3. The subscriber calls `POST /auth/verification-code` with `email` + the
   code (the same email that just received it — either the one that was
   already on file, or the one just backfilled in step 2). On the first
   successful match, a `User` is created (role `DEFAULT`) and linked back to
   the verification-code row; on subsequent calls the same `User` is reused
   (idempotent). The response includes the `identityToken`.

## The identity token

- **Type**: `auth.IdentityClaims` (`internal/infrastructure/api/auth/jwt_claims.go`).
- **Algorithm**: HS256, signed with `JWT_IDENTITY_SECRET`.
- **Lifetime**: 24 hours.
- **Claims**: `sub` — the user id. Nothing else — see "Adding authorization
  later" below.

Issued by `JwtService.GenerateIdentityToken` (`internal/app/services/jwt_service.go`)
when a verification code is confirmed.

## Header *and* cookie

`AuthenticationMiddleware` (`internal/infrastructure/api/middlewares/auth_middlewares.go`)
accepts the token from either transport:

1. The `Authorization` request header.
2. Failing that, the `identity_token` cookie.

`AuthHandler.VerifyCode` calls `apiHelpers.SetIdentityToken`, which sets
`identity_token` as an `HttpOnly` cookie. The exact same token value is also
returned in the response body for clients that prefer the header (mobile
apps, server-to-server).

### Cookie attributes (`internal/infrastructure/api/cookies.go`)

| Attribute | localhost | deployed environments |
|-----------|-----------|-----------------------|
| `HttpOnly` | true | true |
| `Secure` | false | true |
| `SameSite` | `Lax` | `None` (cross-origin SPA + API) |
| `Domain` | `COOKIE_DOMAIN` if set, else host-only |
| `Path` | `/` | `/` |

`SameSite=None` requires `Secure=true`, which is why deployed environments force
both. CORS must allow credentials and the exact frontend origin
(`CORS_ALLOWED_ORIGINS`).

## What the middleware enforces

For a route wrapped with `authProtected()` / `AuthenticationMiddleware()`:

1. A token is present and valid (signature + not expired).

On success the user id (a string) is placed in the request context under
`common.UserIDContextKey`. Services read it with
`common.ExtractUserIdFromContext(ctx)`.

## Routes (`internal/presentation/api/routers/`)

| Method & path | Auth | Purpose |
|---------------|------|---------|
| `POST /subscriptions/webhook` | `X-Webhook-Secret` header (`SUBSCRIPTION_WEBHOOK_SECRET`) | Ingest a subscription webhook, upsert a pending verification code |
| `POST /auth/onboarding` | public | Confirm document (+ email if not on file yet), (re)send the verification code |
| `POST /auth/verification-code` | public | Confirm the code, create/reuse the user, issue the identity token |
| `GET /groups` | token | Search groups by name (`?search=`, min 3 chars) |
| `POST /users/{id}/update-group` | token | Link a user to an existing or newly-created group |

## Adding authorization later

The natural extension point is `Router.authProtected()` in
`internal/presentation/api/routers/router.go` — add role/scope middleware to
that chain (`User.Role` already exists: `ADMIN`, `EVENT_MANAGER`, `DEFAULT`),
and enrich `IdentityClaims` (or introduce a second token type) as needed. As
of this version, `POST /users/{id}/update-group` is reachable by any
authenticated user — gating it to `ADMIN`/`EVENT_MANAGER` is a known
follow-up once that middleware exists.
