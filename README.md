# apismith

Internal tool for exercising Nimbus backend APIs from the same OpenAPI
contract that `oapi-codegen` uses to generate the server.

This is **Phase 1 (API Explorer)** — not a Postman replacement. The OpenAPI
spec is the single source of truth: new endpoints appear here automatically.

```
openapi.yaml  ──►  oapi-codegen  ──►  backend server
       │
       └──►  apismith  ──►  DEV / STAGING API
                    │
                    └── Cognito JWT
```

```sh
apismith ui                         # local explorer
apismith jwt                        # Cognito access token
apismith ls                         # list operations from OpenAPI
apismith call METHOD PATH           # send one request
```

## What you can do

1. Start the console (`apismith ui`)
2. Select **DEV** or **STAGING**
3. Enter or reuse a Cognito Client ID
4. Generate a JWT (same SRP flow as [jwt-token-printer](https://github.com/aegion-dynamic/jwt-token-printer))
5. Search and select any operation from the OpenAPI spec
6. Fill path/query params and a form or JSON body
7. Send the request through the local proxy
8. Inspect status, timing, headers, and body

The browser talks only to this process. The process attaches
`Authorization: Bearer <access token>` and forwards the call, which avoids
CORS and keeps Cognito secrets off the frontend.

## Run

```sh
cp .env.example .env          # fill Cognito + credentials
make build                    # bin/apismith
make run                      # apismith ui → http://localhost:8090
```

The console listens on **127.0.0.1:8090** so it does not collide with the backend on
`:8080`. Override with `CONSOLE_LISTEN` (e.g. `0.0.0.0:8090` on a trusted network).

### CLI

Commands resolve operations from the OpenAPI spec. There is no second API list.

```sh
apismith ls
apismith ls --tag users --search login
apismith ls --json

apismith jwt                              # uses environments + credentials
apismith jwt --token-only                 # pipeable access token
apismith jwt path/to/config.json          # drop-in for jwt-token-printer

apismith call GET /users/me --env dev
apismith call GET /users/me --env staging
apismith call GET /users/me --base-url https://api.example.com/api/v1
apismith call POST /users --body '{"email":"..."}'
apismith call GET /users/{id} --path id=123 --query page=1
apismith call createUser --body-file user.json
apismith call POST /auth/login --no-auth --body-file login.json
apismith call GET /missing --expect 404
```

`call` exits `0` on 2xx (or when `--expect` matches) and `1` otherwise.

The backend verifies Cognito **access** tokens (`token_use=access`), so
apismith attaches the access token, not the ID token.

### OpenAPI spec

By default apismith loads the sibling backend spec:

```
../framework-backend/nimbus_openapi_spec.yaml
```

Override with `CONSOLE_OPENAPI_SPEC`, or place a copy at `openapi/openapi.yaml`.

### Environments

Edit `config/environments.yaml`. Each environment has a `base_url` — that is
the API you hit, local or hosted. DEV defaults to localhost; set STAGING (or
add another env) to your deployed server:

```yaml
  - id: staging
    name: STAGING
    base_url: https://your-hosted-api.example.com/api/v1
    production: false
```

One-off overrides without editing the file:

- CLI: `apismith call GET /users/me --base-url https://your-hosted-api.example.com/api/v1`
- UI: edit the **Base URL** field next to the environment selector
- `.env`: `CONSOLE_BASE_URL=https://your-hosted-api.example.com/api/v1` (overlays the default env)

DEV and STAGING are enabled; PRODUCTION, if you add it with `production: true`,
requires `--confirm-production` (CLI) or an explicit confirmation in the UI.

Secrets do **not** belong in that file. Put them in:

- environment variables (`COGNITO_*`, `CONSOLE_USERNAME`, `CONSOLE_PASSWORD`)
- `config/credentials.yaml` (gitignored; see `config/credentials.yaml.example`)

The credentials file is compatible with jwt-token-printer's `config.json`.

## Layout

```
auth/cognito/     reusable Cognito token generation (SRP + password)
auth/             in-memory auth manager (JWT never written to disk)
openapi/          kin-openapi parser → endpoint catalog
request/          URL builder, proxy executor, masked history
environments/     DEV / STAGING / PRODUCTION
server/           local HTTP API + request proxy
ui/               explorer, request builder, response viewer
cli/              apismith actions (ui, jwt, ls, call)
cmd/apismith/     CLI entrypoint
cmd/console/      alias for `apismith ui`
```

## Security

- JWTs live in memory (browser + process). They are not logged.
- `Authorization` is stored in history as `Bearer ********`.
- Client secrets stay on the server; the UI only sees client IDs.
- The console binds to `127.0.0.1` by default and has no auth of its own;
  do not expose it on an untrusted network without adding protection.
- Production requests require confirmation.

## Later phases (not in this MVP)

- Saved requests
- Repeatable test cases, variables, cleanup
- `apismith run --env staging --suite regression`
- CI/CD gate on failed tests
