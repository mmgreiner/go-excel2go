package main

import (
	"testing"

	"github.com/mmgreiner/go-utils/excel"
	"gotest.tools/assert"
)

//go:generate ./go-excel2go -out "exceltype.go" -type ExcelType -package main ./excel_test.xlsx

func TestReadExcel(t *testing.T) {
	println("testing")
	data := []ExcelType{}
	err := excel.ReadExcel("excel_test.xlsx", true, ExcelType_fromRow, &data)
	assert.Assert(t, err)
	assert.Assert(t, len(data) > 0)
}
