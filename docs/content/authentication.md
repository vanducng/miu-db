---
title: Authentication
description: OAuth login flows for Snowflake and BigQuery
---

## Overview

miu-db uses an asymmetric login model: each backend has a one-time interactive
step that stores a refresh token, and all subsequent queries use the stored
token silently.

| Backend   | Login mechanism                       | Command                                    |
|-----------|---------------------------------------|--------------------------------------------|
| Snowflake | OAuth 2.0 (loopback + PKCE S256)      | `miudb auth login <conn>`                  |
| BigQuery  | Application Default Credentials (ADC) | `gcloud auth application-default login`    |

---

## Snowflake OAuth

### How it works

1. `miudb auth login <conn>` opens the browser to the Snowflake authorization
   endpoint, starts a loopback listener on `http://localhost:8085/`, and waits
   for the redirect.
2. The access token and refresh token are stored in the OS keyring under the
   `miudb` service.
3. On every `query run` or `connections test`, miu-db loads the stored token,
   refreshes it automatically when it is within 5 minutes of expiry, and injects
   the fresh access token into the Snowflake driver config — no manual re-login
   needed until the refresh token itself expires.

### Snowflake admin setup

Create a **PUBLIC client** integration (no client secret, PKCE only). Replace
`<account>` with your Snowflake account identifier.

```sql
CREATE SECURITY INTEGRATION miudb_oauth
  TYPE = OAUTH
  OAUTH_CLIENT = CUSTOM
  OAUTH_CLIENT_TYPE = 'PUBLIC'
  OAUTH_REDIRECT_URI = 'http://localhost:8085/'
  OAUTH_ISSUE_REFRESH_TOKENS = TRUE
  OAUTH_REFRESH_TOKEN_VALIDITY = 86400
  ENABLED = TRUE;
```

After creation, retrieve the client ID:

```sql
SELECT SYSTEM$SHOW_OAUTH_CLIENT_SECRETS('MIUDB_OAUTH');
```

Copy the `OAUTH_CLIENT_ID` value for the connection options below.

### Add a Snowflake OAuth connection

```bash
miudb connections add \
  --name sf-prod \
  --db-type snowflake \
  --host <account>.snowflakecomputing.com \
  --username <user> \
  --option authenticator=oauth \
  --option oauth_client_id=<client-id> \
  --option oauth_authorization_url=https://<account>.snowflakecomputing.com/oauth/authorize \
  --option oauth_token_request_url=https://<account>.snowflakecomputing.com/oauth/token-request \
  --option oauth_scope=session:role:ANALYST \
  --output json
```

No `--password` is required or expected for OAuth connections.

### Login and query

```bash
miudb auth login sf-prod --output json
miudb query run --connection sf-prod --sql 'SELECT CURRENT_ROLE()' --output json
```

### Token status and logout

```bash
miudb auth status sf-prod --output json
miudb auth logout sf-prod --output json
```

### Option reference

| Option key                  | Required | Default                       | Description                                       |
|-----------------------------|----------|-------------------------------|---------------------------------------------------|
| `authenticator`             | yes      | —                             | Must be `oauth`                                   |
| `oauth_client_id`           | yes      | —                             | Client ID from the Snowflake security integration |
| `oauth_authorization_url`   | yes      | —                             | `https://<account>.snowflakecomputing.com/oauth/authorize` |
| `oauth_token_request_url`   | yes      | —                             | `https://<account>.snowflakecomputing.com/oauth/token-request` |
| `oauth_redirect_uri`        | no       | `http://localhost:8085/`      | Must match `OAUTH_REDIRECT_URI` in the integration |
| `oauth_scope`               | no       | —                             | Comma-separated scopes, e.g. `session:role:ANALYST` |
| `oauth_client_secret`       | no       | —                             | Only for CONFIDENTIAL clients (not recommended; prefer PUBLIC + PKCE) |

> **Recommendation:** Use `OAUTH_CLIENT_TYPE = 'PUBLIC'` in the security
> integration. This eliminates `oauth_client_secret` entirely and relies on
> PKCE (S256) for proof of code verifier — the same model used by native
> desktop apps. Confidential-client secret storage in miu-db is deferred.

---

## BigQuery

BigQuery uses Google Application Default Credentials. There is no
`miudb auth login` step for BigQuery; authenticate once with gcloud:

```bash
gcloud auth application-default login
```

Then add the connection:

```bash
miudb connections add \
  --name bq-analytics \
  --db-type bigquery \
  --option project=my-gcp-project \
  --option dataset=analytics \
  --output json
```

### BigQuery option reference

| Option key                | Required | Description                                          |
|---------------------------|----------|------------------------------------------------------|
| `project`                 | yes      | GCP project ID billed for queries                    |
| `dataset`                 | no       | Default dataset for unqualified table references     |
| `bigquery_quota_project`  | no       | Override billing project for quota purposes          |
| `auth_method`             | no       | `adc` (default) or `service_account`                 |
| `credentials_file`        | no       | Path to a service-account JSON key (auth_method=service_account) |

---

## Secret handling

- `oauth_client_secret` (when present) is **redacted** in all `connections list`
  and `connections add` output — it appears as `{"redacted": true}` and is
  listed under `sensitive_targets` in the response.
- The transient access token (`__oauth_access_token`) is a runtime-only value
  injected into memory during query execution. It is **never** written to
  `connections.json`, never shown in any output, and never listed as a sensitive
  target.
- Refresh tokens are stored in the OS keyring via the `miudb` service.
