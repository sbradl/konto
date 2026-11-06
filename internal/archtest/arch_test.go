// Package archtest holds konto's architecture fitness tests: the package
// layering of ADR 0003/0004/0012, the Charm v2 stack rule, the ANSI-16 palette
// of ADR 0005 and the injected clock of ADR 0012. It has no production code.
package archtest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type layer int

const (
	layerDomain layer = iota + 1
	layerSupport
	layerAdapter
	layerConfig
	layerUseCase
	layerUILeaf
	layerView
	layerUIRoot
	layerCmd
	layerE2E
)

var layerNames = map[layer]string{
	layerDomain:  "domain",
	layerSupport: "support",
	layerAdapter: "adapter",
	layerConfig:  "config",
	layerUseCase: "use case",
	layerUILeaf:  "ui leaf",
	layerView:    "view",
	layerUIRoot:  "ui root",
	layerCmd:     "composition root",
	layerE2E:     "e2e harness",
}

// layers classifies every package konto may have (spec §3, ADR 0012). A
// package missing here fails the suite: placing a new package in a layer is a
// deliberate design decision, not something the build infers. Entry is absent
// on purpose (ADR 0010).
var layers = map[string]layer{
	"cmd/konto": layerCmd,

	"internal/domain": layerDomain,

	"internal/parallel":  layerSupport, // stdlib-only plumbing the use cases share; no domain knowledge
	"internal/rulesfile": layerSupport, // the categories.rules line syntax the import and the journal adapter share

	"internal/hledger": layerAdapter,
	"internal/journal": layerAdapter,
	"internal/files":   layerAdapter,

	"internal/config": layerConfig,

	"internal/ledger":    layerUseCase,
	"internal/accounts":  layerUseCase,
	"internal/checks":    layerUseCase,
	"internal/carry":     layerUseCase,
	"internal/report":    layerUseCase,
	"internal/csvimport": layerUseCase,
	"internal/periodic":  layerUseCase,
	"internal/forecast":  layerUseCase,

	"internal/ui": layerUIRoot,

	"internal/ui/theme":   layerUILeaf,
	"internal/ui/layout":  layerUILeaf,
	"internal/ui/keys":    layerUILeaf,
	"internal/ui/widgets": layerUILeaf,
	"internal/ui/uitest":  layerUILeaf,

	"internal/ui/dashboard": layerView,
	"internal/ui/imports":   layerView,
	"internal/ui/scheduled": layerView,
	"internal/ui/forecast":  layerView,
	"internal/ui/reports":   layerView,

	"e2e":             layerE2E,
	"e2e/hledgershim": layerE2E,
	"e2e/shimconf":    layerE2E,
}

// mayImport lists the konto layers each layer may depend on. Dependencies
// point inward (ADR 0003); views never import each other or the root
// (ADR 0004); adapters and use cases meet only through consumer-side ports.
var mayImport = map[layer][]layer{
	layerDomain:  nil,
	layerSupport: nil,
	layerAdapter: {layerDomain, layerSupport},
	layerConfig:  {layerDomain},
	layerUseCase: {layerDomain, layerSupport},
	layerUILeaf:  {layerDomain, layerUILeaf},
	layerView:    {layerDomain, layerUseCase, layerUILeaf},
	layerUIRoot:  {layerDomain, layerUseCase, layerUILeaf, layerView},
	layerCmd:     {layerDomain, layerSupport, layerAdapter, layerConfig, layerUseCase, layerUILeaf, layerView, layerUIRoot},
	layerE2E:     {layerE2E}, // black box: drives the compiled binary only, sharing only the shim's config
}

// charmV2 is the only third-party code konto itself uses (spec §1), confined
// to the UI circle and the composition root. The frozen v1 import paths
// (github.com/charmbracelet/bubbletea, …) are rejected like any other module.
var charmV2 = []string{
	"charm.land/bubbletea/v2",
	"charm.land/lipgloss/v2",
	"charm.land/bubbles/v2",
	"github.com/NimbleMarkets/ntcharts/v2",
}

