package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func scriptEditorUnavailable(owner, path string) string {
	if isWASM {
		return "Script source editing is available in the desktop client."
	}
	scriptMu.RLock()
	info := scriptPackages[owner]
	scriptMu.RUnlock()
	if info.assets != nil && info.assets.zipped || strings.EqualFold(filepath.Ext(path), ".zip") {
		return "Unpack this ZIP script into a folder before editing its Go source."
	}
	if !strings.EqualFold(filepath.Ext(path), ".go") {
		return "No editable Go source file is available for this script."
	}
	return ""
}

func openScriptSourceEditor(owner, path string) *sourceEditor {
	if reason := scriptEditorUnavailable(owner, path); reason != "" {
		consoleMessage("[script] " + reason)
		return nil
	}
	doc, err := loadSourceDocument(path, false)
	if err != nil {
		consoleMessage("[script] open editor: " + err.Error())
		return nil
	}
	return openSourceEditor(doc, sourceEditorOptions{
		kind:           "Script",
		highlight:      highlightGoScript,
		format:         formatGoScript,
		formatPosition: remapGoFormattedPosition,
		reloadTooltip:  "Save the shared script file and reload it in the selected session if it is running.",
		check:          func(value string) error { return checkScriptEditorDraft(owner, doc.path, value) },
		reload: func() (string, error) {
			// A changed ID or duplicate must not reload a different file under
			// the editor's original owner. The user can rediscover it with Refresh.
			info, ok := scanscripts(scriptSearchDirs(), nil)[owner]
			absolute, _ := filepath.Abs(info.path)
			if !ok || absolute != doc.path {
				return "", fmt.Errorf("this script's ID or location changed; use Refresh in Scripts before reloading")
			}
			session := scriptManagerSession()
			running, _ := session.scriptRuntimeSnapshot(owner)
			if err := reloadScriptForSessionResult(session, owner); err != nil {
				return "", err
			}
			if !running {
				return "Saved; source refreshed. The script remains stopped; its enable settings are unchanged.", nil
			}
			return "Saved; script reloaded in the selected session.", nil
		},
		afterSave: func() { refreshscriptsWindow(); refreshscriptDetails() },
	})
}

func checkScriptEditorDraft(owner, path, value string) error {
	scriptMu.RLock()
	assets := scriptPackages[owner].assets
	scriptMu.RUnlock()
	prepared, err := prepareScriptSourceWithAssetsForSession(scriptManagerSession(), owner, []byte(value), restrictedStdlib(), assets)
	defer disposePreparedScript(prepared)
	if err != nil {
		return fmt.Errorf("%s", formatScriptError(path, err))
	}
	return nil
}
