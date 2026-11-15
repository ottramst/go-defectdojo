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
// a later version breaks no caller. That includes the exported types defined
// as an unexported struct, such as the services (type ProductsService
// service).
func TestExportedStructsAreNonComparable(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	structs := map[string]*ast.StructType{}
	var specs []*ast.TypeSpec
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
				if st, ok := ts.Type.(*ast.StructType); ok {
					structs[ts.Name.Name] = st
				}
				specs = append(specs, ts)
			}
		}
	}
	checked, exceptions := 0, map[string]bool{}
	for _, ts := range specs {
		if !ts.Name.IsExported() {
			continue
		}
		// A type defined as another struct type of the package, such as
		// a service.
		st, ok := ts.Type.(*ast.StructType)
		if ident, isIdent := ts.Type.(*ast.Ident); isIdent {
			st, ok = structs[ident.Name]
		}
		if !ok {
			continue
		}
		if comparableTypes[ts.Name.Name] {
			exceptions[ts.Name.Name] = true
			continue
		}
		checked++
		if !startsWithNoCompare(st) {
			t.Errorf("%s: %s must start with the field _ [0]func(), and have only one", fset.Position(ts.Pos()), ts.Name.Name)
		}
	}
	if checked == 0 {
		t.Fatal("no exported structs found")
	}
	for name := range comparableTypes {
		if !exceptions[name] {
			t.Errorf("comparableTypes lists %s, which isn't an exported struct", name)
		}
	}
}

// startsWithNoCompare reports whether the struct's first field is
// _ [0]func(), and no other field is named _.
func startsWithNoCompare(st *ast.StructType) bool {
	if len(st.Fields.List) == 0 {
		return false
	}
	for i, f := range st.Fields.List {
		for _, n := range f.Names {
			if n.Name == "_" && (i > 0 || len(f.Names) > 1) {
				return false
			}
		}
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
