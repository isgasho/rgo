// Copyright ©2019 The rgonomic Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import (
	"fmt"
	"go/types"
	"path"
	"strings"
	"text/template"
	"unicode"

	"github.com/rgonomic/rgo/internal/pkg"
)

// TODO(kortchak): Check input types for validity before making .Call.

// rCall is the template for R .Call function file generation.
func RCallTemplate(words []string) *template.Template {
	return template.Must(template.New("R .Call").Funcs(template.FuncMap{
		"base":      path.Base,
		"snake":     snake(words),
		"varsOf":    varsOf,
		"names":     names,
		"doc":       doc,
		"typecheck": typeCheck,
		"returns":   returns,
		"seelso":    seelso,
		"replace":   strings.ReplaceAll,
	}).Parse(`{{$pkg := .Pkg}}# Code generated by rgnonomic/rgo; DO NOT EDIT.

#' @useDynLib {{base $pkg.Path}}{{range $func := .Funcs}}
{{$params := varsOf $func.Signature.Params}}
#' {{$func.Func.Name}}
#'
#' {{replace $func.FuncDecl.Doc.Text "\n" "\n#' "}}
{{range $p := $params}}{{doc $p}}
{{end}}{{returns $func.Signature.Results}}{{seelso $pkg $func.Func}}
#' @export
{{snake $func.Func.Name}} <- function({{names false $params}}) {
	{{range $p := $params}}{{typecheck $p}}
	{{end}}.Call("{{snake $func.Func.Name}}"{{names true $params}}, PACKAGE = "{{base $pkg.Path}}")
}{{end}}
`))
}

// doc returns an R documentation line for the variable v.
func doc(v *types.Var) string {
	return fmt.Sprintf("#' @param %s is a %s", v.Name(), rDocFor(v.Type()))
}

// seealso returns an @seealso documentation line linking to the fn's
// godoc.org documentation.
func seelso(pkg *types.Package, fn *types.Func) string {
	return fmt.Sprintf("#' @seelso <https://godoc.org/%s#%s>", pkg.Path(), fn.Name())
}

// returns returns an R documentation table for the returned values in t.
func returns(t *types.Tuple) string {
	if t.Len() == 0 {
		return ""
	}
	var buf strings.Builder
	switch t.Len() {
	case 0:
	case 1:
		v := t.At(0)
		doc := rDocFor(v.Type())
		name := v.Name()
		if name != "" {
			name = ", " + name
		}
		fmt.Fprintf(&buf, "#' @return %s%s\n", article(doc, true), name)
	default:
		fmt.Fprintf(&buf, "#' @return A structured value containing:\n")
		for i := 0; i < t.Len(); i++ {
			v := t.At(i)
			doc := rDocFor(v.Type())
			name := v.Name()
			if name == "" {
				name = fmt.Sprintf("r%d", i)
			}
			fmt.Fprintf(&buf, "#' @return - %s, $%s\n", article(doc, false), name)
		}
	}
	return buf.String()
}

// rDocFor returns a string describing the R type based on the given Go type.
func rDocFor(typ types.Type) string {
	rtyp, length := rTypeOf(typ)
	switch typ := typ.Underlying().(type) {
	case *types.Pointer:
		return rDocFor(typ.Elem())
	case *types.Struct:
		return fmt.Sprintf("%s corresponding to %s", rtyp, typ)
	default:
		if rtyp == "list" {
			return rtyp
		}
		switch {
		case length <= 0:
			return fmt.Sprintf("%s vector", rtyp)
		case length == 1:
			return fmt.Sprintf("%s value", rtyp)
		default:
			return fmt.Sprintf("%s vector with %d elements", rtyp, length)
		}
	}
}

// article returns a correct article for a given noun.
func article(noun string, capital bool) string {
	vowel := "a"
	if capital {
		vowel = "A"
	}
	switch unicode.ToLower(rune(noun[0])) {
	case 'a', 'e', 'i', 'o', 'u':
		return vowel + "n " + noun
	default:
		return vowel + " " + noun
	}
}

func typeCheck(p *types.Var) string {
	rtyp, length := rTypeOf(p.Type())
	check := fmt.Sprintf(`if (!is.%[1]s(%[2]s)) {
		stop("Argument '%[2]s' must be of type '%[1]s'.")
	}`, rtyp, p.Name())
	if length > 0 {
		var plural string
		if length != 1 {
			plural = "s"
		}
		check += fmt.Sprintf(`
	if (length(%[1]s) != %[2]d) {
		stop("Argument '%[1]s' must have %d element%s.")
	}`, p.Name(), length, plural)
	}
	return check
}

func rTypeOf(typ types.Type) (rtyp string, length int64) {
	if pkg.IsError(typ) {
		return "character", -1
	}
	switch typ := typ.Underlying().(type) {
	case *types.Pointer:
		return rTypeOf(typ.Elem())
	case *types.Basic:
		return basicRtype(typ), 1
	case *types.Slice:
		elem := typ.Elem()
		if etyp, ok := elem.(*types.Basic); ok {
			if etyp.Kind() == types.Uint8 {
				return "raw", -1
			}
			return basicRtype(etyp), -1
		}
	case *types.Array:
		elem := typ.Elem()
		if etyp, ok := elem.(*types.Basic); ok {
			if etyp.Kind() == types.Uint8 {
				return "raw", typ.Len()
			}
			return basicRtype(etyp), typ.Len()
		}
	case *types.Map, *types.Struct:
		return "list", -1
	}
	return "", -1
}

func basicRtype(typ *types.Basic) string {
	switch info := typ.Info(); {
	case info&types.IsBoolean != 0:
		return "logical"
	case info&types.IsString != 0:
		return "character"
	case info&types.IsInteger != 0:
		return "integer"
	case info&types.IsFloat != 0:
		return "double"
	case info&types.IsComplex != 0:
		return "complex"
	default:
		panic(fmt.Sprintf("unhandled type: %s", typ))
	}
}
