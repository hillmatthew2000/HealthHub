# HealthHub web

The browser front end for the HealthHub API: a Vue 3 single page app for
clinical staff to manage patients and observations, and for administrators to
manage accounts and review the audit trail.

Stack: Vue 3, Vite, TypeScript, Tailwind CSS 4, Pinia, Vue Router and
Vitest. The production image is a static build served by nginx.

## Features

| Page | Who sees it | API |
| --- | --- | --- |
| Sign in | everyone | `POST /api/v1/auth/login` |
| Dashboard: totals, recent observations, API health | any role | `GET /health/ready`, list endpoints with `limit=1` |
| Patients: search, filters, pagination | patients read | `GET /api/v1/patients` |
| Patient detail with an observations tab | patients read | `GET /api/v1/patients/:id`, `GET /api/v1/patients/:id/observations` |
| Create, edit, delete patients | patients create, update, delete | `POST`, `PUT`, `DELETE /api/v1/patients[/:id]` |
| Observations: list, filters, create, edit, delete | observations read and up | `/api/v1/observations[/:id]` |
| Users: list, create, edit roles and active status | admin | `/api/v1/users[/:id]` (`PATCH` for edits) |
| Audit events: filter by actor, record, type, dates | admin | `GET /api/v1/audit-events` |
| Profile and change password | any signed-in user | `GET /api/v1/auth/profile`, `POST /api/v1/auth/change-password` |

Navigation and actions follow the API's access policy
(`internal/auth/roles.go`, mirrored in `src/lib/permissions.ts`), so a nurse
does not see edit buttons and only administrators see Users and Audit
events. The API still enforces every rule; the UI only hides what a role
cannot use.

Patient search uses the API's exact-match filters, because names and contact
details are stored encrypted and the API has no free-text search. The search
box sends a `YYYY-MM-DD` value as `birthdate`, anything with an `@` or made
of digits as `telecom`, and everything else as `family`.

### Sessions

- The access token is kept in memory and in `sessionStorage`, so it survives
  a reload but not closing the tab, and is never sent as a cookie.
- The token is refreshed (`POST /api/v1/auth/refresh`) five minutes before
  it expires (the API issues one hour tokens by default), as long as the user
  was active in the last 30 minutes. An idle session is allowed to expire.
- Any 401 from an authenticated request signs the user out and returns them
  to the sign-in page, which then sends them back to where they were.

### Errors

The API returns RFC 9457 problem details (`application/problem+json`). The
client (`src/api/client.ts`) turns every error into an `ApiError` holding the
problem, and the UI shows its `detail`, the per-field `errors` next to the
matching inputs, and the `requestId` to quote when reporting a problem.
Form validation mirrors the API's rules (for example passwords of 12 to 72
characters), and uses the same field paths such as `name[0].family`.

## Local development

Requirements: Node.js 20.19 or later (CI uses Node 24).

```sh
cd web
npm ci
npm run dev
```

The dev server runs at http://localhost:5173 and proxies `/api` and
`/health` to `http://localhost:8080`, so the app talks to the API as if it
were on the same origin and no CORS setup is needed. To use another API, set
`HEALTHHUB_API_PROXY`:

```sh
HEALTHHUB_API_PROXY=http://192.168.1.96:8081 npm run dev
```

To run the API locally, start the compose stack from the repository root
(`docker compose -f deployments/docker/docker-compose.yml up --build`) or
run the Go server directly. Registration is disabled by default; create a
first administrator with the server's `create-admin` command (run
`server create-admin -h` for its flags; it reads the password from
`ADMIN_PASSWORD` or standard input).

Other scripts:

| Command | What it does |
| --- | --- |
| `npm run lint` | ESLint, no warnings allowed |
| `npm run typecheck` | `vue-tsc` over the app and config |
| `npm test` | Vitest unit tests for the API client, the auth store and form logic |
| `npm run build` | Production build into `dist/` |
| `npm run preview` | Serve the production build locally |

## Runtime configuration

The build contains no environment-specific settings. `index.html` loads
`/config.js` before the app, which sets:

```js
window.__HEALTHHUB_CONFIG__ = { apiBaseUrl: 'http://192.168.1.96:8081' }
```

`apiBaseUrl` is the API origin (plus an optional path prefix), without
`/api/v1`. Empty or unset means the API is served from the same origin as the
app, for example behind a reverse proxy that routes `/api` and `/health` to
it.

In the container, `docker/40-healthhub-config.sh` writes `/config.js` at
start-up from the `API_BASE_URL` environment variable, so one image serves
every environment without a rebuild. The script only accepts a plain
`http(s)://host[:port][/path]` value and refuses to start otherwise. It also
adds the API origin to the page's `Content-Security-Policy` `connect-src`.
`/config.js` is served with `Cache-Control: no-cache`.

## Container image

`web/Dockerfile` builds the app with Node, then serves `dist/` with
`nginx:alpine`:

- runs as the unprivileged `nginx` user and listens on port 80
- falls back to `index.html` for client-side routes
- caches hashed assets for a year and revalidates `index.html` and
  `/config.js` on every load
- sends a strict content security policy, `Referrer-Policy: no-referrer`
  (URLs can contain patient IDs and search terms) and logs request paths
  without query strings
- exposes `GET /healthz` for health checks

```sh
docker build -t healthhub-web web
docker run --rm -p 8082:80 -e API_BASE_URL=http://localhost:8080 healthhub-web
```

## Deploying

The API must allow the web app's origin. Set `ALLOWED_ORIGINS` on the API to
the exact origin users load the app from (scheme, host and port, no trailing
slash), for example:

```sh
ALLOWED_ORIGINS=http://192.168.1.96:8082
```

The API's CORS middleware already allows the `Authorization` and
`Content-Type` headers and the `GET`, `POST`, `PUT`, `PATCH` and `DELETE`
methods this app uses, and answers preflight requests for allowed origins.

### Coolify (lab)

| Setting | Value |
| --- | --- |
| Build pack | Dockerfile |
| Base directory | `/web` |
| Dockerfile location | `web/Dockerfile` (`/Dockerfile` relative to the base directory) |
| Port mapping | `8082:80` |
| Environment | `API_BASE_URL=http://192.168.1.96:8081` |
| Health check path | `/healthz` |

On the API service set `ALLOWED_ORIGINS=http://192.168.1.96:8082`.
