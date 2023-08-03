package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"regexp"
	"time"

	"text/template"

	"github.com/mmgreiner/go-utils/str2"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/xuri/excelize/v2"
)

type MyType string

const (
	Undefined MyType = ""
	Integer   MyType = "int"
	Float     MyType = "float64"
	Date      MyType = "time.Time"
	String    MyType = "string"
	Boolean   MyType = "bool"
)

type Header struct {
	ColName string
	GoName  string
	GoType  MyType
	GoConv  string
	Addr    string
}

var converters = map[MyType]string{
	Undefined: "",
	Integer:   "str2.ToInt(",
	Boolean:   "str2.ToBool(",
	Float:     "str2.ToFloat(",
	Date:      "str2.ToTime(",
	String:    "",
}

func (h *Header) setType(typ MyType, tracing bool) {
	h.GoType = typ
	h.GoConv = converters[typ]
	if tracing {
		h.GoConv = strings.ReplaceAll(h.GoConv, "str2.", "str2.Trace")
	}
}

type TemplateInfo struct {
	FileName        string
	Date            string
	PackageName     string
	TypeName        string
	PretypedHeaders map[string]MyType
	Headers         []Header

	// code generation options
	ColMap  bool
	Tracing bool
}

func (t *TemplateInfo) setPretyped(typ MyType, commastr string) {
	for _, col := range strings.Split(commastr, ",") {
		if col != "" {
			t.PretypedHeaders[col] = typ
		}
	}
}

func (t *TemplateInfo) isPredefined(header string) (MyType, bool) {
	if v, ok := t.PretypedHeaders[header]; ok {
		return v, ok
	}
	return Undefined, false
}

var re = regexp.MustCompile(`\W`)

// convert the give header name to a name which should be compatible with Go.
func toGoName(colName string) string {
	caser := cases.Title(language.English)
	titles := caser.String(colName)
	goName := re.ReplaceAllString(titles, "")
	return goName
}

const Usage = `Usage of excel2go:
excel2go [flags] -package P -type T file
Reads the given excel file (*.xlsx) with header and generates from it a type T struct where all the columns correspond to fields.
Tries to guess the type by looking at the first line. But you can help it with the relative flags.
Typically use in your code with '//go generate excel2go -package MyPackage -type MyType MyExcel.xlsx'
Copyright M. Greiner 2023
Flags:
`

func main() {
	helper := func(typ string) string {
		return "list of comma separated header of the columns containing " + typ
	}

	pkgName := flag.String("package", "MyPackage", "name of the package")
	typName := flag.String("type", "MyType", "name of the type struct")
	dateHeaders := flag.String("dates", "", "header names of columns containing dates")
	intHeaders := flag.String("integers", "", helper("integers"))
	floatHeaders := flag.String("floats", "", helper("floats"))
	boolHeaders := flag.String("booleans", "", helper("booleans"))
	stringHeaders := flag.String("strings", "", helper("strings"))
	colmap := flag.Bool("cols", false, "generate a map between column names and column number")
	outfn := flag.String("out", "stdout", "output file name or stdout")
	tracing := flag.Bool("tracing", true, "when reading a row, log warnings if types don't match")

	flag.Usage = func() {
		w := flag.CommandLine.Output()
		fmt.Fprint(w, Usage)
		flag.PrintDefaults()
	}
	var err error

	flag.Parse()
	if len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(0)
	}

	info := TemplateInfo{
		FileName:        flag.Arg(0),
		Date:            time.Now().Format(time.RFC3339),
		PackageName:     *pkgName,
		TypeName:        *typName,
		PretypedHeaders: map[string]MyType{},
		ColMap:          *colmap,
		Tracing:         *tracing,
	}

	info.setPretyped(Integer, *intHeaders)
	info.setPretyped(Date, *dateHeaders)
	info.setPretyped(Float, *floatHeaders)
	info.setPretyped(Boolean, *boolHeaders)
	info.setPretyped(String, *stringHeaders)

	log.Println("reading from", info.FileName)
	infile, err := os.Open(info.FileName)
	if err != nil {
		panic(err)
	}

	f, err := excelize.OpenReader(infile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	info.Headers = treatHeader(f)
	guessTypes(f, &info)

	generateCode(*outfn, info)

}

// read the header row of the excel file.
// each cell of this firt row is the header. Create a header entry for it.
// The type is still undefined.
func treatHeader(f *excelize.File) []Header {
	sheet := f.GetSheetName(0)
	rows, _ := f.Rows(sheet)

	// get first row with headers
	rows.Next()
	row, _ := rows.Columns()

	headers := make([]Header, 0)
	for c, cell := range row {
		adr, _ := excelize.CoordinatesToCellName(c+1, 1)
		headers = append(headers, Header{
			ColName: cell,
			GoName:  toGoName(cell),
			GoConv:  "",
			Addr:    adr,
			GoType:  Undefined,
		})
	}
	return headers
}

// read the second row (the first one with data) of the excel file.
// try to detect the data type by:
// - checking if the header name was given as a fliag
// - looking at the data row and trying to parse the data.
func guessTypes(f *excelize.File, info *TemplateInfo) {
	sheet := f.GetSheetName(0)
	rows, _ := f.Rows(sheet)

	// skip header row
	rows.Next()

	// get second row, first one with data
	rows.Next()
	row, _ := rows.Columns()

	for h := range info.Headers {
		cell := row[h]
		header := &info.Headers[h]

		// first check, if this is a predefined header
		if typ, ok := info.isPredefined(header.ColName); ok {
			header.setType(typ, info.Tracing)
		} else {
			// now try to determine the type by looking at the data of the first row
			switch {
			case str2.IsInt(cell):
				header.setType(Integer, info.Tracing)
			case str2.IsFloat(cell):
				header.setType(Float, info.Tracing)
			case str2.IsBool(cell):
				header.setType(Boolean, info.Tracing)
			case str2.IsTime(cell):
				header.setType(Date, info.Tracing)
			default:
				header.setType(String, info.Tracing)
			}
		}
	}
}

// define the code as a template which is filled in.
// the fields correspond to the TemplateInfo struct.
var templText = `package {{.PackageName}}

/*
Automatically generated from file {{.FileName}} on {{.Date}}
*/

import (
	"time"
	"github.com/mmgreiner/go-utils/str2"
)

type {{.TypeName}} struct {
	{{ range .Headers}}
	{{- .GoName}} {{.GoType}}   ` + "`" + `csv:"{{.ColName}}"` + "`" + `
	{{ end }}
}

{{ if .ColMap }}
var {{.TypeName}}_colMap = map[string]int{
	{{- range $i, $f := .Headers }}
	"{{ $f.ColName}}": {{$i -}},
	{{- end }}
}
{{ end }}

func {{.TypeName}}_fromRow(row []string) {{.TypeName}} {
	rec := {{.TypeName}}{
	{{ range $i, $f := .Headers }}
		{{$f.GoName}}: {{$f.GoConv}}row[{{$i}}] {{- if $f.GoConv -}}, "{{$f.ColName}}"){{end}},		// {{$f.ColName -}}
	{{ end }}
	}
	return rec
}

`

// generate the code by applying the template
func generateCode(outfn string, info TemplateInfo) {
	// now create a golang struct from it

	tmpl, err := template.New("mytemp").Parse(templText)
	if err != nil {
		panic(err)
	}

	out := os.Stdout
	if outfn != "stdout" {
		out, err = os.Create(outfn)
		if err != nil {
			panic(err)
		}
	}
	defer out.Close()

	log.Println("writing to", outfn)
	err = tmpl.Execute(out, info)
	if err != nil {
		panic(err)
	}
}
