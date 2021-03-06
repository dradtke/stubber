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
	"go/ast"
	"go/format"
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

	"golang.org/x/tools/go/packages"
)

var (
	t = template.Must(template.New("").Parse(`// This file was generated by stubber; DO NOT EDIT

// +build !nostubs
	
package {{.OutputName}}

import (
	{{range $pkg, $empty := .Dependencies}}"{{$pkg}}"
	{{end}}
)

{{range $interface := .Interfaces}}
// {{.ImplName}} is a stubbed implementation of {{.QualName}}.
type {{.ImplName}} struct {
	{{range .Funcs -}}
	// {{.StubName}} defines the implementation for {{.Name}}.
	{{.StubName}} func{{.ParamsString}} {{.ResultsString}}
	{{.CallsName false}} []{{.ParamsStruct}}
	{{end}}
}

{{range .Funcs}}
// {{.Name}} delegates its behavior to the field {{.StubName}}.
func (s *{{$interface.ImplName}}) {{.Name}}{{.ParamsString}} {{.ResultsString}} {
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
var _ {{.QualName}} = (*{{.ImplName}})(nil)
{{end}}
`))
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return strings.Join(*i, ",")
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	var (
		outputDir = flag.String("output", "", "path to output directory; '-' will write result to stdout")
		typeNames = flag.String("types", "", "comma-separated list of type names to stub")
	)
	var renameFlags arrayFlags
	flag.Var(&renameFlags, "rename", "rename an interface to something else in the output")

	log.SetFlags(0)
	log.SetPrefix("stubber: ")
	flag.Parse()

	var types []string
	if *typeNames != "" {
		types = strings.Split(*typeNames, ",")
	}

	// Default to the current directory, but grab the first argument as the dir if it's available.
	inputDirs := flag.Args()
	if len(inputDirs) == 0 {
		inputDirs = []string{"."}
	}

	var out io.Writer
	if *outputDir == "-" {
		out = os.Stdout
		*outputDir = ""
	} else if *outputDir == "" {
		*outputDir = "."
	}

	renames := make(map[string]string)
	for _, rf := range renameFlags {
		parts := strings.Split(rf, "=")
		renames[parts[0]] = parts[1]
	}

	Main(types, inputDirs, *outputDir, out, renames)
}

func Main(types, inputDirs []string, outputDir string, out io.Writer, renames map[string]string) {
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0655); err != nil {
			log.Fatalf("cannot make output directory: %s", err)
		}
	}

	var pkgs []*Package
	for _, inputDir := range inputDirs {
		pkg := NewPackage(inputDir, outputDir)
		pkg.Check(types)
		pkgs = append(pkgs, pkg)
		log.Printf("found package: %s", pkg.InputName)
	}

	// Check for explicit renames.
	for _, pkg := range pkgs {
		for _, iface := range pkg.Interfaces {
			qualName := pkg.Pkg.Name + "." + iface.StubName
			if newName := renames[qualName]; newName != "" {
				iface.StubName = newName
			}
		}
	}

	// Check for duplicate interface names, e.g. "Client"
	defs := make(map[string]int)
	for _, pkg := range pkgs {
		for _, iface := range pkg.Interfaces {
			defs[iface.StubName] += 1
		}
	}
	for name, count := range defs {
		if count <= 1 {
			continue
		}
		for _, pkg := range pkgs {
			for _, iface := range pkg.Interfaces {
				if iface.StubName == name {
					iface.StubName = publicize(pkg.Pkg.Name) + iface.StubName
				}
			}
		}
	}

	var buf bytes.Buffer
	for _, pkg := range pkgs {
		buf.Reset()
		if err := t.Execute(&buf, pkg); err != nil {
			log.Fatal(err)
		}

		code, err := format.Source(buf.Bytes())
		if err != nil {
			log.Println(buf.String())
			log.Fatalf("error formatting stubs: %s", err)
		}

		if out != nil {
			if _, err := out.Write(code); err != nil {
				log.Fatalf("failed to write result: %s", err)
			}
		} else {
			newFilename := filepath.Join(outputDir, pkg.Pkg.Name+"_stubs.go")
			log.Printf("writing %s", newFilename)
			if err := ioutil.WriteFile(newFilename, code, 0644); err != nil {
				log.Fatalf("failed to write output file %s: %s", newFilename, err)
			}
		}
	}
}

type Package struct {
	// OutputName is the name of the output package.
	OutputName string
	// InputName is the name of the input package.
	InputName       string
	Pkg             *packages.Package
	Interfaces      []*Interface
	Dependencies    map[string]struct{}
	DependencyNames map[string]struct{}
}

func NewPackage(inputDir, outputDir string) *Package {
	pkgs, err := packages.Load(&packages.Config{Mode: packages.LoadAllSyntax, BuildFlags: []string{"-tags=nostubs"}}, inputDir)
	if err != nil {
		panic(err)
	}

	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		panic(err)
	}

	p := Package{
		InputName:       pkgs[0].Name,
		OutputName:      filepath.Base(absOutputDir),
		Pkg:             pkgs[0],
		Dependencies:    make(map[string]struct{}),
		DependencyNames: make(map[string]struct{}),
	}
	if outputDir == "" {
		p.OutputName = "stubs"
	}
	return &p
}

func ImportPath(pkgPath string) string {
	parts := strings.Split(pkgPath, "/")
	for len(parts) > 0 {
		path := strings.Join(parts, "/")
		if _, err := packages.Load(nil, path); err == nil {
			log.Println("package " + pkgPath + " successfully imported")
			return path
		}
		parts = parts[1:]
	}
	log.Fatal("unable to import package: " + pkgPath)
	return ""
}

