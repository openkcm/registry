# Changelog

## [1.9.0](https://github.com/openkcm/registry/compare/v1.8.0...v1.9.0) (2026-10-08)


### Features

* add AGENTS.md ([#177](https://github.com/openkcm/registry/issues/177)) ([8007413](https://github.com/openkcm/registry/commit/80074138072c3e95aa49855d9a1e7b263f5f5846))
* add key_limit to tenant configuration ([#240](https://github.com/openkcm/registry/issues/240)) ([0edf459](https://github.com/openkcm/registry/commit/0edf459e8c529356598aa0f505983c33c2825197))
* adopt protovalidate interceptor ([#194](https://github.com/openkcm/registry/issues/194)) ([17a9c3a](https://github.com/openkcm/registry/commit/17a9c3a14846604775d81ec6935892e48dbd9b98))
* Create Unit test result ([#126](https://github.com/openkcm/registry/issues/126)) ([bc5d465](https://github.com/openkcm/registry/commit/bc5d4652ae3e547243d415f4a6e7ad10219864db))
* disable custom id validations on tenant reads ([#150](https://github.com/openkcm/registry/issues/150)) ([c5eae94](https://github.com/openkcm/registry/commit/c5eae943573ee13a92b229d3532442e1691e46ad))
* improve error and debug logs in mapping svc  ([e6e07ae](https://github.com/openkcm/registry/commit/e6e07ae4993e6023106cda895876e8fd6643a113))
* introduce tenant config lifecycle with status tracking ([#238](https://github.com/openkcm/registry/issues/238)) ([a2a73dd](https://github.com/openkcm/registry/commit/a2a73ddf124c41f8851e5281d8e298c344fd7573))
* **metering:** emit system connection and mapping OTEL metrics ([#227](https://github.com/openkcm/registry/issues/227)) ([7b0a341](https://github.com/openkcm/registry/commit/7b0a3416e4e5f026ba2cde74cc52f03134102c10))
* **metering:** emit tenant.created, tenant.removed, and tenant.total metrics ([#225](https://github.com/openkcm/registry/issues/225)) ([44aa132](https://github.com/openkcm/registry/commit/44aa13287c7fc0ee4e4aa89ef748042f6eb247db))
* remove tenant id filter from ListTenants method ([#156](https://github.com/openkcm/registry/issues/156)) ([76f86d0](https://github.com/openkcm/registry/commit/76f86d0b26a69a0802870e45d32cf16cadcb3907))
* return an empty list instead of err NotFound on ListSystems ([#198](https://github.com/openkcm/registry/issues/198)) ([0a7534f](https://github.com/openkcm/registry/commit/0a7534f3e0da5995490f307dc31de8ca200cf959))
* return grpc status already_exists for RegisterSystem ([#159](https://github.com/openkcm/registry/issues/159)) ([37213a8](https://github.com/openkcm/registry/commit/37213a8708cc26ec4e7f0e499b86c1dafe1752af))
* tenant configuration ([#235](https://github.com/openkcm/registry/issues/235)) ([abf5ac8](https://github.com/openkcm/registry/commit/abf5ac851bf0450b54706dcc88e4fa0c9220145b))
* update for latest orbital v0.5.0 ([#148](https://github.com/openkcm/registry/issues/148)) ([674b7ef](https://github.com/openkcm/registry/commit/674b7ef7eb6ba02af126c1710c48f40d16f3cd4b))
* use static import-path meter name for OTEL instrumentation scope ([#241](https://github.com/openkcm/registry/issues/241)) ([8dd5bff](https://github.com/openkcm/registry/commit/8dd5bff193b102093f190f737c6d7d2ee79f6118))


### Bug Fixes

* allowing regional system registration when tenant_id is provided… ([#128](https://github.com/openkcm/registry/issues/128)) ([178b6b7](https://github.com/openkcm/registry/commit/178b6b7d57f7fd01bdba7bbd3b96395e09d3fcae))
* bump Go toolchain to v1.26.4 ([#181](https://github.com/openkcm/registry/issues/181)) ([3955233](https://github.com/openkcm/registry/commit/395523395b5e78c0f71ce86f9f55916e767affe1))
* bump toolchain ([#206](https://github.com/openkcm/registry/issues/206)) ([66d1a9c](https://github.com/openkcm/registry/commit/66d1a9c2b552e678c6315ea5546a452311a2f645))
* create an orbital job in ApplyAuth patch flow ([#237](https://github.com/openkcm/registry/issues/237)) ([862286f](https://github.com/openkcm/registry/commit/862286f7169298035233609ad51568539892d672))
* CxONE findings ([#175](https://github.com/openkcm/registry/issues/175)) ([1c2ecd7](https://github.com/openkcm/registry/commit/1c2ecd7584c5ebb812808b4e17e12025a7b05967))
* **deps:** bump actions/checkout from 6.0.2 to 6.0.3 in the actions-group group ([#180](https://github.com/openkcm/registry/issues/180)) ([8c5d1fa](https://github.com/openkcm/registry/commit/8c5d1fa24f6da310dfab1723ddb83e92037f2f88))
* **deps:** bump actions/checkout from 6.0.3 to 7.0.0 ([#192](https://github.com/openkcm/registry/issues/192)) ([241b683](https://github.com/openkcm/registry/commit/241b683571941015e582bede340aca3ce8ca6447))
* **deps:** bump actions/checkout from 7.0.0 to 7.0.1 in the actions-group group ([#208](https://github.com/openkcm/registry/issues/208)) ([d9ae18a](https://github.com/openkcm/registry/commit/d9ae18a49feaa61d60ea56bf08815425ac0db816))
* **deps:** bump actions/setup-go from 6.3.0 to 6.4.0 in the actions-group group across 1 directory ([#158](https://github.com/openkcm/registry/issues/158)) ([1dd6677](https://github.com/openkcm/registry/commit/1dd6677ceb32be4b333e6d85aa6dd995bdb7e38b))
* **deps:** bump actions/setup-go from 6.4.0 to 6.5.0 in the actions-group group ([#191](https://github.com/openkcm/registry/issues/191)) ([2bfb9e7](https://github.com/openkcm/registry/commit/2bfb9e763e616d5006eed1e73a7ed429a338afca))
* **deps:** bump actions/setup-go from 6.5.0 to 7.0.0 ([#202](https://github.com/openkcm/registry/issues/202)) ([31e0153](https://github.com/openkcm/registry/commit/31e01537bb2a89c4dda21016d350c3611af81e2f))
* **deps:** bump actions/upload-artifact from 7.0.0 to 7.0.1 in the actions-group group ([#161](https://github.com/openkcm/registry/issues/161)) ([761467d](https://github.com/openkcm/registry/commit/761467d6191ad7606c4d951fb3747925a0c055b2))
* **deps:** bump buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go from 1.36.11-20260415201107-50325440f8f2.1 to 1.36.11-20260709200747-435963d16310.1 ([#209](https://github.com/openkcm/registry/issues/209)) ([dc94ff6](https://github.com/openkcm/registry/commit/dc94ff6d4ded5b278ad105e1004fbb0acced41eb))
* **deps:** bump buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go from 1.36.12-20260709200747-435963d16310.1 to 1.36.12-20260825204119-511051f7f437.2 ([#231](https://github.com/openkcm/registry/issues/231)) ([185ae84](https://github.com/openkcm/registry/commit/185ae84fedeb0f7737ab9f210932e53c89eec336))
* **deps:** bump distroless/static-debian12 from `a932952` to `d093aa3` ([#173](https://github.com/openkcm/registry/issues/173)) ([a7f5928](https://github.com/openkcm/registry/commit/a7f59283b42dcb8156e9f7e7bd9acd6b0ddb94d7))
* **deps:** bump distroless/static-debian12 from `aef9602` to `f5b485e` ([#207](https://github.com/openkcm/registry/issues/207)) ([ee4ba84](https://github.com/openkcm/registry/commit/ee4ba84abaf583c829f171fa2aabab5739c9b646))
* **deps:** bump distroless/static-debian12 from `d093aa3` to `aef9602` ([#199](https://github.com/openkcm/registry/issues/199)) ([64a9147](https://github.com/openkcm/registry/commit/64a914710d81105f3d68485f1d4cc2b19930149c))
* **deps:** bump distroless/static-debian12 from `f5b485e` to `afa5c87` ([#222](https://github.com/openkcm/registry/issues/222)) ([e27cab5](https://github.com/openkcm/registry/commit/e27cab54523b408a1a540fd2b4e51a0b5d5d024b))
* **deps:** bump github.com/jackc/pgx/v5 from 5.9.1 to 5.9.2 ([#165](https://github.com/openkcm/registry/issues/165)) ([4bbcccb](https://github.com/openkcm/registry/commit/4bbcccb57f71e562e20ec264bc474aac53ce608e))
* **deps:** bump github.com/openkcm/common-sdk from 1.19.3 to 1.19.4 in the gomod-group group across 1 directory ([#242](https://github.com/openkcm/registry/issues/242)) ([e98c0a5](https://github.com/openkcm/registry/commit/e98c0a520ea1b9fbd67b6ab4fb05a58684a847b2))
* **deps:** bump google.golang.org/grpc from 1.80.0 to 1.81.0 in the gomod-group group ([#168](https://github.com/openkcm/registry/issues/168)) ([49f730f](https://github.com/openkcm/registry/commit/49f730f5d14cbd564f2999f9a36fd4afa47b97e8))
* **deps:** bump the gomod-group group across 1 directory with 13 updates ([#224](https://github.com/openkcm/registry/issues/224)) ([b9cf883](https://github.com/openkcm/registry/commit/b9cf8834368ca23585050eb081d49c891d7ecad5))
* **deps:** bump the gomod-group group across 1 directory with 2 updates ([#203](https://github.com/openkcm/registry/issues/203)) ([72fe036](https://github.com/openkcm/registry/commit/72fe0360ffa5bd4eadf684ca7970db24514d4eac))
* **deps:** bump the gomod-group group across 1 directory with 2 updates ([#234](https://github.com/openkcm/registry/issues/234)) ([f851f30](https://github.com/openkcm/registry/commit/f851f3057019c2577298a89bcd6efc24b0fe6688))
* **deps:** bump the gomod-group group across 1 directory with 4 updates ([#195](https://github.com/openkcm/registry/issues/195)) ([014161f](https://github.com/openkcm/registry/commit/014161f8027a1290534842647b5e3344fa48a68a))
* **deps:** bump the gomod-group group across 1 directory with 5 updates ([#160](https://github.com/openkcm/registry/issues/160)) ([badae2d](https://github.com/openkcm/registry/commit/badae2dd7063399962137ec5a3f9e1f67e525269))
* **deps:** bump the gomod-group group across 1 directory with 6 updates ([#149](https://github.com/openkcm/registry/issues/149)) ([e69daf7](https://github.com/openkcm/registry/commit/e69daf730feef62d3c9eb04054c9ebbf8e363379))
* **deps:** bump the gomod-group group across 1 directory with 6 updates ([#179](https://github.com/openkcm/registry/issues/179)) ([98d11a9](https://github.com/openkcm/registry/commit/98d11a928a9d2ef283f83cd3a05591fbb6167c11))
* **deps:** bump the gomod-group group with 2 updates ([#174](https://github.com/openkcm/registry/issues/174)) ([f6751c7](https://github.com/openkcm/registry/commit/f6751c79fa3f223dece4d7aa1fd3ecc3b2878ee5))
* **deps:** bump the gomod-group group with 2 updates ([#188](https://github.com/openkcm/registry/issues/188)) ([a19e7bb](https://github.com/openkcm/registry/commit/a19e7bbfbc5556211d6615a5a19960c8559c3216))
* **deps:** bump the gomod-group group with 2 updates ([#236](https://github.com/openkcm/registry/issues/236)) ([0696684](https://github.com/openkcm/registry/commit/06966842b4bfa4497ca792baf9605d362295c8a1))
* **deps:** bump the gomod-group group with 3 updates ([#229](https://github.com/openkcm/registry/issues/229)) ([91322c0](https://github.com/openkcm/registry/commit/91322c0ac4d821e1e13042ebe196c1e48e0f31b2))
* **deps:** bump the gomod-group group with 4 updates ([#152](https://github.com/openkcm/registry/issues/152)) ([9c2751e](https://github.com/openkcm/registry/commit/9c2751e409f2f205e881ccc21625c55b69bf1fef))
* **deps:** bump the gomod-group group with 4 updates ([#239](https://github.com/openkcm/registry/issues/239)) ([a245f39](https://github.com/openkcm/registry/commit/a245f3965db64b9abdef00e12692050dc4f80b48))
* **deps:** bump the gomod-group group with 5 updates ([#230](https://github.com/openkcm/registry/issues/230)) ([6e82c55](https://github.com/openkcm/registry/commit/6e82c559ae8e52247fbd7c136a68c9f68cfd5c1f))
* disable exhaustruct_v5 linter ([#228](https://github.com/openkcm/registry/issues/228)) ([c76e184](https://github.com/openkcm/registry/commit/c76e184fd96866682504d7a5887ce7fc449fe49e))
* disable profiling endpoint by default ([#232](https://github.com/openkcm/registry/issues/232)) ([8dff992](https://github.com/openkcm/registry/commit/8dff992dcc453db7cc6e8603210f4b96de29e8cd))
* do not automount service account token ([#147](https://github.com/openkcm/registry/issues/147)) ([91f733f](https://github.com/openkcm/registry/commit/91f733f0bc22e947f49a93f8b4dbe3ea959bfafb))
* flaky integration tests ([#185](https://github.com/openkcm/registry/issues/185)) ([ee35cf4](https://github.com/openkcm/registry/commit/ee35cf477cc1a34e1f28f6150b05431531780907))
* goroutine leak and dropped WithContext return value ([#167](https://github.com/openkcm/registry/issues/167)) ([aec3bea](https://github.com/openkcm/registry/commit/aec3bead8a57b041242600ed7b2cb6b29d9d9e8d))
* **helm:** make image.tag or image.digest mandatory ([#170](https://github.com/openkcm/registry/issues/170)) ([4e95b54](https://github.com/openkcm/registry/commit/4e95b54aa19c6dbc37b442e2e2644691116478f5))
* improve dependabot config ([#145](https://github.com/openkcm/registry/issues/145)) ([ea94a1f](https://github.com/openkcm/registry/commit/ea94a1f9e1aa19d65933034ae5648b1c5efa9c6b))
* improve error handling and logging in system mapping and unmapping ([#204](https://github.com/openkcm/registry/issues/204)) ([5d27d15](https://github.com/openkcm/registry/commit/5d27d152cf8567d5233500218c608fd56a67076e))
* integration tests ([#166](https://github.com/openkcm/registry/issues/166)) ([6e30834](https://github.com/openkcm/registry/commit/6e30834755ac85d0ba80d5c3036bd96e47b2ec5f))
* log errors on invalid tenant region ([#189](https://github.com/openkcm/registry/issues/189)) ([fd842b8](https://github.com/openkcm/registry/commit/fd842b8b159df640068c76f03c1922ff842fd99d))
* make Auth.Properties issuer key required ([#205](https://github.com/openkcm/registry/issues/205)) ([8cde590](https://github.com/openkcm/registry/commit/8cde5900dc3b55b96c0f7dad762eb2a9dbd324f4))
* make failed tenant provisioning attempts recoverable ([2d3a57b](https://github.com/openkcm/registry/commit/2d3a57ba6b0bc336980d3db3b95691330a910929)), closes [#131](https://github.com/openkcm/registry/issues/131)
* make RemoveAuth idempotent ([#201](https://github.com/openkcm/registry/issues/201)) ([cce8484](https://github.com/openkcm/registry/commit/cce8484e8a2ed740315a6ca176a7ee385114535c))
* more on flaky integration tests ([#187](https://github.com/openkcm/registry/issues/187)) ([291a56e](https://github.com/openkcm/registry/commit/291a56efa4a9484bf8db1d1c7b097a8605a53a77))
* race condition reported by CxONE ([#172](https://github.com/openkcm/registry/issues/172)) ([3617f6e](https://github.com/openkcm/registry/commit/3617f6e466614977cb7f2de9b36d73e2de65de80))
* support patch for ApplyAuth ([#197](https://github.com/openkcm/registry/issues/197)) ([164ed2d](https://github.com/openkcm/registry/commit/164ed2de5c06875b7aa6028940e62446a46b88fb))
* update github.com/google/cel-go ([#216](https://github.com/openkcm/registry/issues/216)) ([a53c161](https://github.com/openkcm/registry/commit/a53c161fc0897cf936275360c59e9cc1f4475da4))
* update golang.org/x/net and golang.org/x/text ([#210](https://github.com/openkcm/registry/issues/210)) ([86b0bff](https://github.com/openkcm/registry/commit/86b0bff8dbdd14ccd6f47c788c650d0eef558de3))
* vulnerabilities in golang.org/x/* ([#178](https://github.com/openkcm/registry/issues/178)) ([27c0295](https://github.com/openkcm/registry/commit/27c029533c3ec9f353c820856a845d0d72e0a062))

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
