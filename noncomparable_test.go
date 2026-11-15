package defectdojo_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// comparableTypes are the exported structs meant to be compared and copied as
// values. They are frozen: a field added to them would break unkeyed literals
// or comparisons.
var comparableTypes = map[string]bool{"Date": true, "Nullable": true, "NotificationScope": true}

// Every other exported struct starts with an unexported _ [0]func(), so that
// it can't be compared or written as an unkeyed literal, and a field added in
// a later version breaks no caller.
func TestExportedStructsAreNonComparable(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok || !ts.Name.IsExported() || comparableTypes[ts.Name.Name] {
					continue
				}
				checked++
				if !startsWithNoCompare(st) {
					t.Errorf("%s: %s must start with the field _ [0]func()", fset.Position(ts.Pos()), ts.Name.Name)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no exported structs found")
	}
}

// startsWithNoCompare reports whether the struct's first field is
// _ [0]func().
func startsWithNoCompare(st *ast.StructType) bool {
	if len(st.Fields.List) == 0 {
		return false
	}
	f := st.Fields.List[0]
	if len(f.Names) != 1 || f.Names[0].Name != "_" {
		return false
	}
	arr, ok := f.Type.(*ast.ArrayType)
	if !ok {
		return false
	}
	length, ok := arr.Len.(*ast.BasicLit)
	_, isFunc := arr.Elt.(*ast.FuncType)
	return ok && length.Value == "0" && isFunc
}
