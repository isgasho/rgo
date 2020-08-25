// Copyright ©2019 The rgonomic Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import (
	"bytes"
	"fmt"
	"go/types"
	"path"
	"reflect"
	"strings"
	"text/template"

	"github.com/rgonomic/rgo/internal/pkg"
)

// goFunc is the template for Go function file generation.
func GoFuncTemplate() *template.Template {
	return template.Must(template.New("Go func").Funcs(template.FuncMap{
		"varsOf":     varsOf,
		"go":         goParams,
		"anon":       anonymous,
		"types":      typeNames,
		"mangle":     pkg.Mangle,
		"unpackSEXP": unpackSEXPFuncGo,
		"packSEXP":   packSEXPFuncGo,
		"dec":        func(i int) int { return i - 1 },
	}).Parse(`{{$pkg := .Pkg}}// Code generated by rgnonomic/rgo; DO NOT EDIT.

package main

/*
#define USE_RINTERNALS
#include <R.h>
#include <Rinternals.h>
extern void R_error(char *s);

// TODO(kortschak): Only emit these when needed.
extern Rboolean Rf_isNull(SEXP s);
extern _GoString_ R_gostring(SEXP x, R_xlen_t i);
extern int getListElementIndex(SEXP list, const char *str);
*/
import "C"

import (
	"fmt"
	"unsafe"

	"{{$pkg.Path}}"
)
{{$resultNeedsList := false}}
{{range $func := .Funcs}}{{$params := varsOf $func.Signature.Params}}{{$results := varsOf $func.Signature.Results}}
//export Wrapped_{{$func.Name}}
func Wrapped_{{$func.Name}}({{go "_R_" $params}}) C.SEXP {
	defer func() {
		r := recover()
		if r != nil {
			err := C.CString(fmt.Sprint(r))
			C.R_error(err)
			C.free(unsafe.Pointer(err))
		}
	}()

	{{range $i, $p := $params}}_p{{$i}} := unpackSEXP{{mangle $p.Type}}(_R_{{$p.Name}})
	{{end}}{{with $results}}{{anon . "_r" false}} := {{end}}{{$pkg.Name}}.{{$func.Name}}({{anon $params "_p" false}}{{if $func.Signature.Variadic}}...{{end}})
	{{with $results}}return packSEXP_{{$func.Name}}({{anon . "_r" false}}){{else}}return C.R_NilValue{{end}}
}

{{if $results}}func packSEXP_{{$func.Name}}({{anon $results "p" true}}) C.SEXP {
{{$l := len $results -}}
{{- if eq $l 1 -}}
{{- $p := index $results 0}}	return packSEXP{{mangle $p.Type}}({{if $p.Name}}{{$p.Name}}{{else}}p0{{end -}})
{{- else}}{{$resultNeedsList = true}}	r := C.allocList({{len $results}})
	C.Rf_protect(r)
	names := C.Rf_allocVector(C.STRSXP, {{len $results}})
	C.Rf_protect(names)
	arg := r
{{range $i, $p := $results}}{{$res := printf "r%d" $i}}{{if $p.Name}}{{$res = $p.Name}}{{end}}	C.SET_STRING_ELT(names, {{$i}}, C.Rf_mkCharLenCE(C._GoStringPtr("{{$res}}"), {{len $res}}, C.CE_UTF8))
	C.SETCAR(arg, packSEXP{{mangle $p.Type}}({{if $p.Name}}{{$p.Name}}{{else}}p{{$i}}{{end}}))
{{if lt $i (dec $l)}}	arg = C.CDR(arg)
{{end -}}
{{- end}}	C.setAttrib(r, packSEXP_types_Basic_string("names"), names)
	C.Rf_unprotect(2)
	return r{{end}}
}
{{end}}{{end}}
{{/* TODO(kortschak): Hoist C.SEXP unpacking for basic types out to the C code. */ -}}
{{- .Unpackers.Types | unpackSEXP -}}
{{- .Packers.Types | packSEXP}}func main() {}
`))
}

// goParams returns a comma-separated list of C.SEXP parameters using the
// parameter names in vars with the mangling prefix applied.
func goParams(prefix string, vars []*types.Var) string {
	if len(vars) == 0 {
		return ""
	}
	var buf strings.Builder
	for i, v := range vars {
		if i != 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(prefix)
		buf.WriteString(v.Name())
	}
	buf.WriteString(" C.SEXP")
	return buf.String()
}

