// Package main generates Sh convenience methods that delegate to package-level
// helper functions. This unified generator handles both lesiw.io/fs and
// lesiw.io/command helpers.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

type Config struct {
	SourcePkg  string
	ParamType  string
	OutputFile string
	SkipFuncs  []string
	FixLinks   bool
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// Default skip lists for known packages
var defaultSkips = map[string][]string{
	"lesiw.io/command": {
		"Shell",      // Constructor, not a helper
		"FS",         // Sh caches FS, manually implemented
		"OS",         // Sh caches OS, manually implemented
		"Arch",       // Sh caches Arch, manually implemented
		"Env",        // Sh must probe sh.m, manually implemented
		"Shutdown",   // Sh must delegate to sh.m, manually implemented
		"Handle",     // Manually implemented in sh.go
		"HandleFunc", // Manually implemented in sh.go
		"Unshell",    // Manually implemented in sh.go
	},
}

func run() error {
	var cfg Config

	flag.StringVar(&cfg.SourcePkg, "p", "", "Source package")
	flag.StringVar(&cfg.ParamType, "t", "", "Param type")
	flag.BoolVar(&cfg.FixLinks, "f", false, "Fix doc links")
	flag.Parse()

	if cfg.SourcePkg == "" || cfg.ParamType == "" {
		flag.Usage()
		return fmt.Errorf("required: -p, -t")
	}

	// Derive output filename from param type: FS -> sh_fs.go
	cfg.OutputFile = "sh_" + strings.ToLower(cfg.ParamType) + ".go"

	// Use default skip list for this package
	if defaults, ok := defaultSkips[cfg.SourcePkg]; ok {
		cfg.SkipFuncs = defaults
	}

	return generate(&cfg)
}

func generate(cfg *Config) error {
	// Load source package with types
	pkgCfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles |
			packages.NeedSyntax | packages.NeedTypes |
			packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(pkgCfg, cfg.SourcePkg)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", cfg.SourcePkg, err)
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("%s package not found", cfg.SourcePkg)
	}
	if len(pkgs[0].Errors) > 0 {
		return fmt.Errorf("errors loading %s: %v",
			cfg.SourcePkg, pkgs[0].Errors,
		)
	}

	pkg := pkgs[0]

	// Create skip set
	skipMap := make(map[string]struct{})
	for _, name := range cfg.SkipFuncs {
		skipMap[strings.TrimSpace(name)] = struct{}{}
	}

	// Extract helper functions using type information
	var funcs []FuncInfo
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		if _, skip := skipMap[name]; skip {
			continue
		}

		obj := scope.Lookup(name)
		fn, ok := obj.(*types.Func)
		if !ok || !fn.Exported() {
			continue
		}

		sig := fn.Signature()
		if isHelperSig(sig, cfg.ParamType, pkg.Types) {
			info := extractFuncInfo(fn, cfg, pkg)
			funcs = append(funcs, info)
		}
	}

	// Sort by name for stable output
	sort.Slice(funcs, func(i, j int) bool {
		return funcs[i].Name < funcs[j].Name
	})

	// Collect imports
	importSet := map[string]struct{}{"context": {}}
	for _, f := range funcs {
		for imp := range f.Imports {
			importSet[imp] = struct{}{}
		}
	}
	var imports []string
	for imp := range importSet {
		imports = append(imports, imp)
	}
	sort.Strings(imports)

	// Generate file
	var buf bytes.Buffer
	data := struct {
		SourcePkg string
		Funcs     []FuncInfo
		Imports   []string
	}{
		SourcePkg: cfg.SourcePkg,
		Funcs:     funcs,
		Imports:   imports,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile(cfg.OutputFile, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", cfg.OutputFile, err)
	}

	fmt.Printf("Generated %s with %d methods\n", cfg.OutputFile, len(funcs))
	return nil
}

