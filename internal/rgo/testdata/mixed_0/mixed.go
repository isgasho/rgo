// Code generated by "go generate github.com/rgonomic/rgo/internal/pkg/testdata"; DO NOT EDIT.

package mixed_0

type (
	T  int
	S1 string
)

// Test0 does things with [int] and returns [int int].
func Test0(par0 int) (int, int) {
	var res0 int
	var res1 int
	return res0, res1
}

// Test1 does things with [int] and returns [float64 int].
func Test1(par0 int) (res0 float64, res1 int) {
	return res0, res1
}

// Test2 does things with [int] and returns [].
func Test2(par0 int) {
}

// Test3 does things with [T S1] and returns [S1].
func Test3(par0 T, par1 S1) S1 {
	var res0 S1
	return res0
}
