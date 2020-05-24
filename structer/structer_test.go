package structer_test

import (
	"testing"

	"github.com/kazu/loncha/structer"
)

func TestStructInfo(t *testing.T) {
	src := "../example/structer_def.go"
	pkgname := "example"
	sinfos, err := structer.StrcutInfos(src, pkgname)

	if err != nil {
		t.Errorf("StructInfos() err=%s", err.Error())
	}

	if len(sinfos) == 0 {
		t.Errorf("must len(StructInfos())=%d > 0 ", len(sinfos))
	}
}
