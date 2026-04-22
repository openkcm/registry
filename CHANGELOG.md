# Changelog

## [1.9.0](https://github.com/openkcm/registry/compare/v1.8.0...v1.9.0) (2026-04-20)


### Features

* Create Unit test result ([#126](https://github.com/openkcm/registry/issues/126)) ([bc5d465](https://github.com/openkcm/registry/commit/bc5d4652ae3e547243d415f4a6e7ad10219864db))
* disable custom id validations on tenant reads ([#150](https://github.com/openkcm/registry/issues/150)) ([c5eae94](https://github.com/openkcm/registry/commit/c5eae943573ee13a92b229d3532442e1691e46ad))
* improve error and debug logs in mapping svc  ([e6e07ae](https://github.com/openkcm/registry/commit/e6e07ae4993e6023106cda895876e8fd6643a113))
* remove tenant id filter from ListTenants method ([#156](https://github.com/openkcm/registry/issues/156)) ([76f86d0](https://github.com/openkcm/registry/commit/76f86d0b26a69a0802870e45d32cf16cadcb3907))
* return grpc status already_exists for RegisterSystem ([#159](https://github.com/openkcm/registry/issues/159)) ([37213a8](https://github.com/openkcm/registry/commit/37213a8708cc26ec4e7f0e499b86c1dafe1752af))
* update for latest orbital v0.5.0 ([#148](https://github.com/openkcm/registry/issues/148)) ([674b7ef](https://github.com/openkcm/registry/commit/674b7ef7eb6ba02af126c1710c48f40d16f3cd4b))


### Bug Fixes

* allowing regional system registration when tenant_id is provided… ([#128](https://github.com/openkcm/registry/issues/128)) ([178b6b7](https://github.com/openkcm/registry/commit/178b6b7d57f7fd01bdba7bbd3b96395e09d3fcae))
* **deps:** bump actions/setup-go from 6.3.0 to 6.4.0 in the actions-group group across 1 directory ([#158](https://github.com/openkcm/registry/issues/158)) ([1dd6677](https://github.com/openkcm/registry/commit/1dd6677ceb32be4b333e6d85aa6dd995bdb7e38b))
* **deps:** bump the gomod-group group across 1 directory with 5 updates ([#160](https://github.com/openkcm/registry/issues/160)) ([badae2d](https://github.com/openkcm/registry/commit/badae2dd7063399962137ec5a3f9e1f67e525269))
* **deps:** bump the gomod-group group across 1 directory with 6 updates ([#149](https://github.com/openkcm/registry/issues/149)) ([e69daf7](https://github.com/openkcm/registry/commit/e69daf730feef62d3c9eb04054c9ebbf8e363379))
* **deps:** bump the gomod-group group with 4 updates ([#152](https://github.com/openkcm/registry/issues/152)) ([9c2751e](https://github.com/openkcm/registry/commit/9c2751e409f2f205e881ccc21625c55b69bf1fef))
* do not automount service account token ([#147](https://github.com/openkcm/registry/issues/147)) ([91f733f](https://github.com/openkcm/registry/commit/91f733f0bc22e947f49a93f8b4dbe3ea959bfafb))
* improve dependabot config ([#145](https://github.com/openkcm/registry/issues/145)) ([ea94a1f](https://github.com/openkcm/registry/commit/ea94a1f9e1aa19d65933034ae5648b1c5efa9c6b))
* make failed tenant provisioning attempts recoverable ([2d3a57b](https://github.com/openkcm/registry/commit/2d3a57ba6b0bc336980d3db3b95691330a910929)), closes [#131](https://github.com/openkcm/registry/issues/131)

## [1.8.0](https://github.com/openkcm/registry/compare/v1.7.0...v1.8.0) (2026-01-15)


### Features

* added validation for map types ([#120](https://github.com/openkcm/registry/issues/120)) ([56d83ab](https://github.com/openkcm/registry/commit/56d83ab2cbf99da99d5845f6f0bb9e4aca0fe36b))

## [1.7.0](https://github.com/openkcm/registry/compare/v1.6.0...v1.7.0) (2025-12-18)


### Features

* refactor system into system and regional system ([#107](https://github.com/openkcm/registry/issues/107)) ([9f31e10](https://github.com/openkcm/registry/commit/9f31e1036444677ec0a049d0cc7e07a258907e70))

## [1.6.0](https://github.com/openkcm/registry/compare/v1.5.0...v1.6.0) (2025-12-10)


### Features

* add auth validation & updation for termination  ([4eb498c](https://github.com/openkcm/registry/commit/4eb498c2119efc29aeb21a0d82b9394670e99931))
* add paginated ListAuths endpoint ([85abd4b](https://github.com/openkcm/registry/commit/85abd4b26e42dd0a9781b67ea7badc790740217e))
* add regex validation for UserGroups ([#86](https://github.com/openkcm/registry/issues/86)) ([60488c2](https://github.com/openkcm/registry/commit/60488c2d7e3ce1877472d97bfaaf43cf452e264e))


### Bug Fixes

* Change Github URL ([#89](https://github.com/openkcm/registry/issues/89)) ([d0013a8](https://github.com/openkcm/registry/commit/d0013a8804dce2a18361d81e4a58bbb9346ca0c2))

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
