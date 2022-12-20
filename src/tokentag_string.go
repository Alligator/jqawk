// Code generated by "stringer -type=TokenTag -linecomment"; DO NOT EDIT.

package lang

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[EOF-0]
	_ = x[Error-1]
	_ = x[Ident-2]
	_ = x[Str-3]
	_ = x[Regex-4]
	_ = x[Num-5]
	_ = x[Begin-6]
	_ = x[End-7]
	_ = x[Print-8]
	_ = x[Function-9]
	_ = x[Return-10]
	_ = x[If-11]
	_ = x[Else-12]
	_ = x[For-13]
	_ = x[While-14]
	_ = x[In-15]
	_ = x[Match-16]
	_ = x[True-17]
	_ = x[False-18]
	_ = x[LCurly-19]
	_ = x[RCurly-20]
	_ = x[LSquare-21]
	_ = x[RSquare-22]
	_ = x[LParen-23]
	_ = x[RParen-24]
	_ = x[LessThan-25]
	_ = x[GreaterThan-26]
	_ = x[Dollar-27]
	_ = x[Comma-28]
	_ = x[Dot-29]
	_ = x[Equal-30]
	_ = x[EqualEqual-31]
	_ = x[BangEqual-32]
	_ = x[LessEqual-33]
	_ = x[GreaterEqual-34]
	_ = x[Colon-35]
	_ = x[SemiColon-36]
	_ = x[Plus-37]
	_ = x[Minus-38]
	_ = x[Multiply-39]
	_ = x[Divide-40]
	_ = x[PlusEqual-41]
	_ = x[MinusEqual-42]
	_ = x[MultiplyEqual-43]
	_ = x[DivideEqual-44]
	_ = x[Tilde-45]
	_ = x[BangTilde-46]
	_ = x[AmpAmp-47]
	_ = x[PipePipe-48]
	_ = x[Arrow-49]
	_ = x[Bang-50]
	_ = x[PlusPlus-51]
	_ = x[MinusMinus-52]
}

const _TokenTag_name = "EOFErrorIdentStrRegexNumBeginEndPrintFunctionReturnIfElseForWhileInMatchtruefalse{}[]()<>$,.===!=<=>=:;+-*/+=-=*=/=~!~&&||=>!++--"

var _TokenTag_index = [...]uint8{0, 3, 8, 13, 16, 21, 24, 29, 32, 37, 45, 51, 53, 57, 60, 65, 67, 72, 76, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 95, 97, 99, 101, 102, 103, 104, 105, 106, 107, 109, 111, 113, 115, 116, 118, 120, 122, 124, 125, 127, 129}

func (i TokenTag) String() string {
	if i >= TokenTag(len(_TokenTag_index)-1) {
		return "TokenTag(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _TokenTag_name[_TokenTag_index[i]:_TokenTag_index[i+1]]
}