// isHelperSig checks if a function signature matches the helper pattern:
// - Pattern 1: (ctx context.Context, param ParamType, ...)
// - Pattern 2: (param ParamType, ...) for non-ctx functions
func isHelperSig(sig *types.Signature, paramTypeName string, pkg *types.Package) bool {
	params := sig.Params()
	if params.Len() < 1 {
		return false
	}

	// Look up the expected param type in the package scope
	obj := pkg.Scope().Lookup(paramTypeName)
	if obj == nil {
		return false
	}
	paramType := obj.Type()

	// Pattern 1: (ctx context.Context, param ParamType, ...)
	if params.Len() >= 2 {
		var (
			firstParam  = params.At(0)
			secondParam = params.At(1)
		)

		// Check if first is context.Context and second matches paramType
		match := isContextType(firstParam.Type()) &&
			types.Identical(secondParam.Type(), paramType)
		if match {
			return true
		}
	}

	// Pattern 2: (param ParamType, ...)
	firstParam := params.At(0)
	return types.Identical(firstParam.Type(), paramType)
}

// isContextType checks if a type is context.Context
func isContextType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj != nil && obj.Pkg() != nil &&
		obj.Pkg().Path() == "context" && obj.Name() == "Context"
}

type FuncInfo struct {
	Name       string
	Doc        []string
	Params     string
	Args       string
	Results    string
	ReturnStmt string
	HasCtx     bool
	RecvCall   string
	FuncCall   string
	Imports    map[string]struct{}
}

func extractFuncInfo(fn *types.Func, cfg *Config, pkg *packages.Package) FuncInfo {
	sig := fn.Signature()

	// Create qualifier for type strings
	qf := func(p *types.Package) string {
		if p == nil || p.Path() == "lesiw.io/command" {
			return ""
		}
		return p.Name()
	}

	imports := make(map[string]struct{})
	var params, args, results []string

	// Detect if this is a ctx-based function
	var hasCtx bool
	skipCount := 1 // Default: skip only param

	if sig.Params().Len() >= 2 && isContextType(sig.Params().At(0).Type()) {
		hasCtx = true
		skipCount = 2 // Skip both ctx and param
		imports["context"] = struct{}{}
	}

	// For FS type, need to call sh.FS() instead of sh
	funcCall := fn.Name()
	recvCall := "sh"
	if cfg.ParamType == "FS" {
		recvCall = "sh.FS()"
		funcCall = "fs." + fn.Name()
		imports["lesiw.io/fs"] = struct{}{}
	}

	// Extract remaining params
	for i := skipCount; i < sig.Params().Len(); i++ {
		var (
			param  = sig.Params().At(i)
			typStr = types.TypeString(param.Type(), qf)
		)

		collectImports(param.Type(), imports)

		// Handle variadic
		if sig.Variadic() && i == sig.Params().Len()-1 {
			typStr = "..." + typStr[2:] // []T -> ...T
			args = append(args, param.Name()+"...")
		} else {
			args = append(args, param.Name())
		}

		params = append(params, param.Name()+" "+typStr)
	}

	// Extract return types
	for result := range sig.Results().Variables() {
		typStr := types.TypeString(result.Type(), qf)

		// Swap Machine return type with *Sh
		if typStr == "Machine" {
			typStr = "*Sh"
		}

		collectImports(result.Type(), imports)
		results = append(results, typStr)
	}

	// Build param/result strings
	var paramStr string
	if len(params) > 0 {
		paramStr = ", " + strings.Join(params, ", ")
	}

	var argsStr string
	if len(args) > 0 {
		argsStr = ", " + strings.Join(args, ", ")
	}

	var resultsStr string
	returnStmt := "return "
	if len(results) == 0 {
		returnStmt = ""
	} else if len(results) == 1 {
		resultsStr = results[0]
	} else {
		resultsStr = "(" + strings.Join(results, ", ") + ")"
	}

	// Extract documentation from AST
	var docLines []string
	docText := findFuncDoc(fn, pkg)
	if docText != "" {
		lines := strings.Split(strings.TrimSuffix(docText, "\n"), "\n")
		if cfg.FixLinks {
			for i, line := range lines {
				lines[i] = fixLinks(line, cfg.SourcePkg)
			}
		}
		docLines = append(docLines, lines...)
		docLines = append(docLines, "")

		// Add reference
		var pkgPrefix string
		if cfg.FixLinks {
			pkgPrefix = cfg.SourcePkg + "."
		}
		ref := fmt.Sprintf(
			"This is a convenience method that calls [%s%s].",
			pkgPrefix,
			fn.Name(),
		)
		docLines = append(docLines, ref)
	}

	return FuncInfo{
		Name:       fn.Name(),
		Doc:        docLines,
		Params:     paramStr,
		Args:       argsStr,
		Results:    resultsStr,
		ReturnStmt: returnStmt,
		HasCtx:     hasCtx,
		RecvCall:   recvCall,
		FuncCall:   funcCall,
		Imports:    imports,
	}
}

