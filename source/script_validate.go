package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func checkScriptSourceRequirements(source []byte) error {
	file, err := parser.ParseFile(token.NewFileSet(), "_.go", source, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("syntax error: %w", err)
	}
	allowed := map[string]bool{"gt2": true}
	for _, key := range scriptAllowedPkgs {
		if separator := strings.LastIndexByte(key, '/'); separator >= 0 {
			allowed[key[:separator]] = true
		}
	}
	allowedNames := make([]string, 0, len(allowed))
	for name := range allowed {
		allowedNames = append(allowedNames, name)
	}
	sort.Strings(allowedNames)
	for _, imported := range file.Imports {
		name, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return fmt.Errorf("invalid import %s", imported.Path.Value)
		}
		if name == "gt" {
			return fmt.Errorf("unsupported import %q; use %q for scripting API 2", name, "gt2")
		}
		if !allowed[name] {
			return fmt.Errorf("unsupported import %q; allowed imports: %s", name, strings.Join(allowedNames, ", "))
		}
	}

	version := scriptAPICurrentVersion
	for _, declaration := range file.Decls {
		generated, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, specification := range generated.Specs {
			values, ok := specification.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range values.Names {
				if name.Name != "scriptAPIVersion" {
					continue
				}
				if index >= len(values.Values) {
					return fmt.Errorf("scriptAPIVersion must be the integer %d", scriptAPICurrentVersion)
				}
				literal, ok := values.Values[index].(*ast.BasicLit)
				if !ok || literal.Kind != token.INT {
					return fmt.Errorf("scriptAPIVersion must be the integer %d", scriptAPICurrentVersion)
				}
				parsed, err := strconv.Atoi(literal.Value)
				if err != nil {
					return fmt.Errorf("invalid scriptAPIVersion %q", literal.Value)
				}
				version = parsed
			}
		}
	}
	if version != scriptAPICurrentVersion {
		return fmt.Errorf("unsupported script API version %d; this client supports version %d", version, scriptAPICurrentVersion)
	}
	return nil
}

func validateScriptFile(owner, path string) error {
	scriptMu.RLock()
	info, packaged := scriptPackages[owner]
	scriptMu.RUnlock()
	var source []byte
	if packaged && info.path == path {
		source = append([]byte(nil), info.src...)
	} else {
		root, openErr := os.OpenRoot(filepath.Dir(path))
		if openErr != nil {
			return fmt.Errorf("open script folder: %w", openErr)
		}
		defer root.Close()
		var readErr error
		source, readErr = root.ReadFile(filepath.Base(path))
		if readErr != nil {
			return fmt.Errorf("read script: %w", readErr)
		}
	}
	prepared, err := prepareScriptSource(owner, source, restrictedStdlib())
	if err == nil {
		err = scriptCandidateConflict(owner, prepared.candidate)
	}
	disposePreparedScript(prepared)
	if err != nil {
		return fmt.Errorf("%s", formatScriptError(path, err))
	}
	return nil
}
