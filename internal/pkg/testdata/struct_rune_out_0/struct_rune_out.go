// Code generated by "go generate github.com/rgonomic/rgo/internal/pkg/testdata"; DO NOT EDIT.

package struct_rune_out_0

//{"out":["int32","struct{F1 rune; F2 rune \"rgo:\\\"Rname\\\"\"}"]}
func Test0() struct {
	F1 rune
	F2 rune "rgo:\"Rname\""
} {
	var res0 struct {
		F1 rune
		F2 rune "rgo:\"Rname\""
	}
	return res0
}