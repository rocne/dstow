# Changelog

## [0.2.0](https://github.com/rocne/dstow/compare/v0.1.2...v0.2.0) (2026-07-19)


### Features

* cobra-generated, themed help from the canonical content ([#98](https://github.com/rocne/dstow/issues/98)) ([056869c](https://github.com/rocne/dstow/commit/056869c1fbffbc8433e56e50400140c97a92e6dd))
* color the prose slots — help shows color by default and themed ([#100](https://github.com/rocne/dstow/issues/100)) ([154388c](https://github.com/rocne/dstow/commit/154388c257419c5da2f88d219b2963a398413a2a))
* preset roster = four catppuccin flavors, Whiskers-generated ([#105](https://github.com/rocne/dstow/issues/105)) ([#106](https://github.com/rocne/dstow/issues/106)) ([ffbb1ad](https://github.com/rocne/dstow/commit/ffbb1ad30cac735f3d051ff5e19c90dcf2bc00a1))
* quiet underlined headings in catppuccin-mocha — structure over tint ([#104](https://github.com/rocne/dstow/issues/104)) ([cd17e6e](https://github.com/rocne/dstow/commit/cd17e6e1162873ab483d69e215b95937b2094c32))


### Bug Fixes

* catppuccin-mocha names to blue — lavender was indistinguishable from mauve headings ([#103](https://github.com/rocne/dstow/issues/103)) ([a85a78e](https://github.com/rocne/dstow/commit/a85a78e31a982d8d96e72da12c88b39010197a3a))

## [0.1.2](https://github.com/rocne/dstow/compare/v0.1.1...v0.1.2) (2026-07-19)


### Bug Fixes

* **release:** drop cask manpages/completions dstow doesn't ship yet ([#47](https://github.com/rocne/dstow/issues/47)) ([#94](https://github.com/rocne/dstow/issues/94)) ([f4e3977](https://github.com/rocne/dstow/commit/f4e3977891186cb30ab0bb0cb5cb6c8e176a22fa))

## [0.1.1](https://github.com/rocne/dstow/compare/v0.1.0...v0.1.1) (2026-07-19)


### Bug Fixes

* **signing:** adopt cosign v3 Sigstore bundle (release-ci[#45](https://github.com/rocne/dstow/issues/45), D35) ([#91](https://github.com/rocne/dstow/issues/91)) ([8038bd8](https://github.com/rocne/dstow/commit/8038bd8421c39246d0eb8686aab78203eadc3772))

## 0.1.0 (2026-07-19)


### Features

* add internal/cli, the command surface and composition root ([#74](https://github.com/rocne/dstow/issues/74)) ([8edd1b9](https://github.com/rocne/dstow/commit/8edd1b966b6dff72bb9e3ea1e6a77126d164b140))
* add internal/config, the four-level chain and stow compat ([#60](https://github.com/rocne/dstow/issues/60)) ([b420f09](https://github.com/rocne/dstow/commit/b420f09cc9af73740c2782eb2ffbaed13f9cd517))
* add internal/hooks, the lifecycle and context contract ([#66](https://github.com/rocne/dstow/issues/66)) ([044deaa](https://github.com/rocne/dstow/commit/044deaa34870e7e9c9ad0772a25ffc1ce741bc93))
* add internal/ignore and internal/engine, the gostow seams ([#65](https://github.com/rocne/dstow/issues/65)) ([15aa7b3](https://github.com/rocne/dstow/commit/15aa7b3bfa8158c94bcf048365cb815c95abb15e))
* add internal/ledger, the current-state index ([#61](https://github.com/rocne/dstow/issues/61)) ([02d50d2](https://github.com/rocne/dstow/commit/02d50d2982f685faf507c82725123e94caa50359))
* add internal/name, the pure naming package ([#57](https://github.com/rocne/dstow/issues/57)) ([809bf17](https://github.com/rocne/dstow/commit/809bf17997ec1dd512df9874dec55bc622f0d98d))
* add internal/repo and internal/git, the repo set and the git port ([#62](https://github.com/rocne/dstow/issues/62)) ([82c4da3](https://github.com/rocne/dstow/commit/82c4da31bc32648f3ceedf123643980529a3df0b))
* add internal/ui, the printer, palette, and theming stack ([#59](https://github.com/rocne/dstow/issues/59)) ([c53a251](https://github.com/rocne/dstow/commit/c53a2512b08235ec18aa7f4c388d3c5aa66db7ce))
* add ops deploy verbs and adopt, the app core's deploy half ([#67](https://github.com/rocne/dstow/issues/67)) ([3d53bd4](https://github.com/rocne/dstow/commit/3d53bd4dc0cdde2ce07444e96b4e392e11391027))
* add ops maintenance verbs (check, clean, rebuild) ([#68](https://github.com/rocne/dstow/issues/68)) ([a7a3493](https://github.com/rocne/dstow/commit/a7a34932c5a034097b27fe5708534d3120a964e0))
* add ops repo verbs, snippet rc, and colors theme ([#71](https://github.com/rocne/dstow/issues/71)) ([8386238](https://github.com/rocne/dstow/commit/83862383af9a27ebac52df6e7abef7b17b1c3d32))
* add ops views — list, info, status — and the O9 name helper ([#70](https://github.com/rocne/dstow/issues/70)) ([6c12c2e](https://github.com/rocne/dstow/commit/6c12c2e56ce28d8b5eefb1b4dace049340c49b87))
* emit the vendored snippet.sh from dstow snippet rc ([#87](https://github.com/rocne/dstow/issues/87)) ([d4d0f60](https://github.com/rocne/dstow/commit/d4d0f6005562486ec29b6e17d69f72ac38325202))
* enumerate symlinked directories as packages ([#63](https://github.com/rocne/dstow/issues/63)) ([321d564](https://github.com/rocne/dstow/commit/321d56489aefb4732c5e41cadc8962191a08bec1))
* support --version at the root (D30 contract) ([#80](https://github.com/rocne/dstow/issues/80)) ([16ee744](https://github.com/rocne/dstow/commit/16ee74457fcc0993123b1bdca35030f4603adcb1))
* vendor the canonical install.sh and rc snippet (release-ci[#1](https://github.com/rocne/dstow/issues/1), [#64](https://github.com/rocne/dstow/issues/64)) ([#78](https://github.com/rocne/dstow/issues/78)) ([f436a06](https://github.com/rocne/dstow/commit/f436a065f5a2458916409eb7ca8a17f99a97e2af))
