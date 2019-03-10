package main

import (
	"fmt"
	"os"

	"github.com/kazu/lonacha/structer"
	"go.uber.org/zap"
)

var Logger *zap.Logger

func SetupLogger() {

	Logger, _ = zap.NewProduction()

}

func main() {
	if len(os.Args) < 4 {
		fmt.Fprint(os.Stderr, "gen src pkgname struct_name template")
		return
	}

	src := os.Args[1]
	pkgname := os.Args[2]
	structName := os.Args[3]
	template := os.Args[4]

	SetupLogger()

	sinfos, err := structer.StrcutInfos(src, pkgname)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
	}
	for _, info := range sinfos {
		if info.Name != structName {
			continue
		}
		newSrc, err := sinfos[0].FromTemplate(template)
		if err == nil {
			fmt.Print(newSrc)
		} else {
			fmt.Fprint(os.Stderr, err)
		}
	}

}
