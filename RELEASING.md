# Releasing

Pushing a tag `vX.Y.Z` runs `.github/workflows/release.yml` on a macOS runner
(the macOS tray needs cgo, so darwin builds there; linux and windows
cross-compile). It:

1. runs the tests,
2. builds linux amd64/arm64, darwin arm64/amd64 and windows amd64 with
   GoReleaser, and publishes a GitHub release with the archives and
   `checksums.txt`,
3. attests the archives' build provenance,
4. updates the Homebrew cask in `marcel290820/homebrew-tap`,
5. pushes `PKGBUILD` and `.SRCINFO` to the AUR package `unsit-bin`,
6. pushes a winget manifest to your fork of `winget-pkgs` and opens a pull
   request against `microsoft/winget-pkgs`.

Tags with a pre-release suffix (`v1.1.0-rc1`) make a GitHub pre-release and
skip steps 4 to 6.

The `release-snapshot` job in `ci.yml` runs the same build without
publishing on every push, so a broken release config shows up before you tag.

## One-time setup

1. **Homebrew tap.** Create a public repository `marcel290820/homebrew-tap` with
   an initial commit (a README is enough). Create a fine-grained personal
   access token with *Contents: read and write* on that repository only, and
   save it as the repository secret `HOMEBREW_TAP_TOKEN`.

2. **AUR.** Create an account on <https://aur.archlinux.org>. Make a key for
   CI only, without a passphrase:

   ```sh
   ssh-keygen -t ed25519 -N '' -C 'unsit release' -f aur_unsit
   ```

   Add `aur_unsit.pub` to your AUR account (My Account > SSH Public Key), save
   the contents of `aur_unsit` as the secret `AUR_KEY`, then delete both files.
   The first release creates the `unsit-bin` package. The generated
   `PKGBUILD` has no `# Maintainer:` line; add `aurs[0].maintainers` to
   `.goreleaser.yaml` if you want one.

3. **winget.** Fork `microsoft/winget-pkgs` into your account. Create a
   classic personal access token with the `public_repo` scope (opening a pull
   request on a repository you don't own needs a classic token) and save it
   as the secret `WINGET_TOKEN`. The package identifier is
   `marcel290820.unsit`, with your GitHub name as the publisher; change
   `publisher` and `package_identifier` under `winget` in `.goreleaser.yaml`
   before the first release if you want something else, because it should
   not change later. Microsoft reviews each pull request, the first one more
   closely; until it merges, `winget install unsit` finds nothing and
   Windows users need `install.ps1`.

## Cutting a release

```sh
git tag -a v1.0.0 -m "v1.0.0"
git push origin v1.0.0
```

Then check that `brew install marcel290820/tap/unsit`, `yay -S unsit-bin` and,
once Microsoft has merged the pull request, `winget install unsit` install the
new version.

## Signing

The binaries are not code-signed. The cask removes the quarantine flag after
install, as GoReleaser recommends for unsigned casks, so macOS lets it run
without a Gatekeeper prompt. Apple may stop honoring that; the lasting fix is
signing and notarizing through GoReleaser's `notarize` section, which needs a
paid Apple Developer account. On Windows, `install.ps1` downloads with
PowerShell, which does not mark files as coming from the internet, so
SmartScreen does not prompt; signing would still help reputation checks.
