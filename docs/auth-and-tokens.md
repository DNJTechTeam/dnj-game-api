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
   translated into a `subscription_webhook_verification_codes` row — with a
   fresh 6-digit `verification_code` — keyed by `email` (`user_id` stays
   `null` at this point).
2. The subscriber calls `POST /auth/onboarding` with `email` + `document`. If
   they match a pending record, the verification code is emailed to them
   (PT-BR, via `EmailServiceInterface.SendVerificationCodeEmail`).
3. The subscriber calls `POST /auth/verification-code` with `email` + the
   code. On the first successful match, a `User` is created (role `DEFAULT`)
   and linked back to the verification-code row; on subsequent calls the
   same `User` is reused (idempotent). The response includes the
   `identityToken`.

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
| `POST /auth/onboarding` | public | Confirm email+document, (re)send the verification code |
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
