# Go Daily Driver Notes

The experimental Go CLI is `miudb`.

Build:

```bash
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
```

Current config smoke:

```bash
./.miu-db/miudb connections list \
  --config-dir /Users/vanducng/.config/miu/db \
  --credentials-export /Users/vanducng/.config/miu/db/credentials-export.json \
  --output json
```

Connection test summary on 2026-05-28:

- Passing: 21 of 27 saved connections.
- Ignoring unavailable local-only DBs (`sci-local`, `rnd-local`,
  `tenstreet-local`, `cnb-ai-local`) gives 21 of 23 passing connections.
- Remaining non-local failures are `chat-prod` network timeout and `wg` MySQL
  connection loss/invalid connection through the tunnel. Installed Python
  `miu-db` fails on the same two from this environment.
- Verified with:

```bash
./.miu-db/miudb connections smoke \
  --timeout 20s \
  --concurrency 1 \
  --config-dir /Users/vanducng/.config/miu/db \
  --credentials-export /Users/vanducng/.config/miu/db/credentials-export.json \
  --output json
```

Non-local-only matrix:

```bash
./.miu-db/miudb connections smoke \
  --timeout 20s \
  --concurrency 1 \
  --connection abs-bq-prod \
  --connection abs-bq-dev \
  --connection cnb-snowflake \
  --connection tenstreet-staging \
  --connection tenstreet-prod \
  --connection cdljn \
  --connection sci-prod \
  --connection sci-dev \
  --connection web \
  --connection goclaw-local \
  --connection miubot \
  --connection chat-prod \
  --connection chat-staging \
  --connection csn \
  --connection development \
  --connection driverwave \
  --connection fullapp-proxy \
  --connection tw \
  --connection wg \
  --connection agent-deck \
  --connection cnb-ai-prod \
  --connection cnb-snowflake-dbt-analytics \
  --connection cnb-snowflake-elt \
  --config-dir /Users/vanducng/.config/miu/db \
  --credentials-export /Users/vanducng/.config/miu/db/credentials-export.json \
  --output json
```

- Environmental failures: local ports `54322` and `3307` were not listening.
- Network failure: `chat-prod` timed out from the current network path.
- Connection-specific failure: `wg` closed during MySQL handshake.
- One-at-a-time Docker checks confirmed `sci-local` passes when `sci-db` is
  running, and `rnd-local` passes when `rnd-mysql` is running. These are not
  kept running by this verification.
- `tenstreet-local` still needs a matching local MySQL service on `3307`.
  Candidate container `dev-mysql-1` could not start because a bind-mounted
  seed file path is invalid in the current checkout; `transfer-ai-mysql` starts
  but does not match the saved connection auth/database.
- A throwaway MySQL container with database `tenstreet_lead_nurture`, user
  `tenstreet`, and the saved credential from `credentials-export.json` confirms
  `tenstreet-local` can connect and query through Go when a matching local
  service is present.
- `cnb-ai-local` is backed by `/Users/vanducng/git/work/cnb/products/cnb-ai`.
  Its compose file defaults to a different DB port and the MySQL image rejects
  `MYSQL_USER=root`; running only MySQL with `FORWARD_DB_PORT=3307` and a
  non-root bootstrap `DB_USERNAME` confirms the saved root connection can query
  through Go.

Useful local checks:

```bash
# cnb-ai-local
cd /Users/vanducng/git/work/cnb/products/cnb-ai
FORWARD_DB_PORT=3307 DB_USERNAME=cnb_ai_user docker compose up -d mysql
/Users/vanducng/git/personal/worktrees/miu-db-golang/.miu-db/miudb query run \
  --connection cnb-ai-local \
  --sql "select 1 as one" \
  --config-dir /Users/vanducng/.config/miu/db \
  --credentials-export /Users/vanducng/.config/miu/db/credentials-export.json
docker compose stop mysql
```

```bash
# tenstreet-local throwaway smoke service
secret=$(jq -r '.entries[] | select(.connection=="tenstreet-local" and .kind=="db") | .password' /Users/vanducng/.config/miu/db/credentials-export.json)
docker run -d --name miudb-tenstreet-local-smoke \
  -e MYSQL_DATABASE=tenstreet_lead_nurture \
  -e MYSQL_USER=tenstreet \
  -e MYSQL_PASSWORD="$secret" \
  -e MYSQL_ROOT_PASSWORD="$secret" \
  -p 3307:3306 \
  mysql:8.0
/Users/vanducng/git/personal/worktrees/miu-db-golang/.miu-db/miudb query run \
  --connection tenstreet-local \
  --sql "select 1 as one" \
  --config-dir /Users/vanducng/.config/miu/db \
  --credentials-export /Users/vanducng/.config/miu/db/credentials-export.json
docker rm -f miudb-tenstreet-local-smoke
```
- MySQL TLS follows the current miu-db config model: explicit `options.tls_mode`
  or `extra_options.ssl-mode` drives TLS behavior. Legacy `connection_url`
  query strings are preserved for reference but are not treated as active TLS
  config unless they were materialized into options.

No credential values are printed by `connections list`.
