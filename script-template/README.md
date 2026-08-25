# goThoom script template for VS Code

This workspace provides Go syntax highlighting, completion, navigation, and
type checking for goThoom scripts. The local `gt2` package contains editor-only
stubs for the exact scripting API shipped with this goThoom release.

The stubs do not perform game actions. Run scripts through goThoom, not with
`go run`.

## Set up VS Code

1. Extract this ZIP to a normal working folder.
2. Install Go 1.26.6 from [go.dev](https://go.dev/dl/) if it is not installed.
3. Open the extracted `goThoom-Script-Template` folder in VS Code.
4. Accept the recommendation to install the official **Go** extension. If
   prompted, allow it to install `gopls` and the other Go tools.
5. Open `my_script.go`, change its ID and metadata, and start writing.

The included VS Code settings enable the `script` build tag so `gopls` can
analyze script files. Hover over a `gt2` name or type `gt2.` to see completion
and documentation.

## Check your work

From VS Code's terminal, run:

```sh
go test -tags script ./...
```

You can also press Ctrl-Shift-B (Cmd-Shift-B on macOS) and run the included
**Check goThoom script** build task.

This catches normal Go syntax and type errors against the local `gt2` stubs.
goThoom performs additional validation when it loads the script, so also check
the Scripts window for load or runtime errors.

## Install the finished script

1. In goThoom, open **Actions -> Scripts -> Open scripts folder**.
2. Copy `my_script.go` into that folder.
3. Return to the Scripts window and enable it globally or for a character.

For a script with images or other assets, create a clean folder or ZIP with
exactly one `.go` file at its root and the assets beneath it. Do not distribute
this workspace's `.vscode`, `gt2`, `go.mod`, or `go.work` support files as part
of the installed script package.

See `SCRIPTING_GUIDE.md` for scripting concepts and `gt2/API_REFERENCE.md` for
the complete API available in this release.
