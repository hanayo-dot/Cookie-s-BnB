# API Reference

Base path: `/api/v1`. Requests and responses use JSON unless noted. Errors use
`{"error":{"code":"..."}}`. The API currently exposes health checks and the
initial identity workflow; property, booking, and payment routes are not yet
implemented.

## Health

| Method and path | Purpose | Success |
| --- | --- | --- |
| `GET /healthz` | Process liveness; does not query dependencies. | `200 {"status":"ok"}` |
| `GET /readyz` | Readiness; requires PostgreSQL connectivity. | `200 {"status":"ready"}` |

## Authentication

| Method and path | Purpose | Success |
| --- | --- | --- |
| `POST /auth/register` | Creates a user, business, active owner membership, and session atomically. | `201 {"user":...}` |
| `POST /auth/login` | Verifies credentials and creates a new session. | `200 {"user":...}` |
| `GET /auth/me` | Returns the authenticated user and active business memberships. | `200 {"user":...}` |
| `POST /auth/logout` | Revokes the current session. Requires CSRF token and configured origin. | `204` |

Registration request:

```json
{
  "email": "owner@example.com",
  "full_name": "Amina Owner",
  "password": "a-password-with-at-least-12-characters",
  "business_name": "Coast Stays"
}
```

Login request:

```json
{
  "email": "owner@example.com",
  "password": "a-password-with-at-least-12-characters"
}
```

The API sets an HttpOnly `bnb_session` cookie and a readable `bnb_csrf` cookie.
Browser clients must send cookies with credentialed requests. For logout, copy
the CSRF cookie value into the `X-CSRF-Token` header. The API also requires the
exact configured `APP_ORIGIN` on mutating requests. Local development defaults
to `http://localhost:3000`; production must configure the public browser origin,
TLS, and `COOKIE_SECURE=true`.

Common error codes: `invalid_request` (`400`), `invalid_input` (`400`),
`email_unavailable` (`409`), `invalid_credentials` (`401`),
`unauthenticated` (`401`), `csrf_forbidden` (`403`), `origin_forbidden` (`403`),
and `internal_error` (`500`).

Password reset, access/refresh token rotation, account verification, and
role/permission administration are planned, not implemented.