// findFuncDoc finds the documentation for a function from the AST
func findFuncDoc(fn *types.Func, pkg *packages.Package) string {
	pos := fn.Pos()
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if funcDecl.Name.Pos() == pos && funcDecl.Doc != nil {
				return funcDecl.Doc.Text()
			}
		}
	}
	return ""
}

// collectImports walks a type and collects needed imports
func collectImports(t types.Type, imports map[string]struct{}) {
	switch t := t.(type) {
	case *types.Named:
		if pkg := t.Obj().Pkg(); pkg != nil {
			imports[pkg.Path()] = struct{}{}
		}
	case *types.Pointer:
		collectImports(t.Elem(), imports)
	case *types.Slice:
		collectImports(t.Elem(), imports)
	case *types.Array:
		collectImports(t.Elem(), imports)
	case *types.Map:
		collectImports(t.Key(), imports)
		collectImports(t.Elem(), imports)
	case *types.Chan:
		collectImports(t.Elem(), imports)
	case *types.Signature:
		for v := range t.Params().Variables() {
			collectImports(v.Type(), imports)
		}
		for v := range t.Results().Variables() {
			collectImports(v.Type(), imports)
		}
	}
}

// fixLinks fixes unqualified links in doc comments by prepending package path.
func fixLinks(line, pkg string) string {
	re := regexp.MustCompile(`\[([A-Za-z0-9_]+)\]`)
	return re.ReplaceAllStringFunc(line, func(match string) string {
		ident := match[1 : len(match)-1]
		if strings.Contains(ident, ".") {
			return match
		}
		return "[" + pkg + "." + ident + "]"
	})
}

var tmpl = template.Must(template.New("shgen").Parse(
	`// Code generated by go generate; DO NOT EDIT.
// This file is generated from {{.SourcePkg}} package helper functions.

package command

import (
{{range .Imports}}	"{{.}}"
{{end}})

// The methods below are convenience wrappers that delegate to {{.SourcePkg}}
// package-level helper functions. These are kept in sync with {{.SourcePkg}}
// via code generation to ensure Sh provides ergonomic access to all
{{if eq .SourcePkg "lesiw.io/fs"}}//
// filesystem operations with dynamic fallback to underlying helpers.
{{else}}//
// command operations.
{{end}}
{{range .Funcs}}
{{range .Doc}}// {{.}}
{{end}}{{if .HasCtx}}func (sh *Sh) {{.Name}}(
	ctx context.Context{{.Params}},
) {{.Results}} {
	{{.ReturnStmt}}{{.FuncCall}}(ctx, {{.RecvCall}}{{.Args}})
}{{else}}func (sh *Sh) {{.Name}}({{if .Params}}
	{{slice .Params 2}},
{{end}}) {{.Results}} {
	{{.ReturnStmt}}{{.FuncCall}}({{.RecvCall}}{{.Args}})
}{{end}}

{{end}}
`,
))
