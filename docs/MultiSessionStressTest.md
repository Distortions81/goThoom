# Multi-session stress testing

Use this checklist for release-candidate testing with real Clan Lord accounts.
Do not store account passwords, movie files, or Text Logs in the repository.

## Automated baseline

From `source/`, run:

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=1 go test -race -run 'Test(SessionManagerReconnectSupervisorsAreIndependent|SessionManagerDisconnectAllWaitsForSupervisorShutdown|SessionManagerDisconnectCancelsReconnectBackoff|MultiSessionSelectionAndShutdownStress|ConcurrentSessionRecordings|TextLogsRemainSeparateForDuplicateCharacterSessions|GlobalScriptConfigChangeReachesEverySessionRuntime|PrimaryLiveScriptRegistrationsUseSessionRuntime|ApplyEnabledScriptsUsesConnectedSecondaryWithoutCompatibilityRuntime)' .
```

The focused race run covers four simultaneous supervisors, unexpected
disconnect backoff, cancellation, rapid selection changes, concurrent movie
writes, duplicate-character Text Logs, shared script-setting propagation,
primary/secondary live-runtime ownership, and joined shutdown. Build Windows
amd64, macOS amd64/arm64, and Linux targets
before beginning the platform matrix; a cross-build verifies compilation but
does not replace launching the client on that operating system.

## Live account matrix

Run the following on Windows, macOS, and Linux. Use four accounts when
available; repeat with two accounts if the server limits the test setup.

1. Open four sessions and place them in different areas, including at least
   one indoor and one outdoor scene. Leave all four active for 30 minutes.
   Then place all four in the same busy outdoor scene with shader lighting and
   character shadows enabled, followed by the same indoor scene. Confirm that
   lighting remains identical per character, brightness does not accumulate,
   and all four views continue updating at a stable frame rate.
2. Rapidly select every view in both tiled and freeform layouts. Confirm input,
   Players, Inventory, Chat, Console, Scripts status, and the REC indicator
   always follow the selected view while background views keep updating.
3. Enable one global script and one character-scoped script. Confirm the
   Scripts window reports the selected runtime, global settings reach every
   running instance, and disabling or reloading one scope produces the
   expected runtime set.
4. Start recording all active sessions and generate distinct movement, chat,
   inventory, and area changes. Stop the recordings in a different order and
   play each file back. Confirm each file contains only its owning character.
   Inspect `Text Logs` and confirm each slot wrote a distinct transcript.
5. Force one account's connection to close without using its logout button.
   Confirm only that view enters reconnect status and the other sessions stay
   responsive. Cancel one reconnect during backoff, then repeat and allow it
   to reconnect.
6. Close the application once while all sessions are connected and once while
   two sessions are reconnecting. Confirm the process exits promptly, saved
   recordings open successfully, and the next launch reports no stale active
   sessions.

Record the operating system/version, graphics backend, account count, run
duration, disconnect method, and any diagnostics path for each result.
