---
title: "Cloud Adapters"
---

## BigQuery

Current config uses `endpoint.host` as the project and
`options.bigquery_credentials_path` as the service account file.

## Snowflake

Current config uses `endpoint.host` as account, `endpoint.username` as user, and
`options.private_key_file` with `authenticator: snowflake_jwt` for JWT auth.