// anonymous returns a comma-separated list of numbered parameters corresponding
// to vars with the given prefix. If typed is true, the parameter type is included.
func anonymous(vars []*types.Var, prefix string, typed bool) string {
	if len(vars) == 0 {
		return ""
	}
	var buf strings.Builder
	for i, v := range vars {
		if i != 0 {
			buf.WriteString(", ")
		}
		if !typed {
			buf.WriteString(fmt.Sprintf("%s%d", prefix, i))
			continue
		}
		name := v.Name()
		if name == "" {
			name = fmt.Sprintf("%s%d", prefix, i)
		}
		buf.WriteString(fmt.Sprintf("%s %s", name, path.Base(v.Type().String())))
	}
	return buf.String()
}

// typeNames returns a comma-separated list of the type names corresponding to vars.
func typeNames(vars []*types.Var) string {
	if len(vars) == 0 {
		return ""
	}
	var buf strings.Builder
	for i, v := range vars {
		if i != 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(nameOf(v.Type()))
	}
	return buf.String()
}

// unpackSEXPFuncGo returns the source of functions to unpack R SEXP parameters
// into the given Go types.
func unpackSEXPFuncGo(typs []types.Type) string {
	var buf bytes.Buffer
	for _, typ := range typs {
		fmt.Fprintf(&buf, "func unpackSEXP%s(p C.SEXP) %s {\n", pkg.Mangle(typ), nameOf(typ))
		unpackSEXPFuncBodyGo(&buf, typ)
		buf.WriteString("}\n\n")
	}
	return buf.String()
}

