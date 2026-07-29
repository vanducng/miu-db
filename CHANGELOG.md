# Changelog

## [0.10.4](https://github.com/vanducng/miu-db/compare/v0.10.3...v0.10.4) (2026-07-29)


### Bug Fixes

* **docs:** scope search trigger sizing ([#62](https://github.com/vanducng/miu-db/issues/62)) ([fc30a71](https://github.com/vanducng/miu-db/commit/fc30a71dceae5e9cc17bc75d839fe1cc7ca0935a))

## [0.10.3](https://github.com/vanducng/miu-db/compare/v0.10.2...v0.10.3) (2026-07-29)


### Bug Fixes

* **docs:** restore shell spacing ([#60](https://github.com/vanducng/miu-db/issues/60)) ([a6e8562](https://github.com/vanducng/miu-db/commit/a6e85620754efaa9058f9ba80e473fa64377be79))

## [0.10.2](https://github.com/vanducng/miu-db/compare/v0.10.1...v0.10.2) (2026-07-29)


### Bug Fixes

* **docs:** center navigation shell ([#58](https://github.com/vanducng/miu-db/issues/58)) ([f5d64fd](https://github.com/vanducng/miu-db/commit/f5d64fdeb3e7d6d7337d15229a0c378aa877e934))

## [0.10.1](https://github.com/vanducng/miu-db/compare/v0.10.0...v0.10.1) (2026-07-29)


### Bug Fixes

* **docs:** restore Starlight layout ([#56](https://github.com/vanducng/miu-db/issues/56)) ([ae8845f](https://github.com/vanducng/miu-db/commit/ae8845f63f9ce5a265f7b40317469cebcfbc48a2))

## [0.10.0](https://github.com/vanducng/miu-db/compare/v0.9.1...v0.10.0) (2026-06-20)


### Features

* **erd:** add SQLite introspection for ERD generation ([#51](https://github.com/vanducng/miu-db/issues/51)) ([cd1487a](https://github.com/vanducng/miu-db/commit/cd1487a121dbf193174078f5af2ffe2b48ea3d76))

## [0.9.1](https://github.com/vanducng/miu-db/compare/v0.9.0...v0.9.1) (2026-06-11)


### Bug Fixes

* **erd:** slugify default visuals leaf so group/name stays flat ([#49](https://github.com/vanducng/miu-db/issues/49)) ([6007a1b](https://github.com/vanducng/miu-db/commit/6007a1b28e922eac8b181485f1d1f380c60dc6b0))

## [0.9.0](https://github.com/vanducng/miu-db/compare/v0.8.1...v0.9.0) (2026-06-08)


### Features

* **erd:** default ERD output to .work/visuals when present ([#46](https://github.com/vanducng/miu-db/issues/46)) ([55d7f53](https://github.com/vanducng/miu-db/commit/55d7f53897c84dd4f49c1b5660916d4b7c103d96))

## [0.8.1](https://github.com/vanducng/miu-db/compare/v0.8.0...v0.8.1) (2026-06-08)


### Bug Fixes

* **tests:** apply moved-testdata paths to contract + MCP tests ([#43](https://github.com/vanducng/miu-db/issues/43)) ([a1cef97](https://github.com/vanducng/miu-db/commit/a1cef97daaf5fe44c2dba46f2b77fca4e08215a0))

## [0.8.0](https://github.com/vanducng/miu-db/compare/v0.7.0...v0.8.0) (2026-06-08)


### Features

* **output:** redact secrets from all command output (defense-in-depth) ([#38](https://github.com/vanducng/miu-db/issues/38)) ([c102e7a](https://github.com/vanducng/miu-db/commit/c102e7a8ff417228ba1fa598db50ada20834b4d1))

## [0.7.0](https://github.com/vanducng/miu-db/compare/v0.6.1...v0.7.0) (2026-06-08)


### ⚠ BREAKING CHANGES

* **config:** connections.json must use the "group" key; the legacy "folder_path" key is no longer read on load.

### Code Refactoring

* **config:** drop legacy folder_path support ([#36](https://github.com/vanducng/miu-db/issues/36)) ([e9bafa7](https://github.com/vanducng/miu-db/commit/e9bafa712ab8f80f5ab577dae6864b079b93c92b))

## [0.6.1](https://github.com/vanducng/miu-db/compare/v0.6.0...v0.6.1) (2026-06-08)


### Bug Fixes

* **erd:** 'erd serve' now opens the browser for interactive use ([#33](https://github.com/vanducng/miu-db/issues/33)) ([a0deb0e](https://github.com/vanducng/miu-db/commit/a0deb0e5cfa5ce787fc2c84578d97c3cf6c0fd95))

## [0.6.0](https://github.com/vanducng/miu-db/compare/v0.5.0...v0.6.0) (2026-06-08)


### Features

* **erd:** legible default zoom + clearer 'erd serve' output ([#31](https://github.com/vanducng/miu-db/issues/31)) ([7460941](https://github.com/vanducng/miu-db/commit/746094162c816eaf41b9a5bf45e528d639bfe1c7))

## [0.5.0](https://github.com/vanducng/miu-db/compare/v0.4.1...v0.5.0) (2026-06-08)


### Features

* **connections:** group/name resolution + 'connections list --basic' ([#28](https://github.com/vanducng/miu-db/issues/28)) ([b178653](https://github.com/vanducng/miu-db/commit/b1786530cf204d8f6787f1239acca1eb4d88a298))

## [0.4.1](https://github.com/vanducng/miu-db/compare/v0.4.0...v0.4.1) (2026-06-08)


### Bug Fixes

* **erd:** clear error when MySQL connection has no default database; add short flags ([#26](https://github.com/vanducng/miu-db/issues/26)) ([4ee4463](https://github.com/vanducng/miu-db/commit/4ee446388490d017a55f56b2a6ea28417f74577e))

## [0.4.0](https://github.com/vanducng/miu-db/compare/v0.3.0...v0.4.0) (2026-06-07)


### Features

* **erd:** add 'Focus selected (hide others)' view mode ([#24](https://github.com/vanducng/miu-db/issues/24)) ([1e82624](https://github.com/vanducng/miu-db/commit/1e82624c555de721e62663e94cb8bd258232c92d))

## [0.3.0](https://github.com/vanducng/miu-db/compare/v0.2.18...v0.3.0) (2026-06-07)


### Features

* **erd:** agentic hints + ER viewer DBML panel & UX ([#22](https://github.com/vanducng/miu-db/issues/22)) ([da9f038](https://github.com/vanducng/miu-db/commit/da9f038dbc043c93718e225dd6ef544a0e3969f5))
* **erd:** interactive offline ERD generation for MySQL + Postgres ([#20](https://github.com/vanducng/miu-db/issues/20)) ([0f09822](https://github.com/vanducng/miu-db/commit/0f098223a31dfe8a1a74888bdd7ae3f5bb73bbfc))
* **query:** add 'query script' for multi-statement execution ([#19](https://github.com/vanducng/miu-db/issues/19)) ([08cf57c](https://github.com/vanducng/miu-db/commit/08cf57c934ed591448afccc12f492dccbd24b912))
