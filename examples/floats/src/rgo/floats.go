// Code generated by rgnonomic/rgo; DO NOT EDIT.

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

	"gonum.org/v1/gonum/floats"
)


//export Wrapped_CumProd
func Wrapped_CumProd(_R_dst, _R_s C.SEXP) C.SEXP {
	defer func() {
		r := recover()
		if r != nil {
			err := C.CString(fmt.Sprint(r))
			C.R_error(err)
			C.free(unsafe.Pointer(err))
		}
	}()

	_p0 := unpackSEXP_types_Slice___float64(_R_dst)
	_p1 := unpackSEXP_types_Slice___float64(_R_s)
	_r0 := floats.CumProd(_p0, _p1)
	return packSEXP_CumProd(_r0)
}

func packSEXP_CumProd(p0 []float64) C.SEXP {
	return packSEXP_types_Slice___float64(p0)
}

//export Wrapped_CumSum
func Wrapped_CumSum(_R_dst, _R_s C.SEXP) C.SEXP {
	defer func() {
		r := recover()
		if r != nil {
			err := C.CString(fmt.Sprint(r))
			C.R_error(err)
			C.free(unsafe.Pointer(err))
		}
	}()

	_p0 := unpackSEXP_types_Slice___float64(_R_dst)
	_p1 := unpackSEXP_types_Slice___float64(_R_s)
	_r0 := floats.CumSum(_p0, _p1)
	return packSEXP_CumSum(_r0)
}

func packSEXP_CumSum(p0 []float64) C.SEXP {
	return packSEXP_types_Slice___float64(p0)
}

func unpackSEXP_types_Slice___float64(p C.SEXP) []float64 {
	if C.Rf_isNull(p) != 0 {
		return nil
	}
	n := C.Rf_xlength(p)
	return (*[70368744177664]float64)(unsafe.Pointer(C.REAL(p)))[:n:n]
}

func packSEXP_types_Basic_float64(p float64) C.SEXP {
	return C.ScalarReal(C.double(p))
}

func packSEXP_types_Slice___float64(p []float64) C.SEXP {
	r := C.Rf_allocVector(C.REALSXP, C.R_xlen_t(len(p)))
	C.Rf_protect(r)
	s := (*[70368744177664]float64)(unsafe.Pointer(C.REAL(r)))[:len(p):len(p)]
	copy(s, p)
	C.Rf_unprotect(1)
	return r
}

func main() {}