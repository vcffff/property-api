# PropertyVisit API contract (Flutter MVP)

Base URL: `http://localhost:8080/api/v1`. Use the API host reachable from the
device; Android emulator typically uses `http://10.0.2.2:8080/api/v1`.
All routes below are registered in `internal/routes/routes.go`.

## Conventions

- JSON request header: `Content-Type: application/json`.
- Protected routes: `Authorization: Bearer <access_token>`.
- Admin routes additionally require JWT role `admin`; other authenticated roles
  receive **403**. Missing, malformed or expired JWT returns **401**.
- Errors: `{"error":"message"}`. Do not depend on validator error text.
- IDs are JSON integers. New viewing status URL IDs must be positive signed
  PostgreSQL bigint values. Clients should use IDs returned by the server.
- Dates: `YYYY-MM-DD` in Astana (UTC+05:00). Instants: RFC3339 strings with a
  timezone. Available-slot `starts_at` values are always UTC (`Z`).
- Lists are bare arrays, not paginated envelopes. Viewing/application empty
  lists are `[]`. No viewing list query/body parameter selects another owner.
- Existing global Redis limiter allows **5 requests per IP per minute**, across
  routes. **429**: `rate limit exceeded`; limiter failure can return **500**.
  Pace manual tests; tests of routes do not install the production limiter.

## Route inventory

Paths here are relative to the base URL. `JWT` means either user or admin.
All protected routes can also return 401; admin routes can also return 403.
Unexpected persistence errors generally return 500 (legacy exceptions below).

| Method | Path | Access | Success | Other handler/service statuses |
|---|---|---|---|---|
| POST | `/register` | Public | 201 user summary | 400 validation/registration failure |
| POST | `/login` | Public | 200 tokens + role | 400 invalid body, 401 login failure |
| POST | `/auth/refresh` | Public | 200 rotated tokens | 400 invalid body, 401 refresh failure |
| POST | `/auth/logout` | Public | 204, no body | 400 invalid body, 500 |
| GET | `/users/me` | JWT | 200 user summary | 404 lookup failure |
| GET | `/properties` | JWT | 200 Property[] | 500 |
| GET | `/properties/:id` | JWT | 200 Property | 400 parse failure, 500 lookup failure |
| POST | `/properties` | Admin | 201 Property | 400 JSON failure, 500 DB/Kafka failure |
| PATCH | `/properties/:id` | Admin | 200 Property | 400 invalid ID/body, 404 missing, 500 |
| DELETE | `/properties/:id` | Admin | 204, no body | 400 invalid ID, 404 missing, 500 |
| POST | `/applications` | JWT | 201 Application | 400 body, 404 property missing, 409 active duplicate, 500 |
| GET | `/applications/me` | JWT | 200 own Application[] | 500 |
| GET | `/admin/applications` | Admin | 200 all Application[] | 500 |
| PATCH | `/admin/applications/:id/status` | Admin | 200 Application | 400 ID/status, 404 missing, 409 transition, 500 |
| POST | `/viewings` | JWT | 201 Viewing | 400 ID/body/time, 404 property missing, 409 occupied slot, 500 |
| GET | `/viewings/me` | JWT | 200 own Viewing[] | 500 |
| GET | `/admin/viewings` | Admin | 200 all Viewing[] | 500 |
| PATCH | `/admin/viewings/:id/status` | Admin | 200 Viewing | 400 ID/body/unknown status, 404 missing, 409 transition, 500 |
| GET | `/properties/:id/available-slots?date=YYYY-MM-DD` | JWT | 200 AvailableSlotsResponse | 400 ID/date, 404 property missing, 500 |

Legacy behavior is documented rather than changed: GET property by ID currently
returns 500 for a missing record. Property creation can return 500 after the DB
insert if Kafka publication fails; check the property list before retrying.

### Development-only route (not a Flutter production feature)

`PATCH /dev/users/me/role`, JWT required, body `{"role":"admin"}` or
`{"role":"user"}`. Registered **only** when both `APP_ENV=development` and
`ENABLE_DEV_ROLE_ENDPOINT=true`; otherwise 404. Success 200 returns
`id`, `role`, `access_token`, `token_type`, `expires_in`; errors 400/401/404/500.
Never enable on a public deployment. Older tokens/sessions retain their roles.

## DTOs and requests

### Authentication

Register and login use the same body (registered credentials for login):

```json
{"email":"user@example.com","password":"ExamplePassword123!"}
```

Email must be valid; password minimum length is 6. Registration returns
`id`, `email`, `role`, `created_at`, `updated_at`, **not tokens**.
`GET /users/me` returns only `id`, `email`, `role`.

Login response:

```json
{
  "access_token":"<jwt>",
  "refresh_token":"<opaque-token>",
  "token_type":"Bearer",
  "expires_in":900,
  "role":"user"
}
```

