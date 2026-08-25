# Authentication & tokens

The V2 identity contract lives under `/v2/auth`. V1 passwordless email routes
remain available while their consumers migrate; the new flow does not change
the frontend repository.

## Frontend: obtaining the Google ID token (GIS)

`POST /v2/auth/google` needs a real Google ID token as input — the frontend
gets it from **Google Identity Services (GIS)**, not from this API:

```html
<script src="https://accounts.google.com/gsi/client" async defer></script>
```

```ts
google.accounts.id.initialize({
  client_id: GOOGLE_CLIENT_ID, // same value as the backend's GOOGLE_CLIENT_ID env var
  callback: (response) => {
    // response.credential IS the idToken — send it as-is to POST /v2/auth/google
    apiFetch("/auth/google", { method: "POST", body: JSON.stringify({ idToken: response.credential }) });
  },
});
google.accounts.id.renderButton(buttonEl, { theme: "outline", size: "large" });
// or, for One Tap instead of a button:
google.accounts.id.prompt();
```

Setup in Google Cloud Console (OAuth client, type **Web application**):
add every frontend origin that will call `initialize` (e.g.
`https://app.example.com`, `http://localhost:3000` for local dev) to
**Authorized JavaScript origins**. GIS uses postMessage/FedCM, not a
redirect — unlike the OAuth Playground trick used for manual/QA testing (see
the testing guide), no redirect URI is needed for the real integration.

## Google login and account linking

`POST /v2/auth/google` receives `{ "idToken": "..." }`. The backend uses the
official Google verifier and accepts the token only when all of these checks
pass: signed Google certificate, `iss` in the Google allowlist, `aud` equal to
`GOOGLE_CLIENT_ID`, valid expiration, non-empty subject and email, and
`email_verified=true`. Tests replace the verifier and never call Google.

Linking follows these rules, in order:

1. `(provider=google, subject)` already linked: the token email must still equal
   both the stored identity email and user email; any difference returns `409`.
2. New subject with an existing email: link only to that exact, previously
   verified email account. Existing V1 accounts reached this state through the
   email verification-code flow.
3. New subject and email: create a `DEFAULT` user with incomplete onboarding.
4. Database uniqueness on `(provider, subject)`, user email and CPF hash closes
   concurrent linking races; a conflict never silently changes ownership.

## Cadastro/login por email (V2)

`POST /v2/auth/signup` sends a 6-digit code to the given email — self-service,
no pre-existing record required (unlike the V1 flow below, which only works
for a document a partner webhook already pushed in; that webhook never
shipped, so V1 onboarding is effectively dead for anyone new). Always
responds `200 {"status":"CODE_SENT"}`, even when the email already owns an
account, so the endpoint can't be used to enumerate registered emails. A
resend before the 60-second cooldown responds `429 RATE_LIMITED`.

`POST /v2/auth/signup/verify` (`{"email":"...","code":"042917"}`) confirms
the code and returns the exact same `IdentitySessionResponse` shape as
`/v2/auth/google` — same cookies, same `onboardingRequired` semantics — so a
frontend that already integrated Google login reuses its session-handling
code unchanged. The first successful verification for an email creates a
`DEFAULT` user with incomplete onboarding (same as a fresh Google signup);
verifying an email that already owns an account (Google or a prior signup)
links to it instead of creating a duplicate. The code expires after 15
minutes and locks after 5 wrong attempts; both cases return `401
INVALID_CODE` without revealing which reason applied.

## Session lifecycle

- Access token: HS256 JWT, issuer `dnj-game-api`, audience `dnj-v2`, lifetime
  15 minutes, returned in the response and in `identity_token`.
- Refresh token: opaque random value, lifetime 30 days, sent only as
  `refresh_token`; the database stores SHA-256 of the value, never the token.
- Refresh rotation: every successful `POST /v2/auth/refresh` creates a new token
  and revokes the previous row while preserving the family id.
- Reuse detection: using a revoked token revokes the whole family and returns
  `401 REFRESH_TOKEN_REUSE`.
