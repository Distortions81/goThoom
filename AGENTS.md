# AGENTS

## Working rules

- Read this file and `README.md` when starting a new session.
- Keep solutions simple and focused. Ask before making a consequential
  assumption.
- Do not create branches or push without explicit permission.
- After triggering a GitHub release build, report the release and workflow
  links, then stop. Do not monitor or poll the build, wait for completion, or
  watch logs unless the user explicitly asks.
- Preserve unrelated user changes in a dirty worktree.
- Do not increment versions in `GT_Players.json`, `settings.json`, or
  `characters.json`; those are updated manually.

## Project layout

- The Go module and client are under `source/` (`module gothoom`).
- Use Go 1.26.6 from the official Go distribution. Do not use the system
  `golang-go` package.
- Running the desktop client requires a display. Use Xvfb in a headless
  environment.

## Clean setup

Always extract the prebuilt resource bundle before building from a clean
checkout:

```sh
curl -fsSLO https://m45sci.xyz/u/dist/goThoom/gothoom_deps.tar.gz
tar -C source -xzf gothoom_deps.tar.gz
./build-scripts/setup_dev_env.sh
```

`setup_dev_env.sh` installs the Debian/Ubuntu build dependencies, installs the
required Go version, starts Xvfb when needed, and runs the standard checks.

If a change adds system or bundled-data dependencies, update the setup process,
regenerate the resource archive, and re-share `gothoom_deps.tar.gz`.

## Validation

Run focused tests while working, then normally finish with:

```sh
cd source
go test ./...
```

Use `go build` for a normal client compile. Also run `git diff --check` before
handoff. Validate release or cross-platform changes with the relevant build
script rather than assuming a normal Linux build covers them.

## Release workflow

- A release number must exist in both `source/data/versions.json` and
  `source/data/changelog/<number>.txt` before it is tagged. Keep the versions
  strictly increasing and use the current Clan Lord version for `CLVersion`.
- Write the changelog as user-facing release notes covering changes since the
  previous release. The same file is embedded in the client and published as
  the GitHub release body.
- Run the normal validation, commit the release metadata, and push it to
  `main` before creating the release. Committing and pushing still require the
  user's explicit permission.
- Confirm `main` is clean and matches `origin/main`, and confirm the intended
  tag and release do not already exist. Never reuse or force-move an existing
  release tag.
- Create a test release from the exact current commit, using the numbered
  changelog for its initial notes. For example:

  ```sh
  release_number=44
  release_tag="test${release_number}"
  gh release create "$release_tag" \
    --target "$(git rev-parse HEAD)" \
    --title "$release_tag" \
    --notes-file "source/data/changelog/${release_number}.txt"
  ```

- `.github/workflows/build-binaries.yml` validates that the numbered changelog
  exists, synchronizes the GitHub release body from it, builds the archives,
  and uploads the release assets.
- After the release event triggers the build, look up its workflow URL once
  and report it with the release URL. Do not monitor or poll build status,
  watch logs, or wait for completion unless the user explicitly asks.

## Script API

- Any function, type, variable, constant, or method exposed to scripts must
  also exist as an editor stub in `source/gt2/pluginapi.go`.
- After changing that contract, run:

  ```sh
  cd source/gt2
  go generate ./...
  ```

  Keep the regenerated API reference and editor-support data with the change.
- `gt2.Store` data is private to a script but shared across its characters.
  Character-specific data must include the normalized current character in its
  keys and should be read only after the character is known.
- `gt2.Repeat` and `gt2.Wait` are session-only. Persist dates or due times when
  scheduled work must survive reloads or app restarts.

## Useful build helpers

Run helpers from the repository root:

- `build-scripts/build_binaries.sh`: container/release cross-platform builds.
- `build-scripts/build_binaries_local.sh`: local cross-platform builds.
- `build-scripts/build_script_template.sh`: VS Code script template ZIP.
- `build-scripts/build_wasm.sh`: separate WebAssembly package.
- `build-scripts/update_screenshot.sh`: README and website screenshots; needs
  `cwebp` from the Debian/Ubuntu `webp` package.

## Ebitengine

- Target Ebitengine 2.9.x and do not introduce APIs deprecated in that release.
- Prefer `vector.Fill*`, `vector.StrokePath`, and `vector.Path.Add*` helpers.
- Use `audio.ResampleReader` or `audio.ResampleReaderF32`, not the deprecated
  `audio.Resample` helpers.
- Consult the [Ebitengine 2.9 release notes](https://ebitengine.org/en/documents/2.9.html)
  when touching rendering, input, window, gamepad, text, or audio APIs.