// e2eDeps is the harness's toolkit (features/README.md): Gherkin runner, pty,
// virtual terminal.
var e2eDeps = []string{
	"github.com/cucumber/godog",
	"github.com/creack/pty",
	"github.com/charmbracelet/x/vt",
	"github.com/charmbracelet/x/ansi",
}

// thirdParty lists the modules each layer may import; unlisted layers none.
var thirdParty = map[layer][]string{
	layerUILeaf: charmV2,
	layerView:   charmV2,
	layerUIRoot: charmV2,
	layerCmd:    charmV2,
	layerE2E:    e2eDeps,
}

// forbiddenStdlib bans I/O from the pure domain (non-test files only).
var forbiddenStdlib = map[layer][]string{
	layerDomain: {"os", "os/exec", "io/fs", "io/ioutil", "net", "syscall"},
}

// execAllowed lists the only packages that may start processes: the hledger
// adapter (every hledger call, the preflight's included), main (builds the
// $EDITOR command), and the ui root (runs it via tea.ExecProcess). The e2e
// harness starts konto, the shim and hledger.
var execAllowed = []string{"internal/hledger", "cmd/konto", "internal/ui", "e2e", "e2e/hledgershim"}

// clockFree layers must take "today" as an injected func (ADR 0012).
var clockFree = []layer{layerDomain, layerUseCase}

const themeDir = "internal/ui/theme"

var hexColour = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

type goFile struct {
	name string
	test bool
	ast  *ast.File
}

type pkg struct {
	dir   string // module-relative, slash-separated
	files []goFile
}

type repo struct {
	module string
	pkgs   []pkg
}

// file is one Go file placed in its package and layer.
type file struct {
	goFile
	dir   string
	layer layer
	where string // module-relative path, for messages
}

func (r repo) check() []string {
	var out []string
	for _, p := range r.pkgs {
		l, known := layers[p.dir]
		if !known {
			out = append(out, p.dir+": unclassified package; add it to archtest layers deliberately (spec §3)")

			continue
		}
		for _, gf := range p.files {
			f := file{goFile: gf, dir: p.dir, layer: l, where: path.Join(p.dir, gf.name)}
			out = append(out, r.checkImports(f)...)
			out = append(out, checkClock(f)...)
			out = append(out, checkColours(f)...)
		}
	}

	return out
}

func (r repo) checkImports(f file) []string {
	var out []string
	for _, spec := range f.ast.Imports {
		imp, _ := strconv.Unquote(spec.Path.Value)
		var problem string
		switch {
		case imp == r.module || strings.HasPrefix(imp, r.module+"/"):
			problem = kontoImportProblem(f, strings.TrimPrefix(strings.TrimPrefix(imp, r.module), "/"))
		case isStdlib(imp):
			problem = stdlibImportProblem(f, imp)
		default:
			problem = thirdPartyImportProblem(f, imp)
		}

		if problem != "" {
			out = append(out, f.where+": "+problem)
		}
	}

	return out
}

func kontoImportProblem(f file, dep string) string {
	if dep == f.dir {
		return "" // external test package importing its own package
	}

	if depLayer, known := layers[dep]; known && slices.Contains(mayImport[f.layer], depLayer) {
		return ""
	}

	return fmt.Sprintf("%s must not import %s (ADR 0003)", layerNames[f.layer], dep)
}

func stdlibImportProblem(f file, imp string) string {
	switch {
	case imp == "os/exec" && !slices.Contains(execAllowed, f.dir):
		return layerNames[f.layer] + " must not import os/exec; only the hledger adapter, main and the ui root start processes"
	case !f.test && slices.Contains(forbiddenStdlib[f.layer], imp):
		return fmt.Sprintf("%s must not import %s; it does no I/O", layerNames[f.layer], imp)
	}

	return ""
}

func thirdPartyImportProblem(f file, imp string) string {
	switch {
	case hasModulePrefix(imp, thirdParty[f.layer]):
		return ""
	case hasModulePrefix(imp, charmV2):
		return fmt.Sprintf("%s must not import %s; Charm is confined to internal/ui and cmd", layerNames[f.layer], imp)
	}

	return fmt.Sprintf("third-party import %s not allowed in %s (spec §1)", imp, layerNames[f.layer])
}

