#!/usr/bin/env bash
# Runs inside `asciinema rec`. $1 = demo root, $2 = dir holding the miudb binary.
set -u
root=$1
export HOME="$root/u" XDG_CONFIG_HOME="$root/u/.config" PATH="$2:$PATH" GIT_PAGER=cat PAGER=cat
unset MIUDB_CONFIG_DIR MIU_DB_CONFIG_DIR
rm -rf "$root/u" "$root/shop" && mkdir -p "$root/u" "$root/shop" && cd "$root/shop"
sqlite3 shop.db <<'SQL'
create table users (id integer primary key, name text not null, email text unique);
create table products (id integer primary key, name text not null, price real not null);
create table orders (id integer primary key, user_id integer references users(id),
  product_id integer references products(id), quantity integer default 1);
insert into users values (1, 'Ada', 'ada@example.com'), (2, 'Linus', 'linus@example.com'), (3, 'Grace', 'grace@example.com');
insert into products values (1, 'Keyboard', 49.0), (2, 'Mouse', 19.5), (3, 'Monitor', 189.0);
insert into orders values (1, 1, 1, 1), (2, 1, 2, 2), (3, 2, 3, 1), (4, 3, 2, 3), (5, 3, 1, 1);
SQL

note() { printf '\033[2m# %s\033[0m\n' "$1"; sleep "${2:-0.8}"; }
run() {
  printf '\033[1;32m$\033[0m '
  local s=$1 i
  for ((i = 0; i < ${#s}; i++)); do printf '%s' "${s:i:1}"; sleep 0.02; done
  sleep 0.35
  printf '\n'
  eval "$s"
  sleep "${2:-2}"
}

clear
note "miudb: a headless database CLI for humans and agents"
note "install: curl -fsSL https://db.miu.sh/install.sh | sh" 1.2
run "miudb version | jq -r .data.version" 1.5
clear
note "1. save a connection; passwords would go to the keyring"
run "miudb connections add --name shop --db-type sqlite --path ./shop.db | jq -c '{ok, kind, name: .data.name}'" 2
run "miudb connections list --basic | jq -c '.data.connections[]'" 2
run "miudb connections smoke | jq -c '.summary | {count, passed, failed}'" 2.5
clear
note "2. query: bounded rows, stable JSON envelope, cursor for the rest"
run "miudb query run --connection shop --sql 'select * from orders' --limit 3 | jq -c '.data.result.rows[]'" 2
run "miudb query run --connection shop --sql 'select * from orders' --limit 3 | jq -c .page" 3
clear
note "3. inspect the schema"
run "miudb schema tree --connection shop | jq -r '.data.columns[] | [.table, .name, .type] | @tsv' | column -t" 3.5
clear
note "4. offline interactive ERD, plus read-only MCP tools for agent hosts"
run "miudb erd generate -c shop -f html,json,dbml --out-dir erd | jq -c '{ok, tables: .data.tables}'" 1.5
run "ls erd" 2
note "claude mcp add --transport stdio miudb -- miudb mcp serve --transport stdio" 2.5
clear
note "5. every call is logged as metadata, never result rows"
run "miudb activity --since 1h | jq -r '.data.events[] | [.source, .op, .latency_ms, .sql_shape] | @tsv'" 3.5
clear
note "docs: db.miu.sh  (append .md to any page for agents)" 2.5