// unpackSEXPFuncBodyGo returns the body of a function to unpack R SEXP parameters
// into the given Go types.
func unpackSEXPFuncBodyGo(buf *bytes.Buffer, typ types.Type) {
	switch typ := typ.(type) {
	case *types.Named:
		fmt.Fprintf(buf, "\treturn %s(unpackSEXP%s(p))\n", nameOf(typ), pkg.Mangle(typ.Underlying()))

	case *types.Array:
		// TODO(kortschak): Only do this for [n]int32, [n]float64, [n]complex128 and [n]byte.
		// Otherwise we have a double copy.
		fmt.Fprintf(buf, `	var a %s
	copy(a[:], unpackSEXP%s(p))
	return a
`, typ, pkg.Mangle(types.NewSlice(typ.Elem())))

	case *types.Basic:
		switch typ.Kind() {
		case types.Bool:
			fmt.Fprintln(buf, "\treturn *C.RAW(p) == 1")
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64, types.Uint, types.Uint16, types.Uint32, types.Uint64:
			fmt.Fprintf(buf, "\treturn %s(*C.INTEGER(p))\n", nameOf(typ))
		case types.Uint8:
			fmt.Fprintf(buf, "\treturn %s(*C.RAW(p))\n", nameOf(typ))
		case types.Float64, types.Float32:
			fmt.Fprintf(buf, "\treturn %s(*C.REAL(p))\n", nameOf(typ))
		case types.Complex128:
			fmt.Fprintf(buf, "\treturn %s(*(*complex128)(unsafe.Pointer(C.COMPLEX(p))))\n", nameOf(typ))
		case types.Complex64:
			fmt.Fprintf(buf, "\treturn %s(unpackSEXP%s(p))\n", nameOf(typ), pkg.Mangle(types.Typ[types.Complex128]))
		case types.String:
			fmt.Fprintln(buf, "\treturn C.R_gostring(p, 0)")
		default:
			panic(fmt.Sprintf("unhandled type: %s", typ))
		}

	case *types.Map:
		fmt.Fprintf(buf, `	if C.Rf_isNull(p) != 0 {
		return nil
	}
`)
		elem := typ.Elem()
		if basic, ok := elem.Underlying().(*types.Basic); ok {
			switch basic.Kind() {
			// TODO(kortschak): Make the fast path available
			// to []T where T is one of these kinds.
			case types.Int, types.Int8, types.Int16, types.Int32, types.Uint, types.Uint16, types.Uint32:
				// Maximum length array type for this element type.
				type a [1 << 47]int32
				fmt.Fprintf(buf, `	n := int(C.Rf_xlength(p))
	r := make(map[string]%[2]s, n)
	names := C.getAttrib(p, C.R_NamesSymbol)
	values := (*[%[1]d]int32)(unsafe.Pointer(C.INTEGER(p)))[:n:n]
	for i, elem := range values {
		key := string(C.R_gostring(names, C.R_xlen_t(i)))
		r[key] = %[2]s(elem)
	}
	return r
`, len(&a{}), nameOf(elem))
				return
			case types.Uint8:
				// Maximum length array type for this element type.
				type a [1 << 49]byte
				fmt.Fprintf(buf, `	n := int(C.Rf_xlength(p))
	r := make(map[string]%[2]s, n)
	names := C.getAttrib(p, C.R_NamesSymbol)
	values := (*[%[1]d]%[2]s)(unsafe.Pointer(C.RAW(p)))[:n:n]
	for i, elem := range values {
		key := string(C.R_gostring(names, C.R_xlen_t(i)))
		r[key] = elem
	}
	return r
`, len(&a{}), nameOf(elem))
				return
			case types.Float32, types.Float64:
				// Maximum length array type for this element type.
				type a [1 << 46]float64
				fmt.Fprintf(buf, `	n := int(C.Rf_xlength(p))
	r := make(map[string]%[2]s, n)
	names := C.getAttrib(p, C.R_NamesSymbol)
	values := (*[%[1]d]float64)(unsafe.Pointer(C.REAL(p)))[:n:n]
	for i, elem := range values {
		key := string(C.R_gostring(names, C.R_xlen_t(i)))
		r[key] = %[2]s(elem)
	}
	return r
`, len(&a{}), nameOf(elem))
				return
			case types.Complex64, types.Complex128:
				// Maximum length array type for this element type.
				type a [1 << 45]complex128
				fmt.Fprintf(buf, `	n := int(C.Rf_xlength(p))
	r := make(map[string]%[2]s, n)
	names := C.getAttrib(p, C.R_NamesSymbol)
	values := (*[%[1]d]complex128)(unsafe.Pointer(C.COMPLEX(p)))[:n:n]
	for i, elem := range values {
		key := string(C.R_gostring(names, C.R_xlen_t(i)))
		r[key] = %[2]s(elem)
	}
	return r
`, len(&a{}), nameOf(elem))
				return
			case types.Bool:
				// Maximum length array type for this element type.
				type a [1 << 47]int32
				fmt.Fprintf(buf, `	n := int(C.Rf_xlength(p))
	r := make(map[string]%[2]s, n)
	names := C.getAttrib(p, C.R_NamesSymbol)
	values := (*[%[1]d]int32)(unsafe.Pointer(C.LOGICAL(p)))[:n:n]
	for i, elem := range values {
		key := string(C.R_gostring(names, C.R_xlen_t(i)))
		r[key] = (elem == 1)
	}
	return r
`, len(&a{}), nameOf(elem))
				return
			case types.String:
				fmt.Fprintf(buf, `	n := int(C.Rf_xlength(p))
	r := make(map[string]%[1]s, n)
	names := C.getAttrib(p, C.R_NamesSymbol)
	for i := 0; i < n; i++ {
		key := string(C.R_gostring(names, C.R_xlen_t(i)))
		r[key] = %[1]s(C.R_gostring(p, C.R_xlen_t(i)))
	}
	return r
`, nameOf(elem))
				return
			}
		}
		panic(fmt.Sprintf("TODO: unpack map[string]%s", elem))

	case *types.Pointer:
		fmt.Fprintf(buf, `	if C.Rf_isNull(p) != 0 {
		return nil
	}
	r := unpackSEXP%s(p)
	return &r
`, pkg.Mangle(typ.Elem()))

	case *types.Slice:
		// TODO(kortschak): Use unsafe.Slice when it exists.

		fmt.Fprintf(buf, `	if C.Rf_isNull(p) != 0 {
		return nil
	}
`)
		elem := typ.Elem()
		if elem, ok := elem.(*types.Basic); ok {
			switch elem.Kind() {
			// TODO(kortschak): Make the fast path available
			// to []T where T is one of these kinds.
			case types.Int32:
				// Maximum length array type for this element type.
				type a [1 << 47]int32
				fmt.Fprintf(buf, `	n := C.Rf_xlength(p)
	return (*[%d]%s)(unsafe.Pointer(C.INTEGER(p)))[:n:n]
`, len(&a{}), nameOf(elem))
				return
			case types.Uint8:
				// Maximum length array type for this element type.
				type a [1 << 49]byte
				fmt.Fprintf(buf, `	n := C.Rf_xlength(p)
	return (*[%d]%s)(unsafe.Pointer(C.RAW(p)))[:n:n]
`, len(&a{}), nameOf(elem))
				return
			case types.Float64:
				// Maximum length array type for this element type.
				type a [1 << 46]float64
				fmt.Fprintf(buf, `	n := C.Rf_xlength(p)
	return (*[%d]%s)(unsafe.Pointer(C.REAL(p)))[:n:n]
`, len(&a{}), nameOf(elem))
				return
			case types.Complex128:
				// Maximum length array type for this element type.
				type a [1 << 45]complex128
				fmt.Fprintf(buf, `	n := C.Rf_xlength(p)
	return (*[%d]%s)(unsafe.Pointer(C.COMPLEX(p)))[:n:n]
`, len(&a{}), nameOf(elem))
				return
			case types.Bool:
				// Maximum length array type for this element type.
				type a [1 << 47]int32
				fmt.Fprintf(buf, `	n := C.Rf_xlength(p)
	r := make(%s, n)
	for i, b := range (*[%d]%s)(unsafe.Pointer(C.BOOL(p)))[:n] {
		r[i] = (b == 1)
	}
	return r
`, nameOf(typ), len(&a{}), nameOf(types.Typ[types.Int32]))
				return
			case types.String:
				fmt.Fprintf(buf, `	n := C.Rf_xlength(p)
	r := make(%s, n)
	for i := range r {
		r[i] = %s(C.R_gostring(p, C.R_xlen_t(i)))
	}
	return r
`, nameOf(typ), nameOf(elem))
				return
			}
		}
		fmt.Fprintf(buf, `	n := C.Rf_xlength(p)
	r := make(%s, n)
	for i := range r {
		r[i] = unpackSEXP%s(C.VECTOR_ELT(p, C.R_xlen_t(i)))
	}
	return r
`, nameOf(typ), pkg.Mangle(elem))

	case *types.Struct:
		n := typ.NumFields()
		fmt.Fprintf(buf, `	switch n := C.Rf_xlength(p); {
	case n < %[1]d:
		panic(`+"`missing list element for %[2]s`"+`)
	case n > %[1]d:
		err := C.CString(`+"`extra list element ignored for %[2]s`"+`)
		C.R_error(err)
		C.free(unsafe.Pointer(err))
	}
	var r %[2]s
	var i C.int
`, n, nameOf(typ))
		for i := 0; i < n; i++ {
			f := typ.Field(i)

			fmt.Fprintf(buf, `	key_%s := C.CString("%[1]s")
	defer C.free(unsafe.Pointer(key_%[1]s))
	i = C.getListElementIndex(p, key_%[1]s)
	if i < 0 {
		panic("no list element for field: %[2]s")
	}
	r.%[2]s = unpackSEXP%s(C.VECTOR_ELT(p, C.R_xlen_t(i)))
`, targetFieldName(typ, i), f.Name(), pkg.Mangle(f.Type()))
		}
		fmt.Fprintln(buf, "\treturn r")

	default:
		panic(fmt.Sprintf("unhandled type: %s", typ))
	}
}

