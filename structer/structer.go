package structer

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

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

func LoadAstf(dir string, fset *token.FileSet) (astfs []*ast.File, err error) {

	files, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return
	}

	for _, file := range files {
		if filepath.Ext(file) != ".go" {
			continue
		}
		if match, _ := filepath.Match("*_test.go", filepath.Base(file)); match {
			continue
		}

		astf, err := parser.ParseFile(fset, file, LoadFile(file), 0)
		if err != nil {
			return nil, err
		}
		astfs = append(astfs, astf)
	}
	return
}

func StrcutInfos(src string, pkgname string) (infos []StructInfo, err error) {

	SetupLogger()

	fset := token.NewFileSet()

	//astf, _ := parser.ParseFile(fset, src, LoadFile(src), 0)
	astfs, err := LoadAstf(path.Dir(src), fset)
	if err != nil {
		err = errors.Wrap(err, "parse fail")
		return
	}

	conf := types.Config{Importer: importer.For("source", nil), DisableUnusedImportCheck: true}
	pkg, err := conf.Check(pkgname, fset, astfs, nil)
	_ = pkg
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("fail check\n\tastfs=%+v\n\tfset=%+v\n\tpkg=%s\n\n", astfs, fset, spew.Sdump(pkg)))
		return
	}

	Logger.Info("Package info",
		zap.String("Package", pkg.Path()),
		zap.String("Name", pkg.Name()),
		zap.Reflect("Imports", pkg.Imports()),
		zap.Strings("Scope.Names", pkg.Scope().Names()),
		zap.String("Scope", pkg.Scope().String()))

	scope := pkg.Scope()

	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		_ = obj

		internal, found_struct := obj.Type().Underlying().(*types.Struct)
		if !found_struct {
			continue
		}
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
