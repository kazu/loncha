package structer

import (
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"io/ioutil"
	"strings"

	"text/template"

	"github.com/davecgh/go-spew/spew"
	"go.uber.org/zap"
)

type StructInfo struct {
	PkgName  string
	Name     string
	Fields   map[string]string
	Embedded []string
}

var Logger *zap.Logger

func SetupLogger() {
	Logger, _ = zap.NewProduction()
}

func StrcutInfos(src string, pkgname string) (infos []StructInfo, err error) {

	SetupLogger()

	fset := token.NewFileSet()

	astf, err := parser.ParseFile(fset, src, LoadFile(src), 0)
	if err != nil {
		return
	}

	conf := types.Config{Importer: importer.For("source", nil), DisableUnusedImportCheck: true}
	pkg, err := conf.Check(pkgname, fset, []*ast.File{astf}, nil)
	_ = pkg
	if err != nil {
		err = errors.New(err.Error() + "\n" + spew.Sdump(pkg))
		return
	}

	Logger.Debug("Package info",
		zap.String("Package", pkg.Path()),
		zap.String("Name", pkg.Name()),
		zap.Reflect("Imports", pkg.Imports()),
		zap.String("Scope", pkg.Scope().String()))

	scope := pkg.Scope()

	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		_ = obj

		internal := obj.Type().Underlying().(*types.Struct)
		sinfo := StructInfo{
			PkgName: pkgname,
			Name:    obj.Name(),
			Fields:  map[string]string{},
		}
		for i := 0; i < internal.NumFields(); i++ {
			f := internal.Field(i)
			if !f.IsField() {
				continue
			}
			if f.Embedded() {
				sinfo.Embedded = append(sinfo.Embedded, f.Name())
			} else {
				sinfo.Fields[f.Name()] = f.Type().String()
			}
		}
		Logger.Debug("Object",
			zap.String("Object", spew.Sdump(obj)),
			zap.String("Type", spew.Sdump(obj.Type())),
			zap.String("Field:   %s\n", spew.Sdump(sinfo.Fields)))
		infos = append(infos, sinfo)
	}
	return
}

func LoadFile(src string) string {
	data, _ := ioutil.ReadFile(src)
	return string(data)
}

func Dump(s io.Writer, fset *token.FileSet, d ast.Decl) {
	ast.Fprint(s, fset, d, ast.NotNilFilter)
	fmt.Fprint(s, "\n")

}

func (info StructInfo) FromTemplate(path string) (out string, err error) {
	tmpStr := LoadFile(path)

	t := template.Must(template.New("info").Parse(tmpStr))
	s := &strings.Builder{}
	err = t.Execute(s, info)
	out = s.String()
	return
}
