// Code generated by "go generate github.com/rgonomic/rgo/internal/pkg/testdata"; DO NOT EDIT.

package struct_string_out_0

//{"out":["string","struct{F1 string; F2 string \"rgo:\\\"Rname\\\"\"}"]}
func Test0() struct {
	F1 string
	F2 string "rgo:\"Rname\""
} {
	var res0 struct {
		F1 string
		F2 string "rgo:\"Rname\""
	}
	return res0
}