- Logout: `POST /v2/auth/logout` revokes the current family and expires all
  identity cookies. It is idempotent when the refresh token is absent/unknown.

`GET /v2/auth/session` accepts `Authorization: Bearer <JWT>` first and falls
back to `identity_token`. Validation requires HS256, issuer, audience and
expiration. It returns the current user and `onboardingRequired`.

## Cookies and CSRF

| Cookie | HttpOnly | Path | Lifetime |
|---|---:|---|---:|
| `identity_token` | yes | `/` | 15 minutes |
| `refresh_token` | yes | `/v2/auth` | 30 days |
| `csrf_token` | no | `/v2/auth` | 30 days |

On localhost cookies are `SameSite=Lax` without `Secure`. Published
environments force `Secure` and `SameSite=None`; `COOKIE_DOMAIN` is optional.
Refresh and logout use double-submit CSRF: the client reads `csrf_token` and
sends the exact value as `X-CSRF-Token`. CORS must allow credentials and the
specific origins in `CORS_ALLOWED_ORIGINS`.

## Incomplete profile and onboarding

A Google account is incomplete until CPF, mobile phone and group are present.
`PATCH /v2/auth/onboarding` validates CPF digits/checksum, mobile format and an
existing group. The CPF is stored as HMAC-SHA256 using
`DOCUMENT_HMAC_SECRET`, plus only its last four digits for masking. V2 never
returns the full CPF. Duplicate CPF ownership returns
`409 DOCUMENT_ALREADY_LINKED`.

Legacy users are preserved. The expand migration adds nullable secure fields;
no plaintext CPF is removed during this compatibility phase. Those users can
complete the V2 onboarding to populate the hash. Dropping the legacy
`users.document` column is the contract migration enabler after all V1
consumers migrate.

## V2 operations and examples

```http
POST /v2/auth/google
Content-Type: application/json

{"idToken":"<google-id-token>"}
```

```json
{
  "accessToken": "<15-minute-jwt>",
  "tokenType": "Bearer",
  "expiresIn": 900,
  "csrfToken": "<same-value-as-csrf-cookie>",
  "onboardingRequired": true,
  "user": {
    "id": "42",
    "email": "ana@example.com",
    "name": "Ana",
    "mobilePhone": "",
    "documentMasked": "",
    "role": "DEFAULT",
    "group": null,
    "onboardingComplete": false
  }
}
```

```http
POST /v2/auth/refresh
Cookie: refresh_token=<opaque>; csrf_token=<csrf>
X-CSRF-Token: <csrf>
```

```http
PATCH /v2/auth/onboarding
Authorization: Bearer <access-token>
Content-Type: application/json

{"document":"52998224725","mobilePhone":"5541999990000","groupId":"12"}
```

The frontend integration enabler is intentionally explicit: configure the
Google client with the same client id, send its ID token to `/v2/auth/google`,
include credentials on cookie requests, persist the returned CSRF value only
in memory, and route `onboardingRequired=true` to the completion screen. No
frontend contract is considered delivered until that consumer work is done.

## Maintenance

- Rotate `JWT_IDENTITY_SECRET` and `DOCUMENT_HMAC_SECRET` through the runtime
  secret store; changing either invalidates existing access tokens or CPF
  equality respectively, so rotation needs a planned dual-key migration.
- Keep `GOOGLE_CLIENT_ID` equal to the deployed frontend client audience.
- Investigate `REFRESH_TOKEN_REUSE` as a security event; never log tokens, ID
  tokens, CPF, cookie values or raw Google claims.
- Keep every published status synchronized between
  `docs/openapi/dnj-v2.openapi.yaml`, its operation manifest and automated tests.
- Profile, current-group, membership and invite authorization are documented in
  `docs/profile-and-groups.md`. The frontend integration remains an explicit
  final-stage enabler; this backend iteration does not alter the frontend.

## Preserved V1 passwordless flow

The existing `/v1/auth/onboarding` and `/v1/auth/verification-code` routes
continue to use subscription verification codes and issue the same short-lived
identity JWT. They remain compatibility endpoints and are not part of the V2
OpenAPI contract.
