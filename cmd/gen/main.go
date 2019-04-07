package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/kazu/lonacha/structer"
	"go.uber.org/zap"
)

var Logger *zap.Logger

func SetupLogger() {

	Logger, _ = zap.NewProduction()

}

func main() {
	if len(os.Args) < 5 {
		fmt.Fprintf(os.Stderr, "gen pkgname src struct_name template src="+os.Args[1])
		return
	}

	pkgname := os.Args[1]
	src := os.Args[2]
	structName := os.Args[3]
	template := os.Args[4]
	output := ""
	if len(os.Args) > 5 {
		output = os.Args[5]
	}

	SetupLogger()

	sinfos, err := structer.StrcutInfos(src, pkgname)
	if err != nil || len(sinfos) == 0 {
		fmt.Fprintf(os.Stderr, "err=%s skip parsing file\n", err)
		sinfos = append(sinfos, structer.StructInfo{
			PkgName: pkgname,
			Name:    structName,
		})
	}

	for _, info := range sinfos {
		if info.Name != structName {
			fmt.Fprintf(os.Stderr, "WARN: skip %s \n", info.Name)
			continue
		}
		newSrc, err := sinfos[0].FromTemplate(template)
		if err == nil {
			if len(output) > 0 {
				ioutil.WriteFile(output, []byte(newSrc), 0644)
			} else {
				fmt.Print(newSrc)
			}
		} else {
			fmt.Fprint(os.Stderr, err)
		}
		return
	}

}
