# Post-modernization backlog: terminal hardening, CI tooling, cosmetics

Feature: close the three pending items of the fork analysis, in the order the maintainer chose, one commit per item.
Branch: `chore/post-modernization-backlog`, cut from `master` = `69b0646`.

## Specs

- **S1** — Work in the stated order: terminal hardening, then CI tooling, then cosmetic modernization. User's words: «entonces vamos en el orden que me das».
- **S2** — One commit per item, made when that item is done. User's words: «realiza commit cuando termines cada punto».
- **S3** (item 1, terminal hardening) — Three defects, all in `terminal/`:
  - Nothing in the repository imports `os/signal` today, so an interruption between `SetTermMode` and `RestoreTermMode` leaves the caller's terminal in raw mode. The package must restore the previous terminal state when the process is interrupted, including when the interruption arrives while a prompt is reading.
  - `terminal/runereader_posix.go` reaches the kernel through `syscall.Syscall6` with ioctl request numbers hand-copied per architecture (`runereader_linux.go`, `runereader_ppc64le.go`, `runereader_bsd.go`). It must use `golang.org/x/sys/unix`, which is already a direct dependency.
  - `runereader_posix.go` is tagged `!windows`, so it is compiled for platforms where `syscall.Termios` does not exist (aix, solaris, illumos, plan9, js/wasm). The build tags must admit only the platforms the file can actually support.
- **S4** (item 4, CI tooling) — Add what the analysis listed as missing: a `golangci-lint` configuration plus a CI job that runs it; dependency update automation; and a release workflow so the fork can tag a version. A `Makefile` is deliberately not added: the three recipes it would wrap are already single commands, and `justfile`-style indirection was not asked for.
- **S5** (item 3, cosmetics) — 56 `interface{}` in non-test files (74 including tests) become `any`; the 9 files carrying a legacy `// +build` line next to `//go:build` lose the legacy line; `io/ioutil` in `editor.go` becomes `os.CreateTemp`/`os.ReadFile`. No behaviour change and no public API change.
- **S6** — After each item the repository still passes `gofmt -l .` (empty), `go vet ./...`, `go build ./...`, the example and manual-test programs, and `go test -count=1 ./...`. The terminal change additionally needs independent verification, because no existing test observes signal handling: item 6 of the risk list applies.

## Tasks

- **T1** (S3, S6) — Terminal hardening: signal-driven restore, `x/sys/unix` ioctls, build-tag scope. Route: writer + independent verifier.
- **T2** (S4, S6) — CI tooling: golangci-lint config and job, dependabot, release workflow.
- **T3** (S5, S6) — Cosmetic sweep: `interface{}` → `any`, legacy build tags, `io/ioutil`.

## Log

- **L1** — «entonces vamos en el orden que me das, realiza commit cuando termines cada punto»
- **L2** (context accepted by the maintainer) — The pending list this feature works through: (1) terminal hardening — no `os/signal`, hand-pinned ioctls, over-broad `!windows` tag; (4) CI tooling — no golangci config or job, no dependabot, no release workflow, no Makefile; (3) cosmetics — 56 `interface{}` in non-test code, 9 legacy `// +build` lines, `io/ioutil` in `editor.go`.
- **L3** (correction) — An earlier PR body claimed `editor.go` was "`io/ioutil`-free". It is not: `editor.go:148` uses `ioutil.TempFile` and `editor.go:204` uses `ioutil.ReadFile`. The commit message never said so and the merged PR body was corrected. This is why S5 lists `io/ioutil` as pending.
- **L4** (scope note) — The `io/ioutil` conversion landed inside T2, not T3: staticcheck's `SA1019` fires on it as soon as the linter is enabled, so leaving it for T3 would have left the T2 gate red. S5 for T3 is therefore `interface{}` → `any` and the nine legacy `// +build` lines only.
- **L5** (advisory corrections, after independent verification) — Three findings were fixed before committing T2: the release workflow is now idempotent (`gh release view` first, so re-running does not fail on HTTP 422), it refuses tags that are not `v2.x.y` (the `/v2` module path means a `v1.x.y` tag is not a version of this module), and the `govet.inline` comment no longer justifies itself with a diagnostic that the `ioutil` conversion removed.
- **L6** (decision) — Two `staticcheck` style checks are excluded in `.golangci.yml` with the reason written there: `ST1012` wants an exported symbol renamed (`terminal.InterruptErr`), and `ST1005` wants error text downcased for messages users already see. Both are contract changes, not lint fixes. No linter is disabled and no `//nolint` exists anywhere in the source.
- **L7** (correction, T3) — S6 made the run of T3 depend on `go doc -all` being byte-identical before and after. That gate is unsatisfiable: Go 1.27 renders a type alias with the spelling used in the source, so every `any` shows as `any`, and `GODEBUG=gotypesalias` no longer exists to force the old rendering. Replaced by a stronger proof: every changed line in the diff must equal its previous line with `interface{}` replaced by `any`, checked hunk by hunk, plus a `go doc -all` comparison with both alias spellings canonicalised. Result: 74/74 substitutions, 9 build-tag deletions, no other change, no signature or method-set difference.
- **L8** (follow-up, SIGQUIT) — The one gap S3 left open is closed: `SIGQUIT` joins the interruption set, so a Ctrl-\ during a prompt now restores the terminal instead of leaving it raw. Unlike the other three signals, the re-raised SIGQUIT reaches the Go runtime's own handler, which prints the goroutine dump and exits with status 2 rather than dying by the signal — verified with a standalone probe before writing the test, and pinned by asserting that exit status, because the dump goes to the shared stderr. The subtest is load-bearing: with SIGQUIT removed from `interruptedSignals` it fails with `terminal left with Lflag=0x8a30 … want 0x8a3b`.
