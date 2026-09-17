package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoScriptsRequireUTF8(t *testing.T) {
	const unicodeSource = "package main\nconst Greeting = \"Café 你好 🌟\"\nfunc Init() {}\n"
	for _, tc := range []struct {
		name, source string
		valid        bool
	}{
		{"unicode", unicodeSource, true},
		{"unicode-bom", "\ufeff" + unicodeSource, true},
		{"macroman-string", "package main\nconst Greeting = \"caf\x8e\"\nfunc Init() {}\n", false},
		{"macroman-comment", "package main\n// caf\x8e\nfunc Init() {}\n", false},
		{"malformed-utf8", "package main\n// \xc3\nfunc Init() {}\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner := "encoding-" + tc.name
			grantScriptPermissionsForTest(t, owner)
			prepared, err := compileScriptSource(owner, []byte(tc.source), restrictedStdlib())
			defer disposePreparedScript(prepared)
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
				value, err := prepared.interpreter.Eval("Greeting")
				if err != nil || value.String() != "Café 你好 🌟" {
					t.Fatalf("Unicode value changed: %v, %v", value, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "must use UTF-8") {
				t.Fatalf("invalid encoding compile error = %v", err)
			}

			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "folder"), 0755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"loose.go", filepath.Join("folder", "main.go")} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(tc.source), 0644); err != nil {
					t.Fatal(err)
				}
			}
			writeScriptZip(t, filepath.Join(dir, "archive.zip"), map[string]string{"main.go": tc.source})
			scanned := scanscripts([]string{dir}, nil)
			for _, name := range []string{"loose", "folder", "archive"} {
				info, ok := scanned[name]
				if !ok || info.invalid == tc.valid {
					t.Fatalf("%s encoding status = %+v", name, info)
				}
				if !tc.valid && !strings.Contains(info.err, "must use UTF-8") {
					t.Fatalf("%s encoding error = %q", name, info.err)
				}
			}
		})
	}
}