func findInterfaceDefs(pkg *packages.Package) map[*ast.Ident]types.Object {
	m := make(map[*ast.Ident]types.Object)
	for _, f := range pkg.Syntax {
		for _, decl := range f.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok {
				if gen.Tok == token.TYPE {
					for _, spec := range gen.Specs {
						if tipe, ok := spec.(*ast.TypeSpec); ok {
							if def := pkg.TypesInfo.Defs[tipe.Name]; types.IsInterface(def.Type()) {
								m[tipe.Name] = def
							}
						}
					}
				}
			}
		}
	}
	return m
}

func (p *Package) Check(ts []string) {
	p.Dependencies[p.Pkg.PkgPath] = struct{}{}

	for ident, def := range findInterfaceDefs(p.Pkg) {
		// If any type names were specified, make sure this type was included.
		if len(ts) > 0 {
			var include bool
			for _, typ := range ts {
				if typ == ident.Name {
					include = true
					break
				}
			}
			if !include {
				continue
			}
		}

		iface := Interface{
			Pkg:      p,
			Name:     ident.Name,
			QualName: p.InputName + "." + ident.Name,
			StubName: ident.Name,
		}

		itype := def.Type().Underlying().(*types.Interface)
		for i := 0; i < itype.NumMethods(); i++ {
			method := itype.Method(i)
			if method.Name() == "_" {
				continue
			}

			sig := method.Type().(*types.Signature)
			ifunc := Func{
				Interface: &iface,
				Name:      method.Name(),
				Pkg:       p.Pkg.Types,
				Signature: sig,
			}

			for j := 0; j < ifunc.Signature.Params().Len(); j++ {
				if named, ok := indirect(ifunc.Signature.Params().At(j).Type()).(*types.Named); ok {
					if pkg := named.Obj().Pkg(); pkg != nil {
						p.Dependencies[pkg.Path()] = struct{}{}
						p.DependencyNames[pkg.Name()] = struct{}{}
					}
				}
			}

			for j := 0; j < ifunc.Signature.Results().Len(); j++ {
				if named, ok := indirect(ifunc.Signature.Results().At(j).Type()).(*types.Named); ok {
					if pkg := named.Obj().Pkg(); pkg != nil {
						p.Dependencies[pkg.Path()] = struct{}{}
						p.DependencyNames[pkg.Name()] = struct{}{}
					}
				}
			}

			iface.Funcs = append(iface.Funcs, ifunc)

		}
		p.Interfaces = append(p.Interfaces, &iface)
	}
}

type Interface struct {
	Pkg                      *Package
	Name, QualName, StubName string
	Funcs                    []Func
}

func (i *Interface) ImplName() string {
	return i.StubName
}

type Func struct {
	Interface *Interface
	Name      string
	Pkg       *types.Package
	Signature *types.Signature
}

func (f *Func) Qualifier(pkg *types.Package) string {
	return pkg.Name()
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

func ensureNoCollision(name string, depNames map[string]struct{}) string {
	for {
		if _, ok := depNames[name]; !ok {
			return name
		}
		name = "_" + name
	}
}

func (f *Func) ParamsString() string {
	params := make([]string, f.Signature.Params().Len())
	for i := 0; i < len(params); i++ {
		v := f.Signature.Params().At(i)
		name := ensureNoCollision(v.Name(), f.Interface.Pkg.DependencyNames)
		typeString := types.TypeString(v.Type(), f.Qualifier)
		if f.Signature.Variadic() && i == len(params)-1 {
			if slice, ok := v.Type().(*types.Slice); ok {
				typeString = "..." + types.TypeString(slice.Elem(), f.Qualifier)
			}
		}
		params[i] = name + " " + typeString
	}
	return "(" + strings.Join(params, ", ") + ")"
}

func (f *Func) ParamsStruct() string {
	parts := make([]string, f.Signature.Params().Len())
	for i := 0; i < len(parts); i++ {
		param := f.Signature.Params().At(i)
		name := ensureNoCollision(publicize(param.Name()), f.Interface.Pkg.DependencyNames)
		typeString := types.TypeString(param.Type(), f.Qualifier)
		parts[i] = name + " " + typeString
	}
	return "struct{" + strings.Join(parts, ";") + "}"
}

func (f *Func) ParamsStructValues() string {
	var buf bytes.Buffer
	for i := 0; i < f.Signature.Params().Len(); i++ {
		valueName := f.Signature.Params().At(i).Name()
		keyName := publicize(valueName)
		buf.WriteString(ensureNoCollision(keyName, f.Interface.Pkg.DependencyNames) + ": " + ensureNoCollision(valueName, f.Interface.Pkg.DependencyNames) + ",")
	}
	return buf.String()
}

func (f *Func) ParamNames() string {
	var parts []string
	for i := 0; i < f.Signature.Params().Len(); i++ {
		name := ensureNoCollision(f.Signature.Params().At(i).Name(), f.Interface.Pkg.DependencyNames)
		parts = append(parts, name)
	}
	return strings.Join(parts, ", ")
}

func (f *Func) ResultsString() string {
	return types.TypeString(f.Signature.Results(), f.Qualifier)
}

func (f *Func) HasResults() bool {
	return f.Signature.Results().Len() != 0
}

func publicize(name string) string {
	if len(name) == 0 {
		panic("empty name found, make sure all your interface parameters have a name!")
	}
	// Some well-known names can be given better names than the default capitalization algorithm,
	// i.e. DB is better than Db.
	switch name {
	case "db":
		return "DB"
	default:
		return string(unicode.ToTitle(rune(name[0]))) + name[1:]
	}
}

// indirect returns the type that t points to. If it's not a pointer it
// returns its argument.
func indirect(t types.Type) types.Type {
	for {
		ptype, ok := t.(*types.Pointer)
		if !ok {
			return t
		}
		t = ptype.Elem()
	}
}
