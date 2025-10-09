# Changelog

## [1.2.0](https://github.com/openkcm/registry/compare/v1.1.0...v1.2.0) (2025-10-09)


### Features

* apply auth and get auth ([3a24ce1](https://github.com/openkcm/registry/commit/3a24ce118d4fbc2c8e38088cd52cb139562f64a5)), closes [#54](https://github.com/openkcm/registry/issues/54)
* changed required params for list systems to be one of tenantID or externalID ([#58](https://github.com/openkcm/registry/issues/58)) ([f007454](https://github.com/openkcm/registry/commit/f007454ac76d0a4c3f818e6effafc1ca4bfeabf9))
* remove auth ([efd6674](https://github.com/openkcm/registry/commit/efd6674d1b592169c743f63b7ee42bd7c835842e)), closes [#61](https://github.com/openkcm/registry/issues/61)


### Bug Fixes

* auto migrate auth model ([e635e88](https://github.com/openkcm/registry/commit/e635e88817dec1e38b147368fc7ac6a83abed569)), closes [#64](https://github.com/openkcm/registry/issues/64)

## [1.1.0](https://github.com/openkcm/registry/compare/v1.0.1...v1.1.0) (2025-09-29)


### Features

* apply tenant auth ([53a017b](https://github.com/openkcm/registry/commit/53a017b1d6d33c9223928299f678e7d7577f19b1)), closes [#34](https://github.com/openkcm/registry/issues/34)
* implemented GetTeanant method ([#22](https://github.com/openkcm/registry/issues/22)) ([1b0d1a4](https://github.com/openkcm/registry/commit/1b0d1a413e9f6830c873132694e29859a5f2c623))
* List systems by type ([#25](https://github.com/openkcm/registry/issues/25)) ([a5f1d86](https://github.com/openkcm/registry/commit/a5f1d86edf7c4f38eaf1455603729af61ce5676a))
* make orbital implementation service agnostic ([de89bdf](https://github.com/openkcm/registry/commit/de89bdfcfd5a5b79fe78f229d8362e88d750ce98)), closes [#46](https://github.com/openkcm/registry/issues/46)
* tenant add user groups rpc  ([8f7f343](https://github.com/openkcm/registry/commit/8f7f343f62253113ebc033a2f0c19bf03fd2ee86))


### Bug Fixes

* Switch to bitnamilegacy repo for Helm tests ([#42](https://github.com/openkcm/registry/issues/42)) ([08c17a5](https://github.com/openkcm/registry/commit/08c17a52e2db4a72c9c3fe12de5b183744e84534))

## [1.0.1](https://github.com/openkcm/registry/compare/v1.0.0...v1.0.1) (2025-08-28)


### Bug Fixes

* fix readiness probe ([#13](https://github.com/openkcm/registry/issues/13)) ([337c46a](https://github.com/openkcm/registry/commit/337c46a34741875e9cfa93530258227a7c12a74d))
* fix some chart configuration and version; adjust the status for grpc server ([#14](https://github.com/openkcm/registry/issues/14)) ([6c086f4](https://github.com/openkcm/registry/commit/6c086f4e5c32cb9921b58bef4fd58472c6f283a0))

## 1.0.0 (2025-08-21)


### Features

* code migration from internal ([#4](https://github.com/openkcm/registry/issues/4)) ([06e0dbe](https://github.com/openkcm/registry/commit/06e0dbe072e85290379bb49b8b0cf3eb1c7e53ff))


### Bug Fixes

* add missing files ([#6](https://github.com/openkcm/registry/issues/6)) ([c42d5d6](https://github.com/openkcm/registry/commit/c42d5d6208d6f90370c046c0e893dc7e9ab43675))
* update some of the base setup ([#3](https://github.com/openkcm/registry/issues/3)) ([1055fde](https://github.com/openkcm/registry/commit/1055fde35c65066197aefa3f648c678378e66ad7))
