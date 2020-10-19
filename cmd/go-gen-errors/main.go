package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
	"github.com/spf13/pflag"
	yaml "gopkg.in/yaml.v2"
)

var (
	tpl = template.Must(template.New("").Parse(`package {{ .package }} 
import (
	"errors"
	"fmt"

	"google.golang.org/grpc/status"
)

var  (
	_ fmt.Stringer
	_  = errors.New
)
`))

	tplError = template.Must(template.New("").Funcs(template.FuncMap{
		"capital": strcase.ToCamel,
	}).Parse(`
var _err{{ .name }} error = &err{{ .name }}{}

type err{{ .name }} struct {
	reason string
}

func {{ .name }}({{ .inArgs }}) error {
	return &err{{ .name }}{ {{ .newArgs }} }
}

func (e err{{ .name }}) Error() string {
	return fmt.Sprintf("{{ .format }}", {{ .errArgs }})
}

{{- range $k, $v := $.args }}
func (e err{{ $.name }}) {{ $k | capital }}() {{ $v }} {
	return e.{{ $k }}
}
{{- end }}

{{- if .isCode }}
func (e err{{ $.name }}) IsCodeError() {}

func (e err{{ $.name }}) Code() int {
	return {{ .code }}
}
{{- end }}

{{- if .isCode }}
func (e err{{ $.name }}) GRPCStatus() *status.Status {
	return status.New({{ .code }}, e.Error())	
}
{{- end }}

func Is{{ .name }}(err error) bool {
	return errors.Is(err, _err{{ .name }})
}
`))

	pIn      = pflag.StringP("in", "i", "", "input config")
	pOut     = pflag.StringP("out", "o", "", "output file")
	pPackage = pflag.StringP("package", "p", "", "package name")
)

type config struct {
	Name   string            `yaml:"name"`
	Format string            `yaml:"format"`
	Args   map[string]string `yaml:"args"`
	Code   int               `yaml:"code"`
}

func main() {
	pflag.Parse()

	b, err := ioutil.ReadFile(*pIn)
	die(err)

	var c []config
	die(yaml.Unmarshal(b, &c))

	buf := bytes.NewBuffer(nil)
	die(tpl.Execute(buf, map[string]interface{}{
		"package": *pPackage,
	}))

	for _, e := range c {
		name := e.Name
		var (
			inArgs  []string
			newArgs []string
			errArgs []string
		)

		for a, t := range e.Args {
			inArgs = append(inArgs, fmt.Sprintf("%s %s", a, t))
			newArgs = append(newArgs, fmt.Sprintf("%s: %s", a, a))
			errArgs = append(errArgs, fmt.Sprintf("e.%s", a))
		}

		die(tplError.Execute(buf, map[string]interface{}{
			"name":    name,
			"args":    e.Args,
			"format":  e.Format,
			"isCode":  e.Code > 0,
			"code":    e.Code,
			"inArgs":  strings.Join(inArgs, ", "),
			"newArgs": strings.Join(newArgs, ", "),
			"errArgs": strings.Join(errArgs, ", "),
		}))
	}

	b, err = format.Source(buf.Bytes())
	if err != nil {
		println(buf.String())
		die(err)
	}

	dir := filepath.Dir(*pOut)
	die(os.MkdirAll(dir, 0755))
	die(ioutil.WriteFile(*pOut, b, 0644))
}

func die(err error) {
	if err != nil {
		panic(err)
	}
}
