// Code generated by "stringer -type=ValueTag -linecomment"; DO NOT EDIT.

package lang

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ValueStr-0]
	_ = x[ValueBool-1]
	_ = x[ValueNum-2]
	_ = x[ValueArray-3]
	_ = x[ValueObj-4]
	_ = x[ValueNil-5]
	_ = x[ValueNativeFn-6]
	_ = x[ValueFn-7]
	_ = x[ValueRegex-8]
	_ = x[ValueUnknown-9]
}

const _ValueTag_name = "stringboolnumberarrayobjectnilnativefunctionfunctionregexunknown"

var _ValueTag_index = [...]uint8{0, 6, 10, 16, 21, 27, 30, 44, 52, 57, 64}

func (i ValueTag) String() string {
	if i >= ValueTag(len(_ValueTag_index)-1) {
		return "ValueTag(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _ValueTag_name[_ValueTag_index[i]:_ValueTag_index[i+1]]
}
