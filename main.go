package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"regexp"
	"time"

	"path/filepath"

	"text/template"

	"github.com/mmgreiner/go-utils/str2"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/xuri/excelize/v2"
)

type MyType int

const (
	Integer = "int"
	Float   = "float64"
	Date    = "time.Time"
	String  = "string"
	Boolean = "bool"
)

type Header struct {
	CsvName string
	GoName  string
	GoType  string
	GoConv  string
	Addr    string
}

type TemplateInfo struct {
	FileName    string
	Date        string
	PackageName string
	TypeName    string
	Fields      []Header
}

var re = regexp.MustCompile(`\W`)

func ToGoName(csvName string) string {
	caser := cases.Title(language.English)
	titles := caser.String(csvName)
	goName := re.ReplaceAllString(titles, "")
	return goName
}

func main() {
	info := TemplateInfo{
		Date: time.Now().Format(time.RFC3339),
	}

	flag.StringVar(&info.PackageName, "package", "MyPackage", "name of the package")
	flag.StringVar(&info.TypeName, "type", "MyType", "name of the type struct")
	outfn := flag.String("out", "stdout", "output file name or stdout")
	/*
		flNumeric := flag.String("Numberic", "No,Nr,Amount", "List of header names indicating numbers")
		flDate := flag.String("Date", "Date,Created,Modified", "list of header names indicating a date")
		flBool := flag.Bool("Bool", "", "list of header names indicating boolean")
	*/

	flag.Usage = func() {
		w := flag.CommandLine.Output()
		_, fn := filepath.Split(os.Args[0])
		fmt.Fprintln(w, "Purpose: Read the first sheet of the given excel xlsx file which has aheader row and then data, and converts it to a golang type structure.")
		fmt.Fprintf(w, "Usage: %s [-options] excel-file\n", fn)
		fmt.Fprintln(w, "Options:")
		flag.PrintDefaults()
	}
	var err error

	flag.Parse()
	if len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(0)
	}

	info.FileName = flag.Arg(0)
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

	/*
		numeric := regexp.MustCompile(`No|Nr|Amount`)
		date := regexp.MustCompile("Date|Created|Modified")
	*/

	info.Fields = treatHeader(f)

	guessTypes(f, info.Fields)

	log.Println("generating code to ", outfn)
	generateCode(*outfn, info)

}

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
			CsvName: cell,
			GoName:  ToGoName(cell),
			GoConv:  "",
			Addr:    adr,
		})
	}
	return headers
}

func guessTypes(f *excelize.File, headers []Header) {
	sheet := f.GetSheetName(0)
	rows, _ := f.Rows(sheet)

	// skip header row
	rows.Next()
	// get second row, first one with data
	rows.Next()
	row, _ := rows.Columns()
	for h := range headers {
		cell := row[h]
		switch {
		case str2.IsInt(cell):
			headers[h].GoType = Integer
			headers[h].GoConv = "str2.TraceToInt("
		case str2.IsFloat(cell):
			headers[h].GoType, headers[h].GoConv = Float, "str2.TraceToFloat("
		case str2.IsBool(cell):
			headers[h].GoType = Boolean
			headers[h].GoConv = "str2.TraceToBool("
		case str2.IsTime(cell):
			headers[h].GoType = Date
			headers[h].GoConv = "str2.TraceToTime("
		default:
			headers[h].GoType = String
			headers[h].GoConv = ""
		}
	}
}

var templText = `package {{.PackageName}}

/*
Automatically generated from file {{.FileName}} on {{.Date}}
*/

import (
	"os"
	"time"
	"mmgreiner/str2"
	"github.com/gocarina/gocsv"
)

type {{.TypeName}} struct {
	{{ range .Fields}}
	{{- .GoName}} {{.GoType}}   ` + "`" + `csv:"{{.CsvName}}"` + "`" + `
	{{ end }}
}

var colMap = map[string]int{
	{{- range $i, $f := .Fields }}
	"{{ $f.CsvName}}": {{$i -}},
	{{- end }}
}

func FromRow(row []string) {{.TypeName}} {
	rec := {{.TypeName}}{
	{{ range $i, $f := .Fields }}
		{{$f.GoName}}: {{$f.GoConv}}row[{{$i}}] {{- if $f.GoConv -}}, "{{$f.CsvName}}"){{end}},		// {{$f.CsvName -}}
		{{ else }}

	{{ end }}
	}
	return rec
}

func ReadCsv(fn string) []{{.TypeName}} {
	file, err := os.Open(fn)
	if err != nil { panic(err) }
	defer file.Close()

	records := []*{{.TypeName}}{}
	if err := gocsv.UnmarshalFile(file, &records); err != nil { 
		panic(err)
	}
	return records
}
`

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

	log.Println("writing to ", outfn)
	err = tmpl.Execute(out, info)
	if err != nil {
		panic(err)
	}
}
