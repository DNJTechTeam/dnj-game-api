# Authentication & Tokens

Nucleus API uses a single, deliberately minimal token: an **identity JWT**. It
proves *who* the user is and carries no tenant, roles or scopes. A richer
authorization model is meant to be layered on top later.

## The identity token

- **Type**: `auth.IdentityClaims` (`internal/infrastructure/api/auth/jwt_claims.go`).
- **Algorithm**: HS256, signed with `JWT_IDENTITY_SECRET`.
- **Lifetime**: 24 hours.
- **Claims**:
  - `sub` — the user id.
  - `hasUpdatedPassword` — whether the user has set a definitive password.
  - `emailConfirmed` — whether the email is confirmed.

Issued by `JwtService.GenerateIdentityToken` (`internal/app/services/jwt_service.go`)
on successful login.

## Header *and* cookie

`AuthenticationMiddleware` (`internal/infrastructure/api/middlewares/auth_middlewares.go`)
accepts the token from either transport:

1. The `Authorization` request header.
2. Failing that, the `identity_token` cookie.

On login, `UserAuthHandler.Login` calls `apiHelpers.SetIdentityToken`, which
sets `identity_token` as an `HttpOnly` cookie. The exact same token value is
also returned for clients that prefer the header (mobile apps, server-to-server).

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

For a route wrapped with `authProtected()` / `AuthenticationMiddleware(nil)`:

1. A token is present and valid (signature + not expired).
2. `hasUpdatedPassword` is true — unless the route opts out with
   `AuthMiddlewareCustomizer{SkipPasswordUpdatedValidation: true}`.
3. `emailConfirmed` is true — unless the route opts out with
   `SkipEmailConfirmedValidation: true`.

On success the user id (a string) is placed in the request context under
`common.UserIDContextKey`. Services read it with
`common.ExtractUserIdFromContext(ctx)`.

The customizer is what lets `/auth/confirm-email`, `/auth/resend-confirmation`
and `/auth/change-password` run *before* the user is fully onboarded.

## Auth routes (`internal/presentation/api/routers/auth_router.go`)

| Method & path | Auth | Purpose |
|---------------|------|---------|
| `POST /auth/register` | public | Create a user, send the confirmation email |
| `POST /auth/login` | public | Validate credentials, issue the identity token |
| `POST /auth/forgot-password` | public | Email a temporary password |
| `POST /auth/confirm-email` | token (email check skipped) | Confirm the email |
| `POST /auth/resend-confirmation` | token (email check skipped) | Resend the link |
| `PATCH /auth/change-password` | token (both checks skipped) | Set a definitive password |
| `POST /auth/logout` | token (both checks skipped) | Clear the cookie |

## Passwords

Passwords are hashed with bcrypt in the `User` GORM hooks
(`internal/infrastructure/db/models/user.go`): `BeforeCreate` always hashes;
`BeforeUpdate` re-hashes only when the value is not already a bcrypt hash.

## Email-confirmation token

Separate from the JWT: `EmailConfirmationService` issues an HMAC-SHA256 signed,
base64 payload (`userID|email|expiresAt|issuedAt`) valid for 96 hours, signed
with `EMAIL_CONFIRMATION_SECRET`.

## Adding authorization later

The natural extension point is `Router.authProtected()` in
`internal/presentation/api/routers/router.go` — add role/scope middleware to
that chain, and enrich `IdentityClaims` (or introduce a second token type) as
needed.
