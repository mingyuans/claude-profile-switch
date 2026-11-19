# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project

`ccs` is a Go CLI that switches Claude Code accounts by toggling `CLAUDE_CONFIG_DIR`. See `README.md` for usage and layout.

- Build / test / lint: `make build`, `make test`, `make lint`
- Run `go vet ./...` and `go test ./...` before committing.

## Release rules

- Versions are tags of the form `vMAJOR.MINOR.PATCH` (the release workflow rejects anything else).
- **Every release bumps only the PATCH number** (`x.y.Z`): e.g. `v0.1.2` → `v0.1.3`, regardless of whether the changes are features or fixes. Do not bump MAJOR or MINOR unless the maintainer explicitly asks for it.
- Release from the latest `main` after all intended PRs are merged:
  1. `git checkout main && git pull --ff-only`
  2. `go test ./...`
  3. Find the current highest tag: `git tag --sort=-v:refname | head -1`
  4. `git tag -a vX.Y.(Z+1) -m "<summary of changes>" && git push origin vX.Y.(Z+1)`
- Pushing the tag triggers `.github/workflows/release.yml`, which tests, cross-compiles `ccs-{darwin,linux}-{amd64,arm64}.tar.gz`, writes `checksums.txt` and creates the GitHub Release. Confirm the run succeeded with `gh run list` / `gh release view <tag>`.
- Asset names (`ccs-<os>-<arch>.tar.gz`) and `checksums.txt` are consumed by `install.sh` and `ccs update`; keep them stable.
