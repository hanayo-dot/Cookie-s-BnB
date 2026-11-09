# Implementation Roadmap

Phases are ordered by dependency and correctness risk. Each phase should ship
behind tested API contracts and migrations; unfinished integrations stay
disabled rather than pretending to succeed.

## Phase 0: Architecture and data foundation

- Document module boundaries, deployment assumptions, and security decisions.
- Add initial PostgreSQL migration for identity/tenancy, property inventory,
  reservation integrity, payments, commissions, and audit events.
- Add migration runner, API/web project scaffolds, local development compose,
  and CI checks.
- Initial progress: architecture/data docs, the PostgreSQL foundation migration,
  a Gin API bootstrap with liveness/readiness checks, HTTP tests, and a local
  PostgreSQL Compose service are in place. Migration tooling, web scaffold, and
  CI are still pending.
- Exit: a clean database can migrate from empty; overlap constraint is proven
  with concurrent/integration tests.

## Phase 1: Identity and business access

- Implement registration, login/logout, password reset, session security, role
  and permission checks, business memberships, invitations, and audit events.
- Add authorization tests for every role and cross-business access attempts.
- Initial progress: password-hashed owner registration/login, opaque persistent
  sessions, `/me`, CSRF-protected logout, exact-origin checks, and unit plus
  PostgreSQL-backed workflow verification are in place. Password recovery,
  role/permission administration, invitations, and audit events remain.
- Exit: guest, owner, agent, manager, staff, and super-admin access boundaries
  are explicit and tested.

## Phase 2: Catalog and public discovery

- Implement property/unit CRUD, approval, media adapter, amenities, pricing,
  availability search, responsive public pages, and SEO metadata.
- Add guest/customer CRM records and enquiry capture.
- Exit: approved properties can be found with server-backed filters and
  availability; uploads and validation are secured.

## Phase 3: Reservations and customer portal

- Implement atomic booking holds, confirmation, cancellation policies, booking
  history, guest checkout, email notifications via outbox, and owner/staff
  booking calendar.
- Exit: date boundaries, hold expiry, cancellation, and concurrency cases pass
  integration tests.

## Phase 4: Payments and financial operations

- Implement provider interface and Daraja sandbox adapter, STK push initiation,
  verified callbacks, reconciliation, receipts, partial/deposit payments,
  refunds, commission rules, owner payouts, expenses, and financial reports.
- Exit: no client-only payment success path; replayed callbacks are idempotent;
  amounts reconcile against provider references.

## Phase 5: Operations and partner portals

- Add owner and agent portals, property assignments, earnings statements,
  housekeeping, check-in/out, tasks, maintenance, inventory, and incident flows.
- Exit: property-scoped access and operational assignment workflows are tested.

## Phase 6: Growth, analytics, and production readiness

- Add reviews, promotions, referrals, newsletter, WhatsApp sharing, channel
  attribution, reporting dashboards, notification preferences, and optional AI
  adapters.
- Complete accessibility, performance, threat-model review, backups/restore,
  observability, deployment documentation, and production provider onboarding.
- Exit: end-to-end workflows and restore/deploy runbooks pass in staging.

## First acceptance gates

- Fresh PostgreSQL migration succeeds and constraints/indexes exist.
- A unit cannot acquire overlapping active reservations, even in concurrent
  transactions; cancelled/expired inventory becomes reservable again.
- Booking, payment, commission, and audit records are tenant-scoped and use
  explicit status transitions.
- No credentials are committed; production payment mode refuses missing
  provider verification configuration.