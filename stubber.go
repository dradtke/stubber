// Stubber is a tool to automate the creation of "stubbed" interface implementations.
//
// An interface's stubbed implementation is a struct, satisfying the interface,
// that contains one field for each of the interface's methods. When one of
// these methods is called, it uses the backing field as its implementation, or
// panics if none was defined. This allows the behavior of the stub to be
// modified at run-time in a type-safe way, while only requiring you to define
// the methods that will actually be called.
//
// For example, given this simple interface:
//
//	type SessionManager interface {
//		GetUserID(db *sql.DB, username string) (int64, error)
//	}
//
// Then its stub would look like this:
//
//      type StubbedSessionManager struct {
//      	GetUserIDStub  func(db *sql.DB, username string) (int64, error)
//      	getUserIDCalls []struct {
//      		Db       *sql.DB
//      		Username string
//      	}
//      }
//
//      func (s *StubbedSessionManager) GetUserID(db *sql.DB, username string) (int64, error) {
//      	if s.GetUserIDStub == nil {
//      		panic("StubbedSessionManager.GetUserID: nil method stub")
//      	}
//      	s.getUserIDCalls = append(s.getUserIDCalls, struct {
//      		Db       *sql.DB
//      		Username string
//      	}{Db: db, Username: username})
//      	return (s.GetUserIDStub)(db, username)
//      }
//
//      func (s *StubbedSessionManager) GetUserIDCalls() []struct {
//      	Db       *sql.DB
//      	Username string
//      } {
//      	return s.getUserIDCalls
//      }
//
// Note that StubbedSessionManager implements the SessionManager interface, and that its
// implementation of GetUserID()  uses the backing field GetUserIDStub.
//
// Here's an example of how it would be used in a test:
//
//	func TestSomething(t *testing.T) {
//		sm := &StubbedSessionManager{
//			GetUserIDStub: func(db *sql.DB, username string) (int64, error) {
//				return 0, nil // or whatever implementation you want
//			},
//		}
//
//		// Use sm here anywhere a SessionManager is accepted
//	}
//
// Assuming the rest of your code is built around an actual implementation of
// SessionManager, and that your methods take their dependent resources as
// parameters, then this provides an easy way to mock out service calls, but
// in an easy-to-understand, type-safe manner.
//
// See the example folder for more information.
package main

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
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/tools/imports"
)

var (
	t = template.Must(template.New("").Parse(`// This file was generated by stubber; DO NOT EDIT
	
package {{.Package.OutputName}}

{{range $interface := .Interfaces}}
// {{.ImplName}} is a stubbed implementation of {{if $.Package.External}}{{$.Package.InputName}}.{{end}}{{.Name}}.
type {{.ImplName}} struct {
	{{range .Funcs -}}
	// {{.StubName}} defines the implementation for {{.Name}}.
	{{.StubName}} func({{.ParamsString}}) {{.ResultsString}}
	{{.CallsName false}} []{{.ParamsStruct}}
	{{end}}
}

{{range .Funcs}}
// {{.Name}} delegates its behavior to the field {{.StubName}}.
func (s *{{$interface.ImplName}}) {{.Name}}({{.ParamsString}}) {{.ResultsString}} {
	if s.{{.StubName}} == nil {
		panic("{{$interface.ImplName}}.{{.Name}}: nil method stub")
	}
	s.{{.CallsName false}} = append(s.{{.CallsName false}}, {{.ParamsStruct}}{ {{.ParamsStructValues}} })
	{{if .HasResults}}return {{end}}(s.{{.StubName}})({{.ParamNames}})
}

// {{.CallsName true}} returns a slice of calls made to {{.Name}}. Each element
// of the slice represents the parameters that were provided.
func (s *{{$interface.ImplName}}) {{.CallsName true}}() []{{.ParamsStruct}} {
	return s.{{.CallsName false}}
}
{{end}}

// Compile-time check that the implementation matches the interface.
var _ {{if $.Package.External}}{{$.Package.InputName}}.{{end}}{{.Name}} = (*{{.ImplName}})(nil)
{{end}}
`))
)

func main() {
	var (
		typeNames = flag.String("types", "", "comma-separated list of type names; defaults to all interfaces")
		outputDir = flag.String("output", "", "path to output directory; '-' will write result to stdout")
	)

	log.SetFlags(0)
	log.SetPrefix("stubber: ")
	flag.Parse()

	var types []string
	if *typeNames != "" {
		types = strings.Split(*typeNames, ",")
	}

	// Default to the current directory, but grab the first argument as the dir if it's available.
	inputDir := "."
	if args := flag.Args(); len(args) > 0 {
		inputDir = args[0]
	}

	var out io.Writer
	if *outputDir == "-" {
		out = os.Stdout
	} else if *outputDir == "" {
		*outputDir = inputDir
	}

	Main(types, inputDir, *outputDir, out)
}

func Main(types []string, inputDir, outputDir string, out io.Writer) {
	inputDirInfo, err := os.Stat(inputDir)
	if err != nil {
		log.Fatalf("cannot stat input dir: %s", err)
	}

	if outputDir != "" {
		if err := os.MkdirAll(outputDir, inputDirInfo.Mode()); err != nil {
			log.Fatalf("cannot make output directory: %s", err)
		}
	}

	pkg := NewPackage(inputDir, outputDir)
	pkg.Check(types)

	var buf bytes.Buffer
	for filename, interfaces := range pkg.Defs {
		buf.Reset()
		if err := t.Execute(&buf, struct {
			Package    *Package
			Interfaces []Interface
		}{
			Package:    pkg,
			Interfaces: interfaces,
		}); err != nil {
			log.Fatal(err)
		}

		newFilename := filename[:len(filename)-3] + "_stubs.go"
		code, err := imports.Process(newFilename, buf.Bytes(), nil)
		if err != nil {
			log.Print(buf.String())
			log.Fatalf("cannot process imports: %s", err)
		}

		if out != nil {
			if _, err := out.Write(code); err != nil {
				log.Fatalf("failed to write result: %s", err)
			}
		} else {
			if err := ioutil.WriteFile(filepath.Join(outputDir, newFilename), code, 0644); err != nil {
				log.Fatalf("failed to write output file %s: %s", newFilename, err)
			}
		}
	}
}

type Package struct {
	dir   string
	files map[string]*ast.File
	fs    *token.FileSet
	// OutputName is the name of the output package.
	OutputName string
	// InputName is the name of the input package.
	InputName string
	// External is true if the output is in a different package than the
	// input, indicating that the interface name needs to be qualified if
	// referenced.
	External bool
	Scope    *types.Scope
	Defs     map[string][]Interface
}

func NewPackage(inputDir, outputDir string) *Package {
	pkg, err := build.Default.ImportDir(inputDir, 0)
	if err != nil {
		log.Fatalf("cannot process directory %s: %s", inputDir, err)
	}

	files := make(map[string]*ast.File)
	fs := token.NewFileSet()
	for _, name := range pkg.GoFiles {
		if strings.HasSuffix(name, "_stubs.go") {
			continue
		}
		fullName := filepath.Join(inputDir, name)
		parsed, err := parser.ParseFile(fs, fullName, nil, 0)
		if err != nil {
			log.Fatalf("cannot parse file %s: %s", fullName, err)
		}
		files[name] = parsed
	}

	if len(files) == 0 {
		log.Fatalf("%s: no buildable Go files", inputDir)
	}

	p := &Package{
		dir:        inputDir,
		files:      files,
		fs:         fs,
		InputName:  pkg.Name,
		OutputName: pkg.Name,
	}

	if outputDir != "" && outputDir != inputDir {
		p.OutputName = filepath.Base(outputDir)
		p.External = true
	}

	return p
}

func (p *Package) TypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + p.TypeName(t.Elt)
		} else {
			log.Fatal("Package.TypeName: don't know how to handle non-slice arrays yet")
		}

	case *ast.Ident:
		if p.External && p.Scope.Lookup(t.Name) != nil {
			return p.InputName + "." + t.Name
		}
		return t.Name

	case *ast.SelectorExpr:
		return p.TypeName(t.X) + "." + t.Sel.Name

	case *ast.StarExpr:
		return "*" + p.TypeName(t.X)

	case *ast.MapType:
		return "map[" + p.TypeName(t.Key) + "]" + p.TypeName(t.Value)

	case *ast.InterfaceType:
		if t.Methods != nil && len(t.Methods.List) > 0 {
			log.Fatalf("Package.TypeName: does not currently support non-empty interface literal")
		}
		return "interface{}"

	case *ast.Ellipsis:
		return "..." + p.TypeName(t.Elt)

	case *ast.FuncType:
		var (
			params  = p.FieldListString(t.Params)
			results = p.FieldListString(t.Results)
		)
		return fmt.Sprintf("func(%s) (%s)", params, results)

	default:
		log.Fatalf("Package.TypeName: unknown node type: %T", t)
	}

	return ""
}

