# Multi-session stress testing

Use this checklist for release-candidate testing with real Clan Lord accounts.
Do not store account passwords, movie files, or Text Logs in the repository.

## Automated baseline

From `source/`, run:

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=1 go test -race -run 'Test(SessionManagerReconnectSupervisorsAreIndependent|SessionManagerDisconnectAllWaitsForSupervisorShutdown|SessionManagerDisconnectCancelsReconnectBackoff|MultiSessionSelectionAndShutdownStress|ConcurrentSessionRecordings|TextLogsRemainSeparateForDuplicateCharacterSessions|GlobalScriptConfigChangeReachesEverySessionRuntime|PrimaryLiveScriptRegistrationsUseSessionRuntime|ApplyEnabledScriptsUsesConnectedSecondaryWithoutCompatibilityRuntime)' .
```

The focused race run covers simultaneous supervisors, unexpected-disconnect
backoff, cancellation, rapid selection, concurrent movie writes,
duplicate-character Text Logs, script-setting propagation, runtime ownership,
and joined shutdown. Cross-build Windows amd64, macOS amd64/arm64, and Linux
before the platform matrix; compilation does not replace launching each build.

## Live account matrix

Run the following on Windows, macOS, and Linux. Use as many test accounts as
available, then fill the remaining slots with disconnected tabs to exercise the
ten-tab layout.

1. Add tabs until ten are open. Confirm **+** disables, Ctrl-1 through Ctrl-9
   and Ctrl-0 select the expected visible tab, customized modifiers work, and
   closing a middle tab makes the shortcuts follow the remaining visible order.
2. Connect at least two sessions in different areas. Rapidly switch tabs and
   confirm movement, keyboard input, Inventory, Players, Scripts, Chat, Console,
   title text, login state, logout, and the REC indicator always follow the
   selected character.
3. Compare the active tab's frame rate with a separate client showing the same
   scene and settings. Repeat in a busy outdoor scene with shader lighting and
   sun shadows, then indoors. Background session count should not multiply
   scene or lighting render cost.
4. Start a long bard performance in each connected session. Leave one session
   inactive, switch to it partway through, and confirm its music begins near the
   current point rather than at the beginning. Confirm only the active tab's
   sound effects play.
5. Enable one global script and one character-scoped script. Confirm each
   background runtime continues, the Scripts window reports the selected
   runtime, and global setting changes reach every instance.
6. Record all connected sessions and generate distinct movement, chat,
   inventory, and area changes. Stop in a different order, play each file, and
   confirm each recording and Text Log contains only its owning character.
7. Force one connection closed. Confirm only that tab enters reconnect status,
   the others stay responsive, cancellation during backoff works, and allowing
   a later retry reconnects the correct tab.
8. Close a connected tab and confirm the warning appears and the connection is
   torn down. Confirm the final remaining tab cannot close. Restart and verify
   the open-tab set and selected tab restore without reconnecting automatically.
9. Quit once with every session connected and once with two sessions
   reconnecting. Confirm prompt shutdown and valid saved recordings.

Record the operating system/version, graphics backend, account count, open-tab
count, run duration, disconnect method, active-scene settings, comparative FPS,
and diagnostics path for each result.
