package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testSpec = `openapi: 3.0.3
paths:
  /api/v2/things:
    parameters:
      - name: x
        in: query
    get:
      tags: [things]
    post:
      tags: [things]
      deprecated: true
  /api/v2/things/{id}:
    delete:
      tags: [things]
      x-state: Technical Preview; added in 9.1.0
    put:
      tags: [things]
      x-state: "a | b"
  /api/v2/things/_query:
    post: {}
  /api/v2/other:
    get:
      tags: [Things]
`

func writeRepo(t *testing.T, goSrc string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cache", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cache", "specs", "defectdojo-1.0.0.json"), []byte(testSpec), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "things.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRun(t *testing.T) {
	root := writeRepo(t, `package defectdojo

// List lists things.
//
// DefectDojo API: GET /api/v2/things
// DefectDojo API docs: https://example.com
func (s *ThingsService) List() {}

// Query queries things; a value receiver.
//
// DefectDojo API: POST /api/v2/things/_query
func (s ThingsService) Query() {}

// Generic receiver.
//
// DefectDojo API: GET /api/v2/other
func (s *Others[T]) Get() {}
`)
	if err := run(root, ".cache/specs", []string{"1.0.0"}, "COVERAGE.md"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "COVERAGE.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"**3 of 6 endpoints implemented.**",
		// Tags that differ only in case get distinct anchors, like on GitHub.
		"| [Things](#things) | 1 / 1 |",
		"| [things](#things-1) | 2 / 5 |",
		// The untagged operation gets the tag of its closest path.
		"| POST | `/api/v2/things/_query` | ✅ | `ThingsService.Query` |  |",
		"| GET | `/api/v2/other` | ✅ | `Others.Get` |  |",
		"| PUT | `/api/v2/things/{id}` | ✅ |  | a \\| b |",
		"| GET | `/api/v2/things` | ✅ | `ThingsService.List` |  |",
		"| POST | `/api/v2/things` | ✅ |  | deprecated |",
		"| DELETE | `/api/v2/things/{id}` | ✅ |  | Technical Preview; added in 9.1.0 |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("COVERAGE.md does not contain %q:\n%s", want, got)
		}
	}
}

func TestRun_errors(t *testing.T) {
	tests := map[string]string{
		"unknown endpoint": "package defectdojo\n\n// DefectDojo API: GET /api/nope\nfunc (s *ThingsService) Nope() {}\n",
		"not on a method":  "package defectdojo\n\n// DefectDojo API: GET /api/v2/things\nvar x = 1\n",
		"duplicate": "package defectdojo\n\n// DefectDojo API: GET /api/v2/things\nfunc (s *ThingsService) A() {}\n\n" +
			"// DefectDojo API: GET /api/v2/things\nfunc (s *ThingsService) B() {}\n",
		"lower case method":  "package defectdojo\n\n// DefectDojo API: get /api/v2/things\nfunc (s *ThingsService) A() {}\n",
		"mixed case method":  "package defectdojo\n\n// DefectDojo API: Get /api/v2/things\nfunc (s *ThingsService) A() {}\n",
		"trailing space":     "package defectdojo\n\n// DefectDojo API: GET /api/v2/things \nfunc (s *ThingsService) A() {}\n",
		"unsupported method": "package defectdojo\n\n// DefectDojo API: HEAD /api/v2/things\nfunc (s *ThingsService) A() {}\n",
		"indented":           "package defectdojo\n\nfunc f() {\n\t// DefectDojo API: GET /api/v2/things\n}\n",
		"end of file":        "package defectdojo\n\n// DefectDojo API: GET /api/v2/things\n",
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			root := writeRepo(t, src)
			if err := run(root, ".cache/specs", []string{"1.0.0"}, "COVERAGE.md"); err == nil {
				t.Error("run() error = nil, want error")
			}
		})
	}
}

func TestRun_missingSpec(t *testing.T) {
	root := writeRepo(t, "package defectdojo\n")
	if err := run(root, ".cache/specs", []string{"2.0.0"}, "COVERAGE.md"); err == nil {
		t.Error("run() error = nil, want error")
	}
}