func checkClock(f file) []string {
	if f.test || !slices.Contains(clockFree, f.layer) {
		return nil
	}

	name := localName(f.ast, "time")
	if name == "" {
		return nil
	}
	var out []string
	ast.Inspect(f.ast, func(n ast.Node) bool {
		if isSelector(n, name, "Now") {
			out = append(out, fmt.Sprintf("%s: %s reads time.Now; inject today func() domain.Date (ADR 0012)",
				f.where, layerNames[f.layer]))
		}

		return true
	})

	return out
}

func checkColours(f file) []string {
	if f.test {
		return nil
	}
	lipgloss := localName(f.ast, "charm.land/lipgloss/v2")
	var out []string
	ast.Inspect(f.ast, func(n ast.Node) bool {
		var problem string
		switch n := n.(type) {
		case *ast.BasicLit:
			problem = hexLiteralProblem(n)
		case *ast.CallExpr:
			problem = colourCallProblem(f.dir, lipgloss, n)
		}

		if problem != "" {
			out = append(out, f.where+": "+problem)
		}

		return true
	})

	return out
}

func hexLiteralProblem(lit *ast.BasicLit) string {
	if lit.Kind != token.STRING {
		return ""
	}

	if s, err := strconv.Unquote(lit.Value); err == nil && hexColour.MatchString(s) {
		return fmt.Sprintf("hex colour %q; use ANSI-16 indices (ADR 0005)", s)
	}

	return ""
}

func colourCallProblem(dir, lipgloss string, call *ast.CallExpr) string {
	switch {
	case lipgloss == "" || !isSelector(call.Fun, lipgloss, "Color"):
		return ""
	case dir != themeDir:
		return "lipgloss.Color outside " + themeDir + "; use the named theme styles (ADR 0005)"
	case !isANSIIndex(call.Args):
		return "lipgloss.Color takes a literal ANSI index \"0\"–\"15\" (ADR 0005)"
	}

	return ""
}

func isANSIIndex(args []ast.Expr) bool {
	if len(args) != 1 {
		return false
	}

	lit, ok := args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	s, _ := strconv.Unquote(lit.Value)
	i, err := strconv.Atoi(s)

	return err == nil && i >= 0 && i <= 15 && strconv.Itoa(i) == s
}

func isSelector(n ast.Node, pkgName, sel string) bool {
	s, ok := n.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != sel {
		return false
	}
	id, ok := s.X.(*ast.Ident)

	return ok && id.Name == pkgName
}

// localName returns the name an import is bound to in f, or "" if f does not
// import it (blank and dot imports count as not importing it by name).
func localName(f *ast.File, importPath string) string {
	for _, spec := range f.Imports {
		if imp, _ := strconv.Unquote(spec.Path.Value); imp != importPath {
			continue
		}

		if spec.Name != nil {
			return spec.Name.Name
		}

		return path.Base(strings.TrimSuffix(importPath, "/v2"))
	}

	return ""
}

func isStdlib(imp string) bool {
	first, _, _ := strings.Cut(imp, "/")

	return !strings.Contains(first, ".")
}

func hasModulePrefix(imp string, modules []string) bool {
	for _, m := range modules {
		if imp == m || strings.HasPrefix(imp, m+"/") {
			return true
		}
	}

	return false
}

// loadRepo parses every Go file under the module root except archtest's own.
func loadRepo(t *testing.T) repo {
	t.Helper()
	root := moduleRoot(t)

	pkgs, err := parsePackages(root)
	if err != nil {
		t.Fatal(err)
	}
	r := repo{module: modulePath(t, root)}
	for _, p := range pkgs {
		if p.dir != "internal/archtest" {
			r.pkgs = append(r.pkgs, p)
		}
	}

	return r
}

