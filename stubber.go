package main

// TODO: formatting and importing

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"html/template"
	"log"
	"path/filepath"
	"strings"

	"golang.org/x/tools/imports"
)

var (
	typeNames = flag.String("type", "", "comma-separated list of type names; must be set")
	output    = flag.String("output", "", "output file name; default srcdir/<type>_stub.go")

	t = template.Must(template.New("").Parse(`package {{.Name}}

{{range $interface := .Interfaces}}
type Stubbed{{.Name}} struct {
	{{range .Funcs -}}
	{{.Name}}Stub func({{.ParamsString}}) {{.ResultsString}}
	{{- end}}
}

{{range .Funcs}}
func (s *Stubbed{{$interface.Name}}) {{.Name}}({{.ParamsString}}) {{.ResultsString}} {
	if s.{{.Name}}Stub == nil {
		panic("Stubbed{{$interface.Name}}.{{.Name}}Stub is undefined")
	}
	{{if .HasResults}}return {{end}}(s.{{.Name}}Stub)({{.ParamNames}})
}
{{end}}
{{end}}
`))
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("stubber: ")
	flag.Parse()

	// ts := strings.Split(*typeNames, ",")

	// Default to the current directory, but grab the first argument as the dir if it's available.
	dir := "."
	if args := flag.Args(); len(args) > 0 {
		dir = args[0]
	}

	pkg := NewPackage(dir)
	pkg.Check()

	var buf bytes.Buffer
	if err := t.Execute(&buf, pkg); err != nil {
		log.Fatal(err)
	}

	code, err := imports.Process(pkg.Name+"_stubbed.go", buf.Bytes(), nil)
	if err != nil {
		log.Fatal("cannot process imports: %s", err)
	}

	fmt.Println(string(code))
}

type Package struct {
	dir   string
	files []*ast.File
	fs    *token.FileSet

	Name       string
	Interfaces []Interface
}

func NewPackage(directory string) *Package {
	pkg, err := build.Default.ImportDir(directory, 0)
	if err != nil {
		log.Fatalf("cannot process directory %s: %s", directory, err)
	}

	var files []*ast.File
	fs := token.NewFileSet()
	for _, name := range pkg.GoFiles {
		name = filepath.Join(directory, name)
		parsed, err := parser.ParseFile(fs, name, nil, 0)
		if err != nil {
			log.Fatalf("cannot parse file %s: %s", name, err)
		}
		files = append(files, parsed)
	}

	if len(files) == 0 {
		log.Fatalf("%s: no buildable Go files", directory)
	}

	return &Package{
		dir:   directory,
		files: files,
		fs:    fs,
		Name:  pkg.Name,
	}
}

func (p *Package) Check() {
	config := types.Config{Importer: importer.Default()}
	info := types.Info{}
	if _, err := config.Check(p.dir, p.fs, p.files, &info); err != nil {
		log.Fatalf("cannot check package: %s", err)
	}
	for _, f := range p.files {
		ast.Inspect(f, p.genDecl)
	}
}

func (p *Package) genDecl(node ast.Node) bool {
	// fmt.Printf("%T\n", node)
	if decl, ok := node.(*ast.GenDecl); ok && decl.Tok == token.TYPE {
		for _, spec := range decl.Specs {
			tspec := spec.(*ast.TypeSpec)
			itype, ok := tspec.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			iface := Interface{Name: tspec.Name.Name}
			for _, method := range itype.Methods.List {
				ftype, ok := method.Type.(*ast.FuncType)
				if !ok {
					continue
				}

				ifunc := Func{Name: method.Names[0].Name}
				if ifunc.Name == "_" {
					continue
				}

				if ftype.Params != nil {
					for _, param := range ftype.Params.List {
						v := Var{Type: typeName(param.Type)}
						if len(param.Names) > 0 {
							v.Name = param.Names[0].Name
						}
						ifunc.Params = append(ifunc.Params, v)
					}
				}

				if ftype.Results != nil {
					for _, result := range ftype.Results.List {
						v := Var{Type: typeName(result.Type)}
						if len(result.Names) > 0 {
							v.Name = result.Names[0].Name
						}
						ifunc.Results = append(ifunc.Results, v)
					}
				}

				iface.Funcs = append(iface.Funcs, ifunc)
			}
			p.Interfaces = append(p.Interfaces, iface)
		}
	}
	return true
}

type Interface struct {
	Name  string
	Funcs []Func
}

type Func struct {
	Name    string
	Params  []Var
	Results []Var
}

func (f *Func) ParamsString() string {
	return joinVars(f.Params)
}

func (f *Func) ParamNames() string {
	parts := make([]string, 0, len(f.Params))
	for _, v := range f.Params {
		parts = append(parts, v.Name)
	}
	return strings.Join(parts, ", ")
}

func (f *Func) ResultsString() string {
	s := joinVars(f.Results)
	if len(f.Results) > 1 || (f.HasResults() && f.Results[0].Name != "") {
		s = "(" + s + ")"
	}
	return s
}

func (f *Func) HasResults() bool {
	return len(f.Results) > 0
}

type Var struct {
	Name string
	Type string
}

func joinVars(vars []Var) string {
	parts := make([]string, 0, len(vars))
	for _, v := range vars {
		parts = append(parts, v.Name+" "+v.Type)
	}
	return strings.Join(parts, ", ")
}

func typeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + typeName(t.Elt)
		} else {
			log.Fatal("typeName: don't know how to handle non-slice arrays yet")
		}
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return typeName(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + typeName(t.X)
	default:
		log.Fatalf("typeName: unknown node type: %T", t)
	}

	return ""
}
