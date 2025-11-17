# Changelog

## [1.6.0](https://github.com/openkcm/registry/compare/v1.5.0...v1.6.0) (2025-11-14)


### Features

* add paginated ListAuths endpoint ([85abd4b](https://github.com/openkcm/registry/commit/85abd4b26e42dd0a9781b67ea7badc790740217e))
* add regex validation for UserGroups ([#86](https://github.com/openkcm/registry/issues/86)) ([60488c2](https://github.com/openkcm/registry/commit/60488c2d7e3ce1877472d97bfaaf43cf452e264e))

## [1.5.0](https://github.com/openkcm/registry/compare/v1.4.0...v1.5.0) (2025-11-10)


### Features

* sync auth status with tenant block/unblock  ([34e4fe7](https://github.com/openkcm/registry/commit/34e4fe7cfb08ee7d6f03a06e29ab16ce219396f5))

## [1.4.0](https://github.com/openkcm/registry/compare/v1.3.0...v1.4.0) (2025-11-07)


### Features

* List tenants by labels ([#83](https://github.com/openkcm/registry/issues/83)) ([30739b1](https://github.com/openkcm/registry/commit/30739b16fd3ff055369a2b634f75e139535129a7))
* sync tenant and auth status transitions  ([77e19fc](https://github.com/openkcm/registry/commit/77e19fc6c508294678bdb086435915845c0aa214))

## [1.3.0](https://github.com/openkcm/registry/compare/v1.2.0...v1.3.0) (2025-10-30)


### Features

* add validation for tenant model  ([396129c](https://github.com/openkcm/registry/commit/396129c837dccb2b50d463b54efc798abd32221f))
* configurable system validation ([2897f95](https://github.com/openkcm/registry/commit/2897f9566867081b0d4b1535e752eeb2f11e20f0)), closes [#70](https://github.com/openkcm/registry/issues/70)
* configurable validation ([a24107e](https://github.com/openkcm/registry/commit/a24107ea6ee94a9de505c4de28e9114b9694809c)), closes [#67](https://github.com/openkcm/registry/issues/67)
* document validation package ([fb3b3fd](https://github.com/openkcm/registry/commit/fb3b3fdad0773f77e3f31c47e148d32ca7a9e3aa)), closes [#72](https://github.com/openkcm/registry/issues/72)


### Bug Fixes

* build info injected using ldflag on build ([#44](https://github.com/openkcm/registry/issues/44)) ([cf58f69](https://github.com/openkcm/registry/commit/cf58f69d48016fb840a2f912c4ea5b1ac6986d35))

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
