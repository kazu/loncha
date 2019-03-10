package main

import (
	"testing"

	"github.com/kazu/lonacha/structer"
	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {

	sinfos, err := structer.StrcutInfos("../../sample/hoge/sample.go", "hoge")
	assert.NoError(t, err)
	assert.NotNil(t, sinfos)

	str, err := sinfos[0].FromTemplate("../../container_list/container_list.gtpl")
	assert.NoError(t, err)

	assert.True(t, len(str) > 0)
}