// packSEXPFuncGo returns the source of functions to pack the given Go-typed
// parameters into R SEXP values.
func packSEXPFuncGo(typs []types.Type) string {
	var buf bytes.Buffer
	for _, typ := range typs {
		fmt.Fprintf(&buf, "func packSEXP%s(p %s) C.SEXP {\n", pkg.Mangle(typ), nameOf(typ))
		packSEXPFuncBodyGo(&buf, typ)
		buf.WriteString("}\n\n")
	}
	return buf.String()
}

// packSEXPFuncGo returns the body of a function to pack the given Go-typed
// parameters into R SEXP values.
func packSEXPFuncBodyGo(buf *bytes.Buffer, typ types.Type) {
	switch typ := typ.(type) {
	case *types.Named:
		if pkg.IsError(typ) {
			fmt.Fprintf(buf, `	if p == nil {
		return C.R_NilValue
	}
	return packSEXP%s(p.Error())
`, pkg.Mangle(types.Typ[types.String]))
		} else {
			switch typ := typ.Underlying().(type) {
			case *types.Pointer:
				fmt.Fprintf(buf, "\treturn packSEXP%s((%s)(p))\n", pkg.Mangle(typ), typ)
			default:
				fmt.Fprintf(buf, "\treturn packSEXP%s(%s(p))\n", pkg.Mangle(typ), typ)
			}
		}

	case *types.Array:
		fmt.Fprintf(buf, "\treturn packSEXP%s(p[:])\n", pkg.Mangle(types.NewSlice(typ.Elem())))

	case *types.Basic:
		switch typ.Kind() {
		case types.Bool:
			fmt.Fprintf(buf, `	b := C.int(0)
	if p {
		b = 1
	}
	return C.ScalarLogical(b)
`)
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64, types.Uint, types.Uint16, types.Uint32, types.Uint64:
			fmt.Fprintln(buf, "\treturn C.ScalarInteger(C.int(p))")
		case types.Uint8:
			fmt.Fprintln(buf, "\treturn C.ScalarRaw(C.Rbyte(p))")
		case types.Float64, types.Float32:
			fmt.Fprintln(buf, "\treturn C.ScalarReal(C.double(p))")
		case types.Complex128, types.Complex64:
			fmt.Fprintln(buf, "\treturn C.ScalarComplex(C.struct_Rcomplex{r: C.double(real(p)), i: C.double(imag(p))})")
		case types.String:
			fmt.Fprintln(buf, `	s := C.Rf_mkCharLenCE(C._GoStringPtr(p), C.int(len(p)), C.CE_UTF8)
	return C.ScalarString(s)`)
		default:
			panic(fmt.Sprintf("unhandled type: %s", typ))
		}

	case *types.Map:
		// TODO(kortschak): Handle named simple types properly.
		elem := typ.Elem()
		if basic, ok := elem.Underlying().(*types.Basic); ok {
			switch basic.Kind() {
			// TODO(kortschak): Make the fast path available
			// to []T where T is one of these kinds.
			case types.Int, types.Int8, types.Int16, types.Int32, types.Uint, types.Uint16, types.Uint32:
				// Maximum length array type for this element type.
				type a [1 << 47]int32
				fmt.Fprintf(buf, `	n := len(p)
	r := C.Rf_allocVector(C.%[1]s, C.R_xlen_t(n))
	C.Rf_protect(r)
	names := C.Rf_allocVector(C.STRSXP, C.R_xlen_t(n))
	C.Rf_protect(names)
	s := (*[%[2]d]int32)(unsafe.Pointer(C.INTEGER(r)))[:len(p):len(p)]
	var i C.R_xlen_t
	for k, v := range p {
		C.SET_STRING_ELT(names, i, C.Rf_mkCharLenCE(C._GoStringPtr(k), C.int(len(k)), C.CE_UTF8))
		s[i] = int32(v)
		i++
	}
	C.setAttrib(r, packSEXP_types_Basic_string("names"), names)
	C.Rf_unprotect(2)
	return r
`, rTypeLabelFor(elem), len(&a{}))
				return

			case types.Uint8:
				// Maximum length array type for this element type.
				type a [1 << 49]byte
				fmt.Fprintf(buf, `	n := len(p)
	r := C.Rf_allocVector(C.%[1]s, C.R_xlen_t(n))
	C.Rf_protect(r)
	names := C.Rf_allocVector(C.STRSXP, C.R_xlen_t(n))
	C.Rf_protect(names)
	s := (*[%[2]d]uint8)(unsafe.Pointer(C.RAW(r)))[:len(p):len(p)]
	var i C.R_xlen_t
	for k, v := range p {
		C.SET_STRING_ELT(names, i, C.Rf_mkCharLenCE(C._GoStringPtr(k), C.int(len(k)), C.CE_UTF8))
		i++
	}
	copy(s, p)
	C.setAttrib(r, packSEXP_types_Basic_string("names"), names)
	C.Rf_unprotect(2)
	return r
`, rTypeLabelFor(elem), len(&a{}), pkg.Mangle(elem))
				return

			case types.Float32, types.Float64:
				// Maximum length array type for this element type.
				type a [1 << 46]float64
				fmt.Fprintf(buf, `	n := len(p)
	r := C.Rf_allocVector(C.%[1]s, C.R_xlen_t(n))
	C.Rf_protect(r)
	names := C.Rf_allocVector(C.STRSXP, C.R_xlen_t(n))
	C.Rf_protect(names)
	s := (*[%[2]d]float64)(unsafe.Pointer(C.REAL(r)))[:len(p):len(p)]
	var i C.R_xlen_t
	for k, v := range p {
		C.SET_STRING_ELT(names, i, C.Rf_mkCharLenCE(C._GoStringPtr(k), C.int(len(k)), C.CE_UTF8))
		s[i] = float64(v)
		i++
	}
	C.setAttrib(r, packSEXP_types_Basic_string("names"), names)
	C.Rf_unprotect(2)
	return r
`, rTypeLabelFor(elem), len(&a{}))
				return

			case types.Complex64, types.Complex128:
				// Maximum length array type for this element type.
				type a [1 << 45]complex128
				fmt.Fprintf(buf, `	n := len(p)
	r := C.Rf_allocVector(C.%[1]s, C.R_xlen_t(n))
	C.Rf_protect(r)
	names := C.Rf_allocVector(C.STRSXP, C.R_xlen_t(n))
	C.Rf_protect(names)
	s := (*[%[2]d]complex128)(unsafe.Pointer(C.COMPLEX(r)))[:len(p):len(p)]
	var i C.R_xlen_t
	for k, v := range p {
		C.SET_STRING_ELT(names, i, C.Rf_mkCharLenCE(C._GoStringPtr(k), C.int(len(k)), C.CE_UTF8))
		s[i] = complex128(v)
		i++
	}
	C.setAttrib(r, packSEXP_types_Basic_string("names"), names)
	C.Rf_unprotect(2)
	return r
`, rTypeLabelFor(elem), len(&a{}))
				return

			case types.String:
				fmt.Fprintf(buf, `	n := len(p)
	r := C.Rf_allocVector(C.%[1]s, C.R_xlen_t(n))
	C.Rf_protect(r)
	names := C.Rf_allocVector(C.STRSXP, C.R_xlen_t(n))
	C.Rf_protect(names)
	var i C.R_xlen_t
	for k, v := range p {
		C.SET_STRING_ELT(names, i, C.Rf_mkCharLenCE(C._GoStringPtr(k), C.int(len(k)), C.CE_UTF8))
		C.SET_STRING_ELT(r, i, packSEXP%s(v))
		i++
	}
	C.setAttrib(r, packSEXP_types_Basic_string("names"), names)
	C.Rf_unprotect(2)
	return r
`, rTypeLabelFor(elem), pkg.Mangle(elem))
				return

			case types.Bool:
				// Maximum length array type for this element type.
				type a [1 << 47]int32
				// FIXME(kortschak): Does Rf_allocVector return a
				// zeroed vector? If it does, the loop below doesn't
				// need the else clause.
				// Alternatively, convert the []bool to a []byte:
				//  for i, v := range *(*[]byte)(unsafe.Pointer(&p)) {
				//      s[i] = int32(v)
				//  }
				fmt.Fprintf(buf, `	n := len(p)
	r := C.Rf_allocVector(C.LGLSXP, C.R_xlen_t(n))
	C.Rf_protect(r)
	names := C.Rf_allocVector(C.STRSXP, C.R_xlen_t(n))
	C.Rf_protect(names)
	s := (*[%d]int32)(unsafe.Pointer(C.LOGICAL(r)))[:len(p):len(p)]
	var i C.R_xlen_t
	for k, v := range p {
		C.SET_STRING_ELT(names, i, C.Rf_mkCharLenCE(C._GoStringPtr(k), C.int(len(k)), C.CE_UTF8))
		if v {
			s[i] = 1
		} else {
			s[i] = 0
		}
		i++
	}
	C.setAttrib(r, packSEXP_types_Basic_string("names"), names)
	C.Rf_unprotect(2)
	return r
`, len(&a{}))
				return
			}
		}

		switch {
		case elem.String() == "error":
			fmt.Fprintf(buf, `	n := len(p)
	r := C.Rf_allocVector(C.%[1]s, C.R_xlen_t(n))
	C.Rf_protect(r)
	names := C.Rf_allocVector(C.STRSXP, C.R_xlen_t(n))
	C.Rf_protect(names)
	var i C.R_xlen_t
	for k, v := range p {
		C.SET_STRING_ELT(names, i, C.Rf_mkCharLenCE(C._GoStringPtr(k), C.int(len(k)), C.CE_UTF8))
		s := C.R_NilValue
		if v != nil {
			C.SET_STRING_ELT(r, i, packSEXP%[2]s(v))
		}
		C.SET_STRING_ELT(r, i, s)
		i++
	}
	C.setAttrib(r, packSEXP_types_Basic_string("names"), names)
	C.Rf_unprotect(2)
	return r
`, rTypeLabelFor(elem), pkg.Mangle(elem))

		default:
			fmt.Fprintf(buf, `	n := len(p)
	r := C.Rf_allocVector(C.%s, C.R_xlen_t(n))
	C.Rf_protect(r)
	names := C.Rf_allocVector(C.STRSXP, C.R_xlen_t(n))
	C.Rf_protect(names)
	var i C.R_xlen_t
	for k, v := range p {
		C.SET_STRING_ELT(names, i, C.Rf_mkCharLenCE(C._GoStringPtr(k), C.int(len(k)), C.CE_UTF8))
		C.SET_VECTOR_ELT(r, i, packSEXP%s(v))
		i++
	}
	C.setAttrib(r, packSEXP_types_Basic_string("names"), names)
	C.Rf_unprotect(2)
	return r
`, rTypeLabelFor(elem), pkg.Mangle(elem))
		}

	case *types.Pointer:
		fmt.Fprintf(buf, `	if p == nil {
		return C.R_NilValue
	}
	return packSEXP%s(*p)
`, pkg.Mangle(typ.Elem()))

	case *types.Slice:
		// TODO(kortschak): Handle named simple types properly.
		elem := typ.Elem()
		if elem, ok := elem.(*types.Basic); ok {
			switch elem.Kind() {
			// TODO(kortschak): Make the fast path available
			// to []T where T is one of these kinds.
			case types.Int32:
				// Maximum length array type for this element type.
				type a [1 << 47]int32
				fmt.Fprintf(buf, `	r := C.Rf_allocVector(C.INTSXP, C.R_xlen_t(len(p)))
	C.Rf_protect(r)
	s := (*[%d]%s)(unsafe.Pointer(C.INTEGER(r)))[:len(p):len(p)]
	copy(s, p)
	C.Rf_unprotect(1)
	return r
`, len(&a{}), nameOf(elem))
				return
			case types.Uint8:
				// Maximum length array type for this element type.
				type a [1 << 49]byte
				fmt.Fprintf(buf, `	r := C.Rf_allocVector(C.RAWSXP, C.R_xlen_t(len(p)))
	C.Rf_protect(r)
	s := (*[%d]%s)(unsafe.Pointer(C.RAW(r)))[:len(p):len(p)]
	copy(s, p)
	C.Rf_unprotect(1)
	return r
`, len(&a{}), nameOf(elem))
				return
			case types.Float64:
				// Maximum length array type for this element type.
				type a [1 << 46]float64
				fmt.Fprintf(buf, `	r := C.Rf_allocVector(C.REALSXP, C.R_xlen_t(len(p)))
	C.Rf_protect(r)
	s := (*[%d]%s)(unsafe.Pointer(C.REAL(r)))[:len(p):len(p)]
	copy(s, p)
	C.Rf_unprotect(1)
	return r
`, len(&a{}), nameOf(elem))
				return
			case types.Complex128:
				// Maximum length array type for this element type.
				type a [1 << 45]complex128
				fmt.Fprintf(buf, `	r := C.Rf_allocVector(C.CPLXSXP, C.R_xlen_t(len(p)))
	C.Rf_protect(r)
	s := (*[%d]%s)(unsafe.Pointer(C.CPLXSXP(r)))[:len(p):len(p)]
	copy(s, p)
	C.Rf_unprotect(1)
	return r
`, len(&a{}), nameOf(elem))
				return
			case types.Bool:
				// Maximum length array type for this element type.
				type a [1 << 47]int32
				// FIXME(kortschak): Does Rf_allocVector return a
				// zeroed vector? If it does, the loop below doesn't
				// need the else clause.
				// Alternatively, convert the []bool to a []byte:
				//  for i, v := range *(*[]byte)(unsafe.Pointer(&p)) {
				//      s[i] = int32(v)
				//  }
				fmt.Fprintf(buf, `	r := C.Rf_allocVector(C.LGLSXP, C.R_xlen_t(len(p)))
	C.Rf_protect(r)
	s := (*[%d]%s)(unsafe.Pointer(C.LOGICAL(r)))[:len(p):len(p)]
	for i, v := range p {
		if v {
			s[i] = 1
		} else {
			s[i] = 0
		}
	}
	C.Rf_unprotect(1)
	return r
`, len(&a{}), nameOf(elem))
				return
			}
		}

		switch {
		case elem.String() == "string":
			fmt.Fprint(buf, `	r := C.Rf_allocVector(C.STRSXP, C.R_xlen_t(len(p)))
	C.Rf_protect(r)
	for i, v := range p {
		s := C.Rf_mkCharLenCE(C._GoStringPtr(string(v)), C.int(len(v)), C.CE_UTF8)
		C.SET_STRING_ELT(r, C.R_xlen_t(i), s)
	}
	C.Rf_unprotect(1)
	return r
`)
		case elem.String() == "error":
			fmt.Fprint(buf, `	r := C.Rf_allocVector(C.STRSXP, C.R_xlen_t(len(p)))
	C.Rf_protect(r)
	for i, v := range p {
		s := C.R_NilValue
		if v != nil {
			s = C.Rf_mkCharLenCE(C._GoStringPtr(v), C.int(len(v)), C.CE_UTF8)
		}
		C.SET_STRING_ELT(r, C.R_xlen_t(i), s)
	}
	C.Rf_unprotect(1)
	return r
`)
		default:
			fmt.Fprintf(buf, `	r := C.Rf_allocVector(C.%s, C.R_xlen_t(len(p)))
	C.Rf_protect(r)
	for i, v := range p {
		C.SET_VECTOR_ELT(r, C.R_xlen_t(i), packSEXP%s(v))
	}
	C.Rf_unprotect(1)
	return r
`, rTypeLabelFor(typ), pkg.Mangle(elem))
		}

	case *types.Struct:
		n := typ.NumFields()
		fmt.Fprintf(buf, "\tr := C.allocList(%d)\n\tC.Rf_protect(r)\n", n)
		fmt.Fprintf(buf, "\tnames := C.Rf_allocVector(C.STRSXP, %d)\n\tC.Rf_protect(names)\n", n)
		fmt.Fprintln(buf, "\targ := r")
		for i := 0; i < n; i++ {
			f := typ.Field(i)
			rName := targetFieldName(typ, i)
			fmt.Fprintf(buf, "\tC.SET_STRING_ELT(names, %d, C.Rf_mkCharLenCE(C._GoStringPtr(`%s`), %d, C.CE_UTF8))\n", i, rName, len(rName))
			fmt.Fprintf(buf, "\tC.SETCAR(arg, packSEXP%s(p.%s))\n", pkg.Mangle(f.Type()), f.Name())
			if i < n-1 {
				fmt.Fprintln(buf, "\targ = C.CDR(arg)")
			}
		}
		fmt.Fprintln(buf, "\tC.setAttrib(r, packSEXP_types_Basic_string(`names`), names)\n\tC.Rf_unprotect(2)\n\treturn r")

	default:
		panic(fmt.Sprintf("unhandled type: %s", typ))
	}
}

// nameOf returns the package name-qualified name of t.
func nameOf(t types.Type) string {
	return types.TypeString(t, func(pkg *types.Package) string {
		return pkg.Name()
	})
}

// targetFieldName returns the rgo struct tag of the ith field of s if
// it exists, otherwise the name of the field.
func targetFieldName(s *types.Struct, i int) string {
	tag := reflect.StructTag(s.Tag(i)).Get("rgo")
	if tag != "" {
		return tag
	}
	return s.Field(i).Name()
}

var typeLabelTable = map[string]string{
	"logical":   "LGLSXP",
	"integer":   "INTSXP",
	"double":    "REALSXP",
	"complex":   "CPLXSXP",
	"character": "STRSXP",
	"raw":       "RAWSXP",
	"list":      "VECSXP",
}

func rTypeLabelFor(typ types.Type) string {
	name, _ := rTypeOf(typ)
	label, ok := typeLabelTable[name]
	if !ok {
		return fmt.Sprintf("<%s>", typ)
	}
	return label
}