`POST /auth/refresh` and `POST /auth/logout` take:

```json
{"refresh_token":"<refresh-token>"}
```

No Authorization header is required for these two routes. Refresh consumes the
old refresh token and returns a new pair plus `token_type` and `expires_in`
(no `role` field). Replace **both** stored tokens. Logout revokes the supplied
refresh session, not already-issued access tokens. Access tokens last 15 minutes.

### Property

Property response / POST request fields:

```json
{"id":2,"title":"Apartment","address":"Astana","price":45000000}
```

For creation omit `id`. PATCH accepts any subset of `title`, `address`, `price`;
price is an integer. Existing Property CRUD validation has not been redesigned.

### Application

POST body: `{"property_id":2}`. Ownership comes from JWT and initial status is
`pending`; submitted `user_id`/`status` cannot override them.

```json
{
  "id":8,"user_id":11,"property_id":2,"status":"pending",
  "created_at":"2030-10-01T09:00:00Z","updated_at":"2030-10-01T09:00:00Z"
}
```

Admin status body: `{"status":"approved"}` or `{"status":"rejected"}`.
Only `pending` may transition; other states return 409. Repeated POST by the same
user/property while an application is pending/approved returns 409. Lists order
by `created_at DESC, id DESC`.

### Viewing

POST body (use an existing property and a future slot):

```json
{"property_id":2,"starts_at":"2030-10-24T10:00:00+05:00"}
```

Response:

```json
{
  "id":12,"user_id":11,"property_id":2,
  "starts_at":"2030-10-24T05:00:00Z","status":"pending",
  "created_at":"2030-10-01T09:00:00Z","updated_at":"2030-10-01T09:00:00Z"
}
```

- Statuses: `pending`, `confirmed`, `cancelled`, `completed`.
- Ownership is taken only from verified JWT; client `user_id` is ignored.
- Booking always sets `pending`; client status is ignored.
- Property must exist. Starts must be strictly in the future.
- Duration is 30 minutes, with starts every 30 minutes from **10:00 through
  18:30**, ending no later than **19:00 Astana UTC+05:00**. No fractional seconds.
- Different timestamp offsets representing the same instant identify the same slot.
- Unique PostgreSQL partial index `idx_viewings_active_slot` on
  `(property_id, starts_at)` protects `pending`/`confirmed` slots across **all users**.
  `cancelled`/`completed` rows do not occupy active slots. This does not override
  the future-time requirement when booking again.
- `/viewings/me` returns only the JWT owner's rows, including for an admin.
  Query/body `user_id` cannot select another account.
- `/admin/viewings` returns all owners' rows. Both lists order by
  `starts_at DESC, id DESC` and return `[]` when empty.

Admin PATCH body: `{"status":"confirmed"}`, `{"status":"cancelled"}` or
`{"status":"completed"}`.

| Current | Allowed target |
|---|---|
| pending | confirmed, cancelled |
| confirmed | completed, cancelled |
| cancelled | none |
| completed | none |

Known but disallowed transitions (including same-state updates or target
`pending`) return **409**. Missing/unknown/malformed status returns **400**.
Valid status + nonexistent viewing ID returns **404**. There is no additional
time restriction on admin transitions. Transitions use conditional SQL UPDATE
with RETURNING, not read-before-write; only currently allowed states can update.
Concurrent operations are serialized by PostgreSQL. Two operations can both
succeed if they form an allowed sequence, e.g. pending→confirmed→cancelled.

### AvailableSlotsResponse

```json
{
  "property_id":2,
  "date":"2030-10-24",
  "slots":[
    {"starts_at":"2030-10-24T05:00:00Z","available":true},
    {"starts_at":"2030-10-24T05:30:00Z","available":false}
  ]
}
```

The sample is abbreviated: the actual response contains **18** slots, last at
`13:30:00Z` (18:30 Astana). `available` is false for past/current starts or active
bookings, true otherwise. A past date returns 200 with all slots unavailable.
Cancelled/completed records are excluded. Availability is a snapshot, not a
reservation: Flutter must still handle 409 on booking and refresh availability.

## Exact Insomnia end-to-end sequence

Set environment variables (replace IDs/tokens with actual response values):

```json
{
  "base_url":"http://localhost:8080/api/v1",
  "user_token":"",
  "other_user_token":"",
  "admin_token":"",
  "property_id":2,
  "viewing_id":0,
  "future_date":"2030-10-24",
  "starts_at":"2030-10-24T05:00:00Z"
}
```

Before testing, run `docker compose up -d --build api`. Startup migrates the table
and active-slot index. Duplicate existing active rows cause a startup error:
resolve the data explicitly; migration never removes bookings.
Keep `future_date` in the future and pace requests for the global rate limiter.

1. **Register user**: `POST {{ _.base_url }}/register`, No Auth, JSON
   `{"email":"flutter-user@example.com","password":"ExamplePassword123!"}` → 201.