func (p *Package) FieldListString(fl *ast.FieldList) string {
	var values []string
	for _, field := range fl.List {
		var names []string
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
		values = append(values, strings.Join(names, ", ")+" "+p.TypeName(field.Type))
	}
	return strings.Join(values, ", ")
}

func (p *Package) Check(ts []string) {
	config := types.Config{Importer: importer.For("source", nil)}
	info := types.Info{}
	files := make([]*ast.File, 0, len(p.files))
	for _, f := range p.files {
		files = append(files, f)
	}
	pkg, err := config.Check(p.dir, p.fs, files, &info)
	if err != nil {
		log.Fatalf("cannot check package: %s", err)
	}
	p.Defs = make(map[string][]Interface)
	p.Scope = pkg.Scope()
	for filename, f := range p.files {
		ast.Inspect(f, p.genDecl(filename, ts))
	}
}

func (p *Package) genDecl(filename string, ts []string) func(ast.Node) bool {
	return func(node ast.Node) bool {
		if decl, ok := node.(*ast.GenDecl); ok && decl.Tok == token.TYPE {
			for _, spec := range decl.Specs {
				tspec := spec.(*ast.TypeSpec)
				itype, ok := tspec.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				iface := Interface{
					Name:     tspec.Name.Name,
					External: p.External,
				}

				// If any type names were specified, make sure this type was included.
				if len(ts) > 0 {
					var include bool
					for _, typ := range ts {
						if typ == iface.Name {
							include = true
							break
						}
					}
					if !include {
						continue
					}
				}

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
							if len(param.Names) == 0 {
								ifunc.Params = append(ifunc.Params, Var{
									Type: p.TypeName(param.Type),
								})
							} else {
								for _, ident := range param.Names {
									ifunc.Params = append(ifunc.Params, Var{
										Type: p.TypeName(param.Type),
										Name: ident.Name,
									})
								}
							}
						}
					}

					if ftype.Results != nil {
						for _, result := range ftype.Results.List {
							if len(result.Names) == 0 {
								ifunc.Results = append(ifunc.Results, Var{Type: p.TypeName(result.Type)})
							} else {
								for _, ident := range result.Names {
									ifunc.Results = append(ifunc.Results, Var{
										Type: p.TypeName(result.Type),
										Name: ident.Name,
									})
								}
							}
						}
					}

					iface.Funcs = append(iface.Funcs, ifunc)
				}
				p.Defs[filename] = append(p.Defs[filename], iface)
			}
		}
		return true
	}
}

type Interface struct {
	Name, QualifiedName string
	Funcs               []Func
	External            bool
}

func (i *Interface) ImplName() string {
	if i.External {
		return i.Name
	}
	return "Stubbed" + i.Name
}

type Func struct {
	Name    string
	Params  []Var
	Results []Var
}

func (f *Func) StubName() string {
	return f.Name + "Stub"
}

func (f *Func) CallsName(public bool) string {
	if public {
		return f.Name + "Calls"
	}
	return string(unicode.ToLower(rune(f.Name[0]))) + f.Name[1:] + "Calls"
}

func (f *Func) ParamsString() string {
	return joinVars(f.Params, ", ", false, false)
}

func (f *Func) ParamsStruct() string {
	return "struct{" + joinVars(f.Params, "; ", true, true) + "}"
}

func (f *Func) ParamsStructValues() string {
	var buf bytes.Buffer
	for _, v := range f.Params {
		buf.WriteString(publicize(v.Name) + ": " + v.Name + ",")
	}
	return buf.String()
}

func (f *Func) ParamNames() string {
	parts := make([]string, 0, len(f.Params))
	for _, v := range f.Params {
		name := v.Name
		if isVariadic(v.Type) {
			name = name + "..."
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, ", ")
}

func (f *Func) ResultsString() string {
	s := joinVars(f.Results, ", ", false, false)
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

func joinVars(vars []Var, sep string, public, inStruct bool) string {
	parts := make([]string, 0, len(vars))
	for _, v := range vars {
		name, typ := v.Name, v.Type
		if public {
			name = publicize(name)
		}
		if inStruct && isVariadic(v.Type) {
			typ = "[]" + v.Type[3:]
		}
		parts = append(parts, name+" "+typ)
	}
	return strings.Join(parts, sep)
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

// TODO: improve this to make some variable names more readable, e.g. "db" -> "DB"
func publicize(name string) string {
	return string(unicode.ToTitle(rune(name[0]))) + name[1:]
}

func isVariadic(typ string) bool {
	return strings.HasPrefix(typ, "...")
}
