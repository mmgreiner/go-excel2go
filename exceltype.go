package main

/*
Automatically generated from file ./excel_test.xlsx on 2023-08-03T14:42:38+02:00
*/

import (
	"time"

	"github.com/mmgreiner/go-utils/str2"
)

type ExcelType struct {
	IntegerValue int       `csv:"Integer Value"`
	DateValue    time.Time `csv:"Date Value"`
	FloatValue   float64   `csv:"Float Value"`
	Text         string    `csv:"Text"`
}

func ExcelType_fromRow(row []string) ExcelType {
	rec := ExcelType{

		IntegerValue: str2.TraceToInt(row[0], "Integer Value"), // Integer Value
		DateValue:    str2.TraceToTime(row[1], "Date Value"),   // Date Value
		FloatValue:   str2.TraceToFloat(row[2], "Float Value"), // Float Value
		Text:         row[3],                                   // Text
	}
	return rec
}
