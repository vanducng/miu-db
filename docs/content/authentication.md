---
title: Authentication
description: Snowflake OAuth and key-pair auth, BigQuery ADC and service accounts, and how tokens are stored.
---

| Backend | Auth | One-time step |
|---|---|---|
| Postgres, MySQL | Password | `--password` at `connections add`, stored per [secret store](/usage/#secret-stores) |
| Snowflake | Password, key pair (JWT), or OAuth | OAuth: `miudb auth login <conn>` |
| BigQuery | Application Default Credentials or a service-account file | `gcloud auth application-default login` |

## Snowflake

`--host` is the account, `--username` the user. Optional `--option` keys:
`warehouse`, `role`, `database`, `schema`.

### Key pair (JWT)

```sh
miudb connections add --name wh --db-type snowflake \
  --host <account> --username <user> \
  --option authenticator=snowflake_jwt \
  --option private_key_file=~/.ssh/snowflake_key.p8 \
  --option warehouse=DEV_WH
```

The key must be an unencrypted PKCS#8 PEM file.

### OAuth

`miudb auth login <conn>` opens the browser, listens on
`http://localhost:8085/`, and completes an authorization-code flow with PKCE
(S256). The access and refresh tokens go to the keyring under the `miudb`
service. Every later `query run` or `connections test` refreshes the access
token when it is within 5 minutes of expiry, so you log in again only when the
refresh token expires.

Admin setup, once per account. A public client needs no secret:

```sql
CREATE SECURITY INTEGRATION miudb_oauth
  TYPE = OAUTH
  OAUTH_CLIENT = CUSTOM
  OAUTH_CLIENT_TYPE = 'PUBLIC'
  OAUTH_REDIRECT_URI = 'http://localhost:8085/'
  OAUTH_ISSUE_REFRESH_TOKENS = TRUE
  OAUTH_REFRESH_TOKEN_VALIDITY = 86400
  ENABLED = TRUE;

SELECT SYSTEM$SHOW_OAUTH_CLIENT_SECRETS('MIUDB_OAUTH');  -- copy OAUTH_CLIENT_ID
```

Add the connection (no `--password`), then log in:

```sh
miudb connections add --name sf-prod --db-type snowflake \
  --host <account>.snowflakecomputing.com --username <user> \
  --option authenticator=oauth \
  --option oauth_client_id=<client-id> \
  --option oauth_authorization_url=https://<account>.snowflakecomputing.com/oauth/authorize \
  --option oauth_token_request_url=https://<account>.snowflakecomputing.com/oauth/token-request \
  --option oauth_scope=session:role:ANALYST

miudb auth login sf-prod
miudb auth status sf-prod
miudb auth logout sf-prod
```

| Option | Required | Default | Notes |
|---|---|---|---|
| `authenticator` | yes | | `oauth` |
| `oauth_client_id` | yes | | From the security integration |
| `oauth_authorization_url` | yes | | `https://<account>.snowflakecomputing.com/oauth/authorize` |
| `oauth_token_request_url` | yes | | `https://<account>.snowflakecomputing.com/oauth/token-request` |
| `oauth_redirect_uri` | no | `http://localhost:8085/` | Must match `OAUTH_REDIRECT_URI` |
| `oauth_scope` | no | | Comma-separated, for example `session:role:ANALYST` |
| `oauth_client_secret` | no | | Confidential clients only. Prefer a public client with PKCE. |

## BigQuery

`--host` is the GCP project billed for queries.

```sh
gcloud auth application-default login
miudb connections add --name bq --db-type bigquery --host my-gcp-project
```

| Option | Default | Notes |
|---|---|---|
| `auth_method` | `adc` | `adc` or `service_account`. Anything else fails. |
| `bigquery_credentials_path` | | Service-account JSON. Required for `service_account`; with `adc` it is used when set. |
| `bigquery_quota_project` | | Project charged for quota |

Without credentials, queries fail with `BigQuery: no credentials found` and the
fix: run the gcloud login, or set `bigquery_credentials_path` or
`GOOGLE_APPLICATION_CREDENTIALS`. There is no `miudb auth login` for BigQuery.

## Secret handling

- `oauth_client_secret` is redacted in every output and listed under
  `sensitive_targets`.
- The access token is injected in memory at query time. It is never written to
  `connections.json` and never printed.
- Refresh tokens live in the keyring under the `miudb` service.
