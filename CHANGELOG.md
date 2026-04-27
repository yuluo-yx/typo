# Changelog

## [1.2.0](https://github.com/yuluo-yx/typo/compare/v1.1.0...v1.2.0) (2026-04-27)


### Features

* **engine:** add command options params support ([#130](https://github.com/yuluo-yx/typo/issues/130)) ([6d90d52](https://github.com/yuluo-yx/typo/commit/6d90d52a23444bc86e87ba88abd5bb30546451a8))
* **parser:** add generic stderr parser for broad CLI coverage ([#131](https://github.com/yuluo-yx/typo/issues/131)) ([712bb07](https://github.com/yuluo-yx/typo/commit/712bb07544159372f9e9379962feff67dad1bee0))


### Bug Fixes

* **codespell:** ignore test spell ([d8b2af4](https://github.com/yuluo-yx/typo/commit/d8b2af475e6dd204124433b72651df2731cabf62))


### Performance Improvements

* optimize perf ([#135](https://github.com/yuluo-yx/typo/issues/135)) ([91b30b9](https://github.com/yuluo-yx/typo/commit/91b30b9f3c3006a89ca15bae96040a9b712bf965))

## [1.1.0](https://github.com/yuluo-yx/typo/compare/v1.0.0...v1.1.0) (2026-04-23)


### Features

* **cmd:** add fix debug command ([#129](https://github.com/yuluo-yx/typo/issues/129)) ([fd14c6a](https://github.com/yuluo-yx/typo/commit/fd14c6a59e30b99a0aad1e6886e1eaa4c8ce5ac8))
* **cmd:** add typo stats command ([642daf1](https://github.com/yuluo-yx/typo/commit/642daf19452c60a5ac2a800e13d3f0cb59874b83))
* **enginee:** add env alias support ([#124](https://github.com/yuluo-yx/typo/issues/124)) ([d921b4d](https://github.com/yuluo-yx/typo/commit/d921b4d355d480699e9ffbc5b01477e97228ee01))
* **enginee:** add shell env context,  style ([#128](https://github.com/yuluo-yx/typo/issues/128)) ([0e6abbb](https://github.com/yuluo-yx/typo/commit/0e6abbb92d893992ff4e06029d46b4e443d49809))
* **hisroty:** add auto learn from usage history ([#127](https://github.com/yuluo-yx/typo/issues/127)) ([c26d2d0](https://github.com/yuluo-yx/typo/commit/c26d2d0c38af6a3c2e2075878cad974c41617871))


### Bug Fixes

* **enginee:** correct transposed nested subcommands ([#119](https://github.com/yuluo-yx/typo/issues/119)) ([7ce978c](https://github.com/yuluo-yx/typo/commit/7ce978c7cfdfcc97e0532f2e3d307f25411b90ee))

## [1.0.0](https://github.com/yuluo-yx/typo/compare/v0.2.0...v1.0.0) (2026-04-18)


### ⚠ BREAKING CHANGES

* **docs:** Establishes the v1.x stability contract. All CLI subcommands, flags, config keys, and shell integration behaviors documented in docs/reference/stability.md are now subject to semantic versioning guarantees.

### Features

* add type pkg ([#107](https://github.com/yuluo-yx/typo/issues/107)) ([aa76bde](https://github.com/yuluo-yx/typo/commit/aa76bde3a847dac315a574d73e86a0db638526e9))
* add utils pkg ([#108](https://github.com/yuluo-yx/typo/issues/108)) ([bc673ef](https://github.com/yuluo-yx/typo/commit/bc673ef417ab0351760b0aacb32bd40c21d1263e))
* **cloud:** add more cloud cli support ([#85](https://github.com/yuluo-yx/typo/issues/85)) ([2034415](https://github.com/yuluo-yx/typo/commit/2034415122bc908f6ef3d52ffa122b8537a759be))
* **command:** add typo command fix ([#34](https://github.com/yuluo-yx/typo/issues/34)) ([dbb3d5d](https://github.com/yuluo-yx/typo/commit/dbb3d5d6aaf065e331ede192921e289a5b3a1dd4))
* **commands:** add common IaC tools to DiscoverCommon ([#55](https://github.com/yuluo-yx/typo/issues/55)) ([0e5c9e7](https://github.com/yuluo-yx/typo/commit/0e5c9e7ac3130f21642a6983e1cc3facc4a5f954))
* **doctor:** update typo doctor command ([#87](https://github.com/yuluo-yx/typo/issues/87)) ([e360552](https://github.com/yuluo-yx/typo/commit/e360552f66daa48ceb7fd259e1f3697b29624083))
* **fish:** add fish integration ([#86](https://github.com/yuluo-yx/typo/issues/86)) ([40db08b](https://github.com/yuluo-yx/typo/commit/40db08b37bdc2f39bd0c93a8633b21f712f6df09))
* **install:** add brew install support ([#90](https://github.com/yuluo-yx/typo/issues/90)) ([6bcf273](https://github.com/yuluo-yx/typo/commit/6bcf2736b895ab1862b0cf7b4ad9994caf883119))
* migrate utility functions to internal/utils package ([#116](https://github.com/yuluo-yx/typo/issues/116)) ([a884838](https://github.com/yuluo-yx/typo/commit/a884838a7c8277d8bd04b1096f707e0c2a6d1c7f))
* move Rule and HistoryEntry to internal/types as stable data contracts ([#115](https://github.com/yuluo-yx/typo/issues/115)) ([10549cb](https://github.com/yuluo-yx/typo/commit/10549cb923ea0c75dfaaf49380413fc2165dc673))
* optimize shell & history ([#114](https://github.com/yuluo-yx/typo/issues/114)) ([5cf8136](https://github.com/yuluo-yx/typo/commit/5cf81361b29580d0a23aeef30af15c15201adc9e))
* optimize subcommanf logic ([d0d48f8](https://github.com/yuluo-yx/typo/commit/d0d48f89d7a39844250f63039b612e017ccfc771))
* optimize subcommanf logic ([21085cf](https://github.com/yuluo-yx/typo/commit/21085cff941490a00149f4f4a714fba99c6a21a1))
* support multi-level subcommand ([#105](https://github.com/yuluo-yx/typo/issues/105)) ([e3ed58c](https://github.com/yuluo-yx/typo/commit/e3ed58c04fd8d76e39cd32bd97985b9cc3802d3d))
* **win:** add windows download scripts and update related docs ([#72](https://github.com/yuluo-yx/typo/issues/72)) ([180dca7](https://github.com/yuluo-yx/typo/commit/180dca7e51b56821488f4b53be4b7c7eda43fa6f))


### Bug Fixes

* **bash:** fix bash 4.x integration error ([#79](https://github.com/yuluo-yx/typo/issues/79)) ([fbda5e2](https://github.com/yuluo-yx/typo/commit/fbda5e240329648b66e821ad67ffa665a23371c0))
* **ci:** reduce benchmark false positives with 10% threshold and rolling baseline ([#83](https://github.com/yuluo-yx/typo/issues/83)) ([1768b78](https://github.com/yuluo-yx/typo/commit/1768b78e003b38b51d4036689fcbf8103519270b))
* **lint:** fix code lint error ([b8a3652](https://github.com/yuluo-yx/typo/commit/b8a36521e8d7d22f9983ed49605721dd6e44bac5))
* **make:** fix repeat makefile target ([c665d74](https://github.com/yuluo-yx/typo/commit/c665d74a3cdd059ef2df5261d87a1f74c1bbbc7a))
* **windows:** fix windows install script bug ([#113](https://github.com/yuluo-yx/typo/issues/113)) ([094b054](https://github.com/yuluo-yx/typo/commit/094b054f76fb8ed1e5e6f19b104c2c3c9f46a160))


### Performance Improvements

* **engine:** eliminate redundant distance computations on hot path ([#74](https://github.com/yuluo-yx/typo/issues/74)) ([16f7ef4](https://github.com/yuluo-yx/typo/commit/16f7ef4fb887b65cd613b8ba3d04e5809787a7f4)), closes [#44](https://github.com/yuluo-yx/typo/issues/44)
* optimize command tree performance ([#111](https://github.com/yuluo-yx/typo/issues/111)) ([d8458ce](https://github.com/yuluo-yx/typo/commit/d8458ce30dcd6507245196eac587878169bc96cd))


### Miscellaneous Chores

* **docs:** define v1.x stability and compatibility contract ([#89](https://github.com/yuluo-yx/typo/issues/89)) ([287eecd](https://github.com/yuluo-yx/typo/commit/287eecd5a2fb2fb4af0b251bec5dca7bb1f95db9))

## [0.2.0](https://github.com/yuluo-yx/typo/compare/v0.1.1...v0.2.0) (2026-04-06)

### Features

* **command:** adapt windows powershell ([#42](https://github.com/yuluo-yx/typo/issues/42)) ([d742c4f](https://github.com/yuluo-yx/typo/commit/d742c4fa3055a1b52db5c64774e706970fe9d7ae))
* **command:** add cloud provider CLIs to common commands list, for aws gcloud az ([#23](https://github.com/yuluo-yx/typo/issues/23)) ([e5b6987](https://github.com/yuluo-yx/typo/commit/e5b698720bbacb72ad290cfe8f4bba9d968d07d3))
* **config:** add typo config file ([#24](https://github.com/yuluo-yx/typo/issues/24)) ([e77167d](https://github.com/yuluo-yx/typo/commit/e77167d4e2b95543b102e9f1a75b64c03cc248ca))
* **config:** support rule enable and disable config ([#33](https://github.com/yuluo-yx/typo/issues/33)) ([21797e6](https://github.com/yuluo-yx/typo/commit/21797e657a87ade8b1ee6a270f7dbcbe4c12f206))
* **install:** add bash shell integration ([#26](https://github.com/yuluo-yx/typo/issues/26)) ([6363eaa](https://github.com/yuluo-yx/typo/commit/6363eaa4f6f586c8e2bdf70de189985bcc4dda1e))
* **release:** add windows platform binary ([#40](https://github.com/yuluo-yx/typo/issues/40)) ([57109d4](https://github.com/yuluo-yx/typo/commit/57109d4934639db53a9d0130c96b8ac178d31271))

### Bug Fixes

* **ci:** fix golanglint not run bug ([#45](https://github.com/yuluo-yx/typo/issues/45)) ([5070c0c](https://github.com/yuluo-yx/typo/commit/5070c0c132f9f9cc6a05082588bef8c318a96504))
* **ci:** gate release upload on release tag output ([73270f8](https://github.com/yuluo-yx/typo/commit/73270f885700543c5a258be3bdeda134fcb69814))
* **ci:** remove check in release-please action ([e5af183](https://github.com/yuluo-yx/typo/commit/e5af1831d67c0d035c5d40a747f8424b5a41655c))

## [0.1.1](https://github.com/yuluo-yx/typo/compare/v0.1.0...v0.1.1) (2026-03-31)

### Bug Fixes

* **release:** update release-please to include build phase ([39ca46a](https://github.com/yuluo-yx/typo/commit/39ca46aa5b6b79fe4e80698c205acf093d060ca9))
* **release:** update release-please to include build phase ([a202c2b](https://github.com/yuluo-yx/typo/commit/a202c2bbb53f8aea7e36fd75b44448ac5cb917da))

## [0.1.0](https://github.com/yuluo-yx/typo/compare/v0.0.1...v0.1.0) (2026-03-31)

### Features

* **release:** integrate semver release automation ([c7ee665](https://github.com/yuluo-yx/typo/commit/c7ee66528d6e99ada6e2975c300d21af2d779ebe))

### Bug Fixes

* increase default help timeout to 1s to stabilize CI ([2ebcb10](https://github.com/yuluo-yx/typo/commit/2ebcb1073ad802adbd811178d89cb7e2e4ab2790))
* parse comma-separated subcommands for npm v6-v11 help formats ([b1b4d5d](https://github.com/yuluo-yx/typo/commit/b1b4d5d6b671f32ae82e0bd5c8afb4a839887f9f))
* parse comma-separated subcommands for npm v6-v11 help formats ([d1e043c](https://github.com/yuluo-yx/typo/commit/d1e043cb60f9886ef6b9c60ebcdbb347619cc8ae))
* **release:** add bootstrap sha so release-please has a commit baseline ([69c5182](https://github.com/yuluo-yx/typo/commit/69c518234db7f695e4230eea2a2195ee0641835a))
* **release:** add bootstrap sha so release-please has a commit baseline ([6d11ae8](https://github.com/yuluo-yx/typo/commit/6d11ae834404a85f5b478c6e82ba9f17c0ab998a))
* **release:** security enhancement in ci to pin SHA-256 image tag ([0a6b2ab](https://github.com/yuluo-yx/typo/commit/0a6b2ab79d26883e0314ef0a59d676841cd60709))
* **release:** update release-please to include config location ([ca66bad](https://github.com/yuluo-yx/typo/commit/ca66bad483fc99e714a69080d233e8acf72174db))
* **release:** update release-please to include config location ([bf950ab](https://github.com/yuluo-yx/typo/commit/bf950ab9f98870cfd4b4ecf7c2b1db1b2251c37e))