func modulePath(t *testing.T, root string) string {
	t.Helper()

	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(mod), "\n") {
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest)
		}
	}
	t.Fatal("no module line in go.mod")

	return ""
}

// parsePackages parses every Go file under root, grouped by directory in walk
// order, skipping the directories the go tool skips.
func parsePackages(root string) ([]pkg, error) {
	fset := token.NewFileSet()
	var pkgs []pkg
	index := map[string]int{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && p != root && ignoredDir(d.Name()):
			return filepath.SkipDir
		case d.IsDir() || !strings.HasSuffix(p, ".go"):
			return nil
		}

		parsed, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, filepath.Dir(p))

		dir := filepath.ToSlash(rel)
		if _, seen := index[dir]; !seen {
			index[dir] = len(pkgs)
			pkgs = append(pkgs, pkg{dir: dir})
		}
		gf := goFile{name: d.Name(), test: strings.HasSuffix(p, "_test.go"), ast: parsed}
		pkgs[index[dir]].files = append(pkgs[index[dir]].files, gf)

		return nil
	})

	return pkgs, err
}

func ignoredDir(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor"
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

// The real repository follows the design.
func TestRepositoryFollowsArchitecture(t *testing.T) {
	r := loadRepo(t)
	for _, v := range r.check() {
		t.Error(v)
	}
}

// The loader actually sees the repository, so the test above cannot pass
// vacuously.
func TestLoaderFindsKnownPackages(t *testing.T) {
	r := loadRepo(t)
	if r.module != "github.com/sbradl/konto" {
		t.Errorf("module = %q", r.module)
	}
	dirs := make([]string, 0, len(r.pkgs))
	for _, p := range r.pkgs {
		dirs = append(dirs, p.dir)
	}
	for _, want := range []string{"internal/domain", "internal/hledger"} {
		if !slices.Contains(dirs, want) {
			t.Errorf("loader missed %s; saw %v", want, dirs)
		}
	}

	if slices.Contains(dirs, "internal/archtest") {
		t.Error("loader must skip archtest itself")
	}
}

const testModule = "github.com/sbradl/konto"

func fakeRepo(t *testing.T, dir, file, src string) repo {
	t.Helper()

	f, err := parser.ParseFile(token.NewFileSet(), file, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}

	return repo{module: testModule, pkgs: []pkg{{dir: dir, files: []goFile{{name: file, test: strings.HasSuffix(file, "_test.go"), ast: f}}}}}
}

func imports(pkgName string, paths ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\nimport (\n", pkgName)
	for _, p := range paths {
		fmt.Fprintf(&b, "\t_ %q\n", p)
	}
	b.WriteString(")\n")

	return b.String()
}

// Each rule catches its violation and admits what the design allows.
func TestRules(t *testing.T) {
	k := func(p string) string { return testModule + "/" + p }
	cases := []struct {
		name, dir, file, src string
		want                 string // substring of the single expected violation; "" = none
	}{
		// layering (ADR 0003, 0004)
		{"domain imports nothing of konto's", "internal/domain", "a.go", imports("domain", k("internal/ledger")), "domain must not import internal/ledger"},
		{"use case must not import adapter", "internal/report", "a.go", imports("report", k("internal/hledger")), "use case must not import internal/hledger"},
		{"use case must not import use case", "internal/forecast", "a.go", imports("forecast", k("internal/periodic")), "use case must not import internal/periodic"},
		{"use case imports domain", "internal/csvimport", "a.go", imports("csvimport", k("internal/domain")), ""},
		{"use case imports support", "internal/checks", "a.go", imports("checks", k("internal/parallel")), ""},
		{"support imports nothing of konto's", "internal/parallel", "a.go", imports("parallel", k("internal/domain")), "support must not import internal/domain"},
		{"domain does not import support", "internal/domain", "a.go", imports("domain", k("internal/parallel")), "domain must not import internal/parallel"},
		{"adapter must not import use case", "internal/hledger", "a.go", imports("hledger", k("internal/report")), "adapter must not import internal/report"},
		{"adapter must not import adapter", "internal/journal", "a.go", imports("journal", k("internal/hledger")), "adapter must not import internal/hledger"},
		{"config must not import adapter", "internal/config", "a.go", imports("config", k("internal/journal")), "config must not import internal/journal"},
		{"view must not import view", "internal/ui/dashboard", "a.go", imports("dashboard", k("internal/ui/reports")), "view must not import internal/ui/reports"},
		{"view must not import adapter", "internal/ui/imports", "a.go", imports("imports", k("internal/journal")), "view must not import internal/journal"},
		{"view must not import ui root", "internal/ui/scheduled", "a.go", imports("scheduled", k("internal/ui")), "view must not import internal/ui"},
		{"view imports domain, use case, leaves, charm", "internal/ui/forecast", "a.go", imports("forecast", k("internal/domain"), k("internal/forecast"), k("internal/ui/widgets"), "charm.land/bubbletea/v2", "github.com/NimbleMarkets/ntcharts/v2/linechart"), ""},
		{"leaf must not import view", "internal/ui/widgets", "a.go", imports("widgets", k("internal/ui/dashboard")), "ui leaf must not import internal/ui/dashboard"},
		{"leaf imports leaf", "internal/ui/widgets", "a.go", imports("widgets", k("internal/ui/theme")), ""},
		{"ui root imports views and use cases", "internal/ui", "a.go", imports("ui", k("internal/ui/dashboard"), k("internal/checks")), ""},
		{"ui root must not import adapter", "internal/ui", "a.go", imports("ui", k("internal/hledger")), "ui root must not import internal/hledger"},
		{"main wires everything", "cmd/konto", "main.go", imports("main", k("internal/hledger"), k("internal/journal"), k("internal/config"), k("internal/ledger"), k("internal/ui")), ""},
		{"importing an unclassified package", "internal/report", "a.go", imports("report", k("internal/util")), "use case must not import internal/util"},
		{"external test package imports its own package", "internal/report", "a_test.go", imports("report_test", k("internal/report"), k("internal/domain")), ""},
		{"test files obey layering too", "internal/report", "a_test.go", imports("report", k("internal/hledger")), "use case must not import internal/hledger"},
		{"unclassified package", "internal/entry", "a.go", imports("entry"), "internal/entry: unclassified package"},

		// dependencies (spec §1)
		{"use case must not import charm", "internal/periodic", "a.go", imports("periodic", "charm.land/lipgloss/v2"), "use case must not import charm.land/lipgloss/v2"},
		{"charm v1 paths rejected", "internal/ui", "a.go", imports("ui", "github.com/charmbracelet/bubbletea"), "third-party import github.com/charmbracelet/bubbletea not allowed"},
		{"no other third-party modules", "internal/domain", "a.go", imports("domain", "github.com/shopspring/decimal"), "third-party import github.com/shopspring/decimal not allowed"},
		{"e2e harness toolkit", "e2e", "a_test.go", imports("e2e", "github.com/cucumber/godog", "github.com/creack/pty", "github.com/charmbracelet/x/vt", "os/exec"), ""},
		{"e2e is a black box", "e2e", "a_test.go", imports("e2e", k("internal/domain")), "e2e harness must not import internal/domain"},
		{"e2e toolkit stays in e2e", "internal/ui", "a.go", imports("ui", "github.com/charmbracelet/x/vt"), "third-party import github.com/charmbracelet/x/vt not allowed in ui root"},
		{"e2e must not use charm", "e2e", "a_test.go", imports("e2e", "charm.land/bubbletea/v2"), "e2e harness must not import charm.land/bubbletea/v2"},

		// I/O and processes
		{"domain does no I/O", "internal/domain", "a.go", imports("domain", "os"), "domain must not import os;"},
		{"domain tests may use os", "internal/domain", "a_test.go", imports("domain", "os"), ""},
		{"use case must not exec", "internal/ledger", "a.go", imports("ledger", "os/exec"), "use case must not import os/exec"},
		{"view must not exec", "internal/ui/imports", "a.go", imports("imports", "os/exec"), "view must not import os/exec"},
		{"hledger adapter execs", "internal/hledger", "a.go", imports("hledger", "os/exec"), ""},
		{"ui root execs $EDITOR", "internal/ui", "a.go", imports("ui", "os/exec"), ""},

		// clock (ADR 0012)
		{"use case must not read the clock", "internal/forecast", "a.go", "package forecast\nimport \"time\"\nvar x = time.Now()\n", "use case reads time.Now"},
		{"aliased time import still caught", "internal/accounts", "a.go", "package accounts\nimport t \"time\"\nvar x = t.Now()\n", "use case reads time.Now"},
		{"domain must not read the clock", "internal/domain", "a.go", "package domain\nimport \"time\"\nvar x = time.Now()\n", "domain reads time.Now"},
		{"other time functions fine", "internal/forecast", "a.go", "package forecast\nimport \"time\"\nvar x = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)\n", ""},
		{"use case tests may read the clock", "internal/forecast", "a_test.go", "package forecast\nimport \"time\"\nvar x = time.Now()\n", ""},
		{"main reads the clock", "cmd/konto", "main.go", "package main\nimport \"time\"\nvar x = time.Now()\n", ""},

		// colours (ADR 0005)
		{"theme builds ANSI colours", "internal/ui/theme", "a.go", "package theme\nimport \"charm.land/lipgloss/v2\"\nvar a, b = lipgloss.Color(\"0\"), lipgloss.Color(\"15\")\n", ""},
		{"theme index above 15", "internal/ui/theme", "a.go", "package theme\nimport \"charm.land/lipgloss/v2\"\nvar a = lipgloss.Color(\"16\")\n", "literal ANSI index"},
		{"theme index below 0", "internal/ui/theme", "a.go", "package theme\nimport \"charm.land/lipgloss/v2\"\nvar a = lipgloss.Color(\"-1\")\n", "literal ANSI index"},
		{"theme index padded", "internal/ui/theme", "a.go", "package theme\nimport \"charm.land/lipgloss/v2\"\nvar a = lipgloss.Color(\"03\")\n", "literal ANSI index"},
		{"theme non-literal colour", "internal/ui/theme", "a.go", "package theme\nimport \"charm.land/lipgloss/v2\"\nvar n = \"3\"\nvar a = lipgloss.Color(n)\n", "literal ANSI index"},
		{"theme non-string literal", "internal/ui/theme", "a.go", "package theme\nimport \"charm.land/lipgloss/v2\"\nvar a = lipgloss.Color(3)\n", "literal ANSI index"},
		{"theme colour arity", "internal/ui/theme", "a.go", "package theme\nimport \"charm.land/lipgloss/v2\"\nvar a = lipgloss.Color()\n", "literal ANSI index"},
		{"views use theme names", "internal/ui/reports", "a.go", "package reports\nimport lg \"charm.land/lipgloss/v2\"\nvar a = lg.Color(\"1\")\n", "lipgloss.Color outside internal/ui/theme"},
		{"hex colour", "internal/ui/theme", "a.go", "package theme\nvar a = \"#1e1e2e\"\n", "hex colour \"#1e1e2e\""},
		{"short hex colour", "internal/ui/widgets", "a.go", "package widgets\nvar a = \"#fff\"\n", "hex colour \"#fff\""},
		{"non-colour hash strings fine", "internal/ui/widgets", "a.go", "package widgets\nvar a, b = \"#12\", \"# heading\"\n", ""},
		{"colour tests may use lipgloss.Color", "internal/ui/reports", "a_test.go", "package reports\nimport \"charm.land/lipgloss/v2\"\nvar a = lipgloss.Color(\"#ff0000\")\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := fakeRepo(t, c.dir, c.file, c.src).check()
			switch {
			case c.want == "" && len(got) != 0:
				t.Errorf("want no violations, got %q", got)
			case c.want != "" && (len(got) != 1 || !strings.Contains(got[0], c.want)):
				t.Errorf("want one violation containing %q, got %q", c.want, got)
			}
		})
	}
}
