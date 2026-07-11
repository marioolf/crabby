# Releasing Crabby

Releases are automated by [`.github/workflows/release.yml`](../.github/workflows/release.yml).
Pushing a `v*` tag builds every platform and publishes a GitHub Release with the
binaries attached.

## Cutting a release

1. **Bump the version.** Edit `VERSION` (no leading `v`), e.g. `0.2.0`.
2. **Update the changelog.** Move items from `## [Unreleased]` into a new
   `## [0.2.0] - YYYY-MM-DD` section in `CHANGELOG.md` and update the link
   references at the bottom.
3. **Commit** both files.
4. **Tag and push:**

   ```bash
   git tag v0.2.0     # must equal "v" + the VERSION file
   git push origin main --tags
   ```

That's it. The workflow verifies the tag matches `VERSION`, cross-compiles all
targets, packages them, and creates the release. If the tag and `VERSION`
disagree, the workflow fails on purpose.

## Release assets

Asset names are stable (no version in the filename) so the installers can always
fetch `releases/latest/download/<asset>`:

| Platform      | Asset                        | Contains     |
| ------------- | ---------------------------- | ------------ |
| Linux amd64   | `crabby_linux_amd64.tar.gz`  | `crabby`     |
| Linux arm64   | `crabby_linux_arm64.tar.gz`  | `crabby`     |
| Windows amd64 | `crabby_windows_amd64.zip`   | `crabby.exe` |
| Windows arm64 | `crabby_windows_arm64.zip`   | `crabby.exe` |
| —             | `checksums.txt`              | SHA-256 sums |

The Windows asset is the WSL-forwarding wrapper; the Linux asset is the real
crabby binary.

## Testing the packaging locally

```bash
make release      # writes archives + checksums.txt into dist/
```

## First release checklist

Before the very first `v0.1.0` tag:

- Push the repository to `github.com/<owner>/crabby`.
- If your repo is not `marioolf/crabby`, update the `CRABBY_REPO` default in
  `install.sh` / `install.ps1` and the URLs in `README.md`.
- No secrets to configure: the workflow uses the automatic `GITHUB_TOKEN`.
  Ensure **Settings → Actions → General → Workflow permissions** is set to
  "Read and write permissions" (required for creating releases).
