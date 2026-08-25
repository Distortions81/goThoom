package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode"

	scriptapi "gt2"
)

func TestScriptEditorPackageMatchesRuntimeSurface(t *testing.T) {
	if scriptAPICurrentVersion != 2 {
		t.Fatalf("script API version = %d, want 2", scriptAPICurrentVersion)
	}
	if _, exists := basescriptExports["gt/gt"]; exists {
		t.Fatal("old gt import path is still exported")
	}
	fileSet := token.NewFileSet()
	_, thisFile, _, _ := runtime.Caller(0)
	contractDir := filepath.Join(filepath.Dir(thisFile), "gt2")
	contractPath := filepath.Join(contractDir, "pluginapi.go")
	contractSource, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read canonical gt2 contract: %v", err)
	}
	file, err := parser.ParseFile(fileSet, contractPath, contractSource, 0)
	if err != nil {
		t.Fatalf("parse gt2 editor package: %v", err)
	}
	reference, err := os.ReadFile(filepath.Join(contractDir, "API_REFERENCE.md"))
	if err != nil {
		t.Fatalf("read generated gt2 API reference: %v", err)
	}
	contractHash := sha256.Sum256(contractSource)
	hashMarker := fmt.Sprintf("Source SHA-256: `%x`", contractHash)
	if !bytes.Contains(reference, []byte(hashMarker)) {
		t.Fatal("gt2 API reference is stale; run go generate in source/gt2")
	}
	editorNames := editorTopLevelNames(file)
	runtimeSymbols := exportsForscript("contract")["gt2/gt2"]
	runtimeNames := map[string]bool{}
	for name := range runtimeSymbols {
		if exportedScriptName(name) {
			runtimeNames[name] = true
		}
	}
	if !reflect.DeepEqual(editorNames, runtimeNames) {
		t.Fatalf("editor/runtime symbols differ\neditor:  %v\nruntime: %v", editorNames, runtimeNames)
	}
	if _, ok := runtimeSymbols["CLVersion"]; !ok {
		t.Fatal("runtime CLVersion export missing")
	}
	if _, ok := runtimeSymbols["clVersion"]; ok {
		t.Fatal("unusable lowercase clVersion is still exported")
	}
	for _, name := range []string{"Player", "Item", "Mobile", "Click", "World"} {
		typ := runtimeSymbols[name].Type()
		if typ.Kind() != reflect.Pointer || typ.Elem().PkgPath() != "gt2" {
			t.Errorf("runtime %s is %v from %q, want public gt2 type", name, typ, typ.Elem().PkgPath())
		}
	}
	removed := []string{
		"AddHotkey", "AddShortcuts", "After", "AfterDur", "Chat", "ChatFrom", "Cmd", "Console", "ConsoleMsg",
		"CreatureChat", "EnqueueCommand", "EquipById", "EquipPartial", "Every", "EveryDur", "Has", "Input",
		"Key", "KeyJustPressed", "MouseJustPressed", "MouseWheel", "Notify", "NPCChat", "OtherChat", "OtherChatFrom",
		"PlayerChat", "PlayerChatFrom", "RegisterChatHandler", "RegisterCommand", "RegisterConsoleTriggers",
		"RegisterInputHandler", "RegisterPlayerHandler", "RegisterTrigger", "RegisterTriggers", "RemoveHotkey", "Run",
		"RunCommand", "SelfChat", "SetInput", "SleepTicks", "UnequipById", "UnequipPartial",
		"IgnoreCase", "StartsWith", "EndsWith", "Includes", "Lower", "Upper", "Trim", "TrimStart", "TrimEnd",
		"Words", "Join", "Replace", "Split",
	}
	for _, name := range removed {
		if runtimeNames[name] || editorNames[name] {
			t.Errorf("removed v1 symbol %s is still public", name)
		}
	}

	for _, contract := range []struct {
		name string
		typ  reflect.Type
	}{
		{name: "Player", typ: reflect.TypeOf(scriptapi.Player{})},
		{name: "Item", typ: reflect.TypeOf(scriptapi.Item{})},
		{name: "Mobile", typ: reflect.TypeOf(scriptapi.Mobile{})},
		{name: "Click", typ: reflect.TypeOf(scriptapi.Click{})},
		{name: "World", typ: reflect.TypeOf(scriptapi.World{})},
	} {
		editorFields := editorStructFields(t, fileSet, file, contract.name)
		runtimeFields := runtimeStructFields(contract.typ)
		if !reflect.DeepEqual(editorFields, runtimeFields) {
			t.Errorf("%s fields differ\neditor:  %v\nruntime: %v", contract.name, editorFields, runtimeFields)
		}
	}
}

func exportedScriptName(name string) bool {
	first, _ := utf8FirstRune(name)
	return unicode.IsUpper(first)
}

func utf8FirstRune(value string) (rune, int) {
	for _, r := range value {
		return r, len(string(r))
	}
	return 0, 0
}

func editorTopLevelNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Recv == nil && exportedScriptName(decl.Name.Name) {
				names[decl.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if exportedScriptName(spec.Name.Name) {
						names[spec.Name.Name] = true
					}
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if exportedScriptName(name.Name) {
							names[name.Name] = true
						}
					}
				}
			}
		}
	}
	return names
}

func editorStructFields(t *testing.T, fileSet *token.FileSet, file *ast.File, name string) map[string]string {
	t.Helper()
	fields := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != name {
				continue
			}
			structure, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				t.Fatalf("gt2.%s is not a struct", name)
			}
			for _, field := range structure.Fields.List {
				var formatted bytes.Buffer
				if err := format.Node(&formatted, fileSet, field.Type); err != nil {
					t.Fatalf("format gt2.%s field: %v", name, err)
				}
				fieldType := normalizeContractType(formatted.String())
				for _, fieldName := range field.Names {
					if exportedScriptName(fieldName.Name) {
						fields[fieldName.Name] = fieldType
					}
				}
			}
			return fields
		}
	}
	t.Fatalf("gt2.%s not found", name)
	return nil
}

func runtimeStructFields(typ reflect.Type) map[string]string {
	fields := map[string]string{}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.IsExported() {
			fields[field.Name] = normalizeContractType(field.Type.String())
		}
	}
	return fields
}

func normalizeContractType(value string) string {
	value = strings.ReplaceAll(value, "byte", "uint8")
	value = strings.ReplaceAll(value, "main.", "")
	return strings.ReplaceAll(value, "gt2.", "")
}
