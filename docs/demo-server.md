# Public VPS demo

ByeClaude includes a small read-only HTTP demo server:

```sh
byeclaude serve
```

Open `http://127.0.0.1:8080`. The embedded page accepts a public GitHub `owner/repository` slug and can run either a normal attribution scan or a rewrite-impact plan.

The demo has **no cleanup endpoint**.

## Security boundary

The built-in server is intentionally narrower than the CLI:

- public GitHub repositories only;
- accepts `owner/repository`, not arbitrary clone URLs;
- constructs the `https://github.com/owner/repository.git` URL server-side;
- disables Git credential helpers for its clone command;
- never accepts a GitHub token from the browser;
- no checkout and no execution of repository scripts/hooks;
- temporary mirror workspace removed after each audit;
- request body capped at 8 KiB;
- bounded in-flight repository audits;
- per-audit timeout;
- no CORS wildcard;
- no rewrite, push, hook or restore HTTP endpoint.

This makes it suitable as a **demo boundary**, not a complete multi-tenant production platform.

## Run the binary

Behind a reverse proxy on the same VPS:

```sh
byeclaude serve \
  --listen 127.0.0.1:8080 \
  --max-inflight 2 \
  --timeout 60s
```

Keep `127.0.0.1` when Caddy/nginx is in front. Bind to `0.0.0.0` only when your network/container boundary is deliberate.

Use a server-side rules file if the demo should classify more than the default Claude/Anthropic attribution:

```sh
byeclaude serve --rules ./rules.json
```

Website visitors cannot replace that ruleset.

## Docker

Build:

```sh
docker build -f deploy/demo/Dockerfile -t byeclaude-demo .
```

Run:

```sh
docker run --rm \
  -p 127.0.0.1:8080:8080 \
  byeclaude-demo
```

The image contains Git + CA certificates and runs ByeClaude as an unprivileged user.

## Caddy

An intentionally small example lives at [`deploy/demo/Caddyfile.example`](../deploy/demo/Caddyfile.example).

Replace the domain, then point Caddy at the loopback-bound ByeClaude server.

For an internet-facing demo, add rate limiting at the reverse proxy/WAF/provider layer. ByeClaude itself only bounds **concurrent work**, so it returns HTTP `429` when all audit slots are occupied, but it is not a per-IP abuse-prevention system.

## HTTP API

Health:

```http
GET /healthz
```

Audit:

```http
POST /v1/audits
Content-Type: application/json

{
  "repository": "owner/repository",
  "mode": "scan"
}
```

Plan:

```json
{
  "repository": "owner/repository",
  "mode": "plan"
}
```

The response is the same batch JSON model used by the CLI. Plan mode includes the estimated commit-DAG reconstruction work and rewrite readiness.

## Private repositories later

Do not make a public server token capable of reading private repositories and then let anonymous visitors choose repository slugs.

For private-repo support, use GitHub OAuth or preferably a GitHub App so each user explicitly authorizes repository access. Keep those credentials server-side and short-lived/scoped. The current demo intentionally does not implement that auth flow.