2. **Login user**: `POST {{ _.base_url }}/login`, No Auth, same JSON → 200.
   Save `access_token` as `user_token`; save refresh token separately.
3. Register/login a second account with email `flutter-other@example.com` and
   the same example password. Save its access token as `other_user_token`.
4. **Login an existing admin** using `/login` and its own credentials; save
   `admin_token`. If no admin exists, use the documented dev-only role endpoint
   on a *separate* test account, then disable it outside local development.
5. **Get property IDs**: `GET {{ _.base_url }}/properties`, Bearer `user_token` → 200.
   Save an existing `id` as `property_id`. If empty, create with admin token:
   `POST {{ _.base_url }}/properties`, JSON
   `{"title":"Flutter MVP test","address":"Astana","price":45000000}` → 201.
6. **Get slots**:
   `GET {{ _.base_url }}/properties/{{ _.property_id }}/available-slots?date={{ _.future_date }}`,
   Bearer `user_token` → 200. Save an `available:true` slot's exact UTC
   `starts_at` as the environment's `starts_at`.
7. **Book**: `POST {{ _.base_url }}/viewings`, Bearer `user_token`, JSON
   `{"property_id":{{ _.property_id }},"starts_at":"{{ _.starts_at }}"}` → 201.
   Save returned `id` as `viewing_id`; ownership must match the logged-in user.
8. Repeat step 7, including with `other_user_token` → **409**. Repeat step 6:
   the selected slot must be `available:false`.
9. **Own list**: `GET {{ _.base_url }}/viewings/me`, Bearer `user_token` → 200,
   contains the viewing. With `other_user_token`, it must not contain it.
   Adding `?user_id=<first user's ID>` must not change this isolation.
10. **Admin list**: `GET {{ _.base_url }}/admin/viewings`, Bearer `admin_token`
    → 200, contains all owners. User token → **403**; No Auth → **401**.
11. **Confirm**:
    `PATCH {{ _.base_url }}/admin/viewings/{{ _.viewing_id }}/status`,
    Bearer `admin_token`, JSON `{"status":"confirmed"}` → 200.
    Repeat same request → **409**. User token → **403**; No Auth → **401**.
12. Same PATCH with `{"status":"completed"}` → 200. Then
    `{"status":"cancelled"}` → **409** (terminal state).
13. Create another viewing via step 7 at a free future slot (the completed
    viewing's slot is released too). Save its new ID as `viewing_id`.
    PATCH it directly with `{"status":"cancelled"}` → 200.
    Availability must become true again; rebooking → 201.
    Confirm that new viewing, then cancel it → both 200, covering the other
    cancellation path. Always use the new ID returned by each POST.
14. **Invalid inputs** (admin token for status requests):
    - PATCH status URL with ID `0` or `abc` → 400.
    - PATCH existing ID with `{"status":"approved"}` → 400.
    - PATCH existing pending viewing with `{"status":"completed"}` → 409.
    - PATCH a verified nonexistent positive ID with `{"status":"confirmed"}` → 404.
    - GET available slots with `date=2030-02-30` → 400.
    - POST viewing with a valid future date at `09:30`, `19:00`, `15:15`,
      `15:00:01` or fractional seconds Astana time → 400.
    - POST viewing in the past → 400; nonexistent property + valid future slot → 404.
15. **Refresh**: `POST {{ _.base_url }}/auth/refresh`, No Auth, JSON
    `{"refresh_token":"<saved refresh token>"}` → 200. Replace both tokens.
    Reusing the consumed refresh token → 401.

## Verification and deployment limits

Run `go test -p 1 ./...` and `go build -p 1 ./...`.
PostgreSQL tests require `APPLICATION_TEST_DATABASE_URL` pointing at a dedicated
test database whose user can create/drop schemas. The existing fixture isolates
each test in a fresh schema. Without this variable these tests **skip**, not pass:

- `TestMyViewingsPostgresOwnership`
- `TestViewingPostgresTransitions` (all 16 state combinations, 404, 500, admin list)
- `TestViewingPostgresBookingAndAvailability`
- `TestViewingPostgresConcurrency` (unique-index and conditional-update races)
- `TestViewingPostgresMigrationPreservesDuplicates`
- Existing application PostgreSQL tests use the same variable.

Existing Redis session tests also skip when localhost Redis is unavailable.
The commands do not launch or rebuild the deployed API. PostgreSQL integration
tests migrate and write only inside their isolated test schemas; Redis session
tests create and consume their own test sessions. Do not treat a successful
build as a deployed API.

Pre-existing production risks are outside this focused viewing change: JWT
signing configuration is hardcoded in auth/middleware; Gin currently trusts all
proxies; the dev role switch must remain off in public environments. Treat this
as a local MVP backend, not a production security certification.
