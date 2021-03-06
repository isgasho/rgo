package cca

import (
	"gonum.org/v1/gonum/blas/blas64"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

// CCA performs a canonical correlation analysis of the input data x
// and y, columns of which should be interpretable as two sets of
// measurements on the same observations (rows).
func CCA(x, y blas64.GeneralCols) (ccors []float64, pVecs, qVecs, phiVs, psiVs blas64.GeneralCols, err error) {
	var xdata, ydata mat.Dense
	xdata.SetRawMatrix(rowMajor(x))
	ydata.SetRawMatrix(rowMajor(y))

	var cc stat.CC
	err = cc.CanonicalCorrelations(&xdata, &ydata, nil)
	if err != nil {
		return nil, pVecs, qVecs, phiVs, psiVs, err
	}
	ccors = cc.CorrsTo(nil)

	var _pVecs, _qVecs, _phiVs, _psiVs mat.Dense
	cc.LeftTo(&_pVecs, true)
	cc.RightTo(&_qVecs, true)
	cc.LeftTo(&_phiVs, false)
	cc.RightTo(&_psiVs, false)

	return ccors,
		colMajor(_pVecs.RawMatrix()),
		colMajor(_qVecs.RawMatrix()),
		colMajor(_phiVs.RawMatrix()),
		colMajor(_psiVs.RawMatrix()),
		err
}

func rowMajor(a blas64.GeneralCols) blas64.General {
	t := blas64.General{
		Rows:   a.Rows,
		Cols:   a.Cols,
		Data:   make([]float64, len(a.Data)),
		Stride: a.Cols,
	}
	t.From(a)
	return t
}

func colMajor(a blas64.General) blas64.GeneralCols {
	t := blas64.GeneralCols{
		Rows:   a.Rows,
		Cols:   a.Cols,
		Data:   make([]float64, len(a.Data)),
		Stride: a.Rows,
	}
	t.From(a)
	return t
}
