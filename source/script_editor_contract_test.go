package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode"
)

func TestScriptEditorPackageMatchesRuntimeSurface(t *testing.T) {
	fileSet := token.NewFileSet()
	_, thisFile, _, _ := runtime.Caller(0)
	file, err := parser.ParseFile(fileSet, filepath.Join(filepath.Dir(thisFile), "gt", "pluginapi.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse gt editor package: %v", err)
	}
	editorNames := editorTopLevelNames(file)
	runtimeSymbols := exportsForscript("contract")["gt/gt"]
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

	for _, contract := range []struct {
		name string
		typ  reflect.Type
	}{
		{name: "Player", typ: reflect.TypeOf(Player{})},
		{name: "InventoryItem", typ: reflect.TypeOf(InventoryItem{})},
		{name: "Mobile", typ: reflect.TypeOf(Mobile{})},
		{name: "ClickInfo", typ: reflect.TypeOf(ClickInfo{})},
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
				t.Fatalf("gt.%s is not a struct", name)
			}
			for _, field := range structure.Fields.List {
				var formatted bytes.Buffer
				if err := format.Node(&formatted, fileSet, field.Type); err != nil {
					t.Fatalf("format gt.%s field: %v", name, err)
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
	t.Fatalf("gt.%s not found", name)
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
	return strings.ReplaceAll(value, "main.", "")
}
