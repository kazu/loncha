package container_list

import (
	"io"

	"github.com/clipperhouse/typewriter"
)

func init() {
	err := typewriter.Register(NewContainerListWriter())
	if err != nil {
		panic(err)
	}
}

type ContainerListWriter struct{}

func NewContainerListWriter() *ContainerListWriter {
	return &ContainerListWriter{}
}

func (sw *ContainerListWriter) Name() string {
	return "ContainerList"
}

func (sw *ContainerListWriter) Imports(t typewriter.Type) (result []typewriter.ImportSpec) {
	return []typewriter.ImportSpec{
		{Path: "unsafe"},
		{Path: "github.com/kazu/loncha/list_head"},
	}
}
func (sw *ContainerListWriter) Write(w io.Writer, t typewriter.Type) error {
	tag, found := t.FindTag(sw)

	if !found {
		// nothing to be done
		return nil
	}

	license := `
// ContainerListWriter is a base of http://golang.org/pkg/container/list/
// this is tuning performancem, reduce heap usage.
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
`
	if _, err := w.Write([]byte(license)); err != nil {
		return err
	}

	tmpl, err := templates.ByTag(t, tag)

	if err != nil {
		return err
	}

	if err := tmpl.Execute(w, t); err != nil {
		return err
	}

	return nil
}
