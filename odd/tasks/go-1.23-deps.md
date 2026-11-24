# Go 1.23 floor + dependency refresh

Feature: raise the module's declared Go floor to 1.23 and bring every direct dependency to its newest release compatible with that floor, keeping the module buildable, tested and with a green CI matrix.
Branch: `chore/go-1.21-modernization` (continues the open PR #1).

## Specs

- **S1** — Bump the direct dependencies. User's words: «sigue con los bumps».
- **S2** — Declared floor is Go 1.23. User's words: «ahora si, subelo a 1.23».
- **S3** — Newest release per module that requires at most `go 1.23.0`: `golang.org/x/text` v0.28.0, `golang.org/x/term` v0.34.0, `golang.org/x/sys` v0.35.0, `github.com/stretchr/testify` v1.12.1, `github.com/mattn/go-isatty` v0.0.24, `github.com/mattn/go-colorable` v0.1.16, `github.com/creack/pty` v1.1.24, `github.com/mgutz/ansi` v0.0.0-20200706080929-d51e80ef957d. `github.com/Netflix/go-expect` and `github.com/kballard/go-shellquote` are already at their newest published version and stay unchanged; `github.com/hinshun/vt10x` moves to v0.0.0-20220301184237-5011da428d02.
- **S4** — The module builds, vets, formats and its full test suite passes at the new versions: `go build ./...`, `go vet ./...`, `gofmt -l .` empty, `go test -count=1 ./...` green.
- **S5** — The CI matrix reflects the new floor (`1.23` + `stable`) and every job in it is green. The pre-existing `test (macos-latest, 1.21)` failure must be resolved, not inherited.

## Tasks

- **T1** (S2, S3) — `go.mod`/`go.sum`: floor to `go 1.23.0`, all direct bumps applied, tidied.
- **T2** (S4) — Local verification: build, vet, gofmt, full test suite, ignored example programs.
- **T3** (S5) — `.github/workflows/test.yml`: matrix `["1.23", "stable"]`, macOS leg on the stable toolchain only, comment records why.
- **T4** (S3) — `hinshun/vt10x` to v0.0.0-20220301184237-5011da428d02, after independent verification found it pinned one release behind.
- **T5** (S1, S4) — `TestTitle` in `transform_test.go`: locks the exported `Title` contract that the `x/text` upgrade crosses, with expectations taken from a base-vs-candidate differential.

Work-unit commit: `3d0a6de`.

Delivery: stacked PRs to `master` (`chain_strategy=stacked-to-main`).

- **First slice** — `6bd0e3a..562e544` on `chore/modernize-go-1.21`, Go 1.21 modernization, 9 commits. It carries `562e544`, which keeps the pre-1.24 Go leg off macOS: without it that slice shipped a red `test (macos-latest, 1.21)` leg, because the `LC_UUID` problem in L4 is not specific to Go 1.23 — every toolchain older than 1.24 links binaries that image refuses to load.
- **Second slice** — this feature, on `chore/go-1.23-deps`, based on the first slice's branch so its diff shows only the dependency change plus `TestTitle` and this document. Retarget it to `master` once the first slice merges.
- Both slices land well under the ~400-line review advisory on their own; the 515-line total measured before the split was the reason for chaining them.
- `chore/go-1.21-modernization` stays on the remote at `45f9685` as the pre-split branch. The PR that tracked it is closed with a pointer to both slices, because force-pushing a published branch was denied by policy.

## Log

- **L1** — «sigue con los bumps»
- **L2** — «ahora si, subelo a 1.23» (choosing the 1.23 floor over the 1.21-compatible maximum and over `@latest`/1.26)
- **L3** (evidence) — Proxy data: `x/text` v0.23.0, `x/term` v0.30.0 and `x/sys` v0.31.0 are the first releases declaring `go 1.23.0`; `x/text` v0.42.0+, `x/term` v0.46.0+ and `x/sys` v0.48.0+ declare `go 1.26.0`. So 1.23 caps the three `x/*` modules at v0.28.0 / v0.34.0 / v0.35.0.
- **L4** (evidence) — CI run `45480424023` at `e2c5dca`: only `test (macos-latest, 1.21)` failed, with `dyld: missing LC_UUID load command` + `signal: abort trap`. Cause: the Go linker emits `LC_UUID` on Mach-O by default only from Go 1.24 (go.dev/doc/go1.24, golang/go#68678, commit 6a4feb5); the go1.23 backport emits it only with `-B`. Raising the floor to 1.23 therefore does not fix that leg.
- **L5** (decision) — macOS is tested on the `stable` toolchain only, and the Go 1.23 compatibility leg runs on Linux and Windows, where the same toolchain passes. Chosen over `-ldflags=-B gobuildid` on that leg: the flag is a linker workaround for an old-toolchain/new-OS mismatch, and keeping it in the matrix would silently mask future `LC_UUID` regressions.
- **L6** (correction) — Independent verification returned S3 as fail: `hinshun/vt10x` was pinned at v0.0.0-20220119200601-820417d04eec while `@latest` is v0.0.0-20220301184237-5011da428d02. The first draft of S3 claimed it was already newest; that claim was wrong, and the version now moves. Re-verified: `go mod tidy -diff` clean, suite green.
- **L7** (evidence) — `Title` had no dedicated test and no assertion on its output (`TestComposeTransformers` called it, but compared the result through a following `ToLower`, which masks casing regressions). It is the exported surface the `x/text` v0.4.0 -> v0.28.0 jump crosses (`cases.Title(language.English)`). A base-vs-candidate differential over 16 inputs (apostrophes, ligatures, Greek, `ß`, dotted `İ`, whitespace, non-string) produced byte-identical output, so the new `TestTitle` records pre-existing behaviour rather than blessing a change. The same expectations were re-run against `x/text` v0.4.0, v0.13.0, v0.22.0, v0.28.0 and v0.34.0: identical in all five, so the pin is not fitted to one release.
- **L8** (correction) — Verification also flagged the PR wording: `go-spew`, `go-difflib` and `gopkg.in/yaml.v3` did not leave the module graph entirely; they left `go.mod`'s require blocks and `go.sum`'s `h1:` lines because testify v1.12.1 reaches them only on its test path. They are not imported by this library outside `_test.go` files.
