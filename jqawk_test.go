package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alligator/jqawk/cli"
	"github.com/alligator/jqawk/src"
)

type testCase struct {
	name          string
	prog          string
	json          string
	json2         string
	expected      string
	expectedError string
	expectedJson  string
	args          []string
}

// basic tests with no JSON input
var basicTests = []struct {
	prog     string
	expected string
}{
	// BEGIN and END
	{"BEGIN { print 'hello' }", "hello\n"},
	{"BEGIN { print 'hello' } BEGIN { print 'other hello' }", "hello\nother hello\n"},
	{"END { print 'bye' }", "bye\n"},
	{"BEGIN { print 'hello' } END { print 'bye' }", "hello\nbye\n"},

	// operators
	{"BEGIN { print 2 + 3 }", "5\n"},
	{"BEGIN { print 2 - 3 }", "-1\n"},
	{"BEGIN { print 2 * 3 }", "6\n"},
	{"BEGIN { print 6 / 3 }", "2\n"},
	{"BEGIN { print 6 / 2 - 1 * 3 }", "0\n"},
	{"BEGIN { print 4 % 3 }", "1\n"},
	{"BEGIN { print -4 % 3 }", "-1\n"},
	{"BEGIN { print 2 ** 2 }", "4\n"},
	{"BEGIN { print 2 ** 4 }", "16\n"},
	{"BEGIN { print 2 ** -1 }", "0.5\n"},
	{"BEGIN { print -2 ** 2 }", "-4\n"},
	{"BEGIN { print 2 ** -2 }", "0.25\n"},

	{"BEGIN { print 3 < 4 }", "true\n"},
	{"BEGIN { print 3 <= 3 }", "true\n"},
	{"BEGIN { print 3 > 2 }", "true\n"},
	{"BEGIN { print 3 >= 3 }", "true\n"},
	{"BEGIN { print false && true; }", "false\n"},
	{"BEGIN { print false || true; }", "true\n"},

	{"BEGIN { obj = { a: 1 }; print !obj.a }", "false\n"},
	{"BEGIN { obj = { a: 1 }; print -obj.a + 2 }", "1\n"},
	{"BEGIN { obj = { a: 1 }; print -(obj.a + 2) }", "-3\n"},

	{"BEGIN { for(i = 0; i < 2; i++) print a++, b--, b--, ++c, --d, --d }", "0 0 -1 1 -1 -2\n1 -2 -3 2 -3 -4\n"},
	{"BEGIN { a = 3; a += 3; print a }", "6\n"},
	{"BEGIN { a = 3; a -= 3; print a }", "0\n"},
	{"BEGIN { a = 3; a *= 3; print a }", "9\n"},
	{"BEGIN { a = 3; a /= 3; print a }", "1\n"},
	{"BEGIN { a = 3; b = 2; a -= b -= 1; print a }", "2\n"},

	{"BEGIN { print !false, !true }", "true false\n"},
	{"BEGIN { n = -3; p = 3; print +n, -p }", "-3 -3\n"},

	{"BEGIN { print 4 > 7 ? 't' : 'f' }", "f\n"},
	{"BEGIN { print 4 == 4 ? 't' : 'f' }", "t\n"},

	{"BEGIN { a = '1'; a += 2; print a }", "12\n"},
	{"BEGIN { a = 1; a += '2'; print a }", "12\n"},
	{"BEGIN { a = +'1'; print a is number }", "true\n"},
	{"BEGIN { a = false; a += 1; print a }", "1\n"},

	// associativity
	{"BEGIN { print 3 - 2 -1 }", "0\n"},
	{"BEGIN { print 8 / 4 / 2 }", "1\n"},
	{"BEGIN { print 2 ** 2 ** 3 }", "256\n"},

	// printf %v
	{"BEGIN { printf('v %v %v %v %v\n', 1, 'one', [1, 2, 3], function() { }) }", "v 1 one [1, 2, 3] <function>\n"},

	// object/array/string indexing
	{"BEGIN { a = [1]; print a[0], a[1], a.nope }", "1 null null\n"},
	{"BEGIN { a = { b: 2 }; print a.b, a.a, a.nope, a['b'], a[0] }", "2 null null 2 null\n"},
	{"BEGIN { a = 't'; print a[0], a[1], a.b, a.length }", "t null null <nativefunction>\n"},

	// indexing edge cases
	{"BEGIN { a = {}; print a[false] }", "null\n"},

	// number conversions
	{"BEGIN { a = false; b = true; print a + 0, b + 0 }", "0 1\n"},
	{"BEGIN { a = false; b = true; print a + 0, b + 0 }", "0 1\n"},
}

var tests = []testCase{
	{
		name: "README example",
		prog: `
			BEGIN {
				print 'Pay'
				print '----------------'
			}

			$.hours > 0 {
				printf("%-8s %f\n", $.name, $.rate * $.hours)
				total += $.rate * $.hours
			}

			END {
				print '----------------'
				print'Total   ', total
			}
		`,
		json: `[
			{ "name": "Beth", "rate": 4, "hours": 0 },
			{ "name": "Dan", "rate": 3.75, "hours": 0 },
			{ "name": "Kathy", "rate": 4, "hours": 10 },
			{ "name": "Mark", "rate": 5, "hours": 20 },
			{ "name": "Mary", "rate": 5.50, "hours": 22 },
			{ "name": "Susie", "rate": 4.25, "hours": 18 }
		]`,
		expected: `Pay
----------------
Kathy    40
Mark     100
Mary     121
Susie    76.5
----------------
Total    337.5
`,
	},
	{
		name: "advent of code example",
		prog: `
			BEGIN {
				part1 = 0
			}

			{
				chars = []
				for (c in $.split("")) {
					if (c ~ "[0-9]") {
						chars.push(c)
					}
				}
				part1 += num(chars[0] + chars[-1])
			}

			END {
				print "part 1:", part1
			}
		`,
		json:     `["1abc2", "pqr3stu8vwx", "a1b2c3d4e5f", "treb7uchet"]`,
		expected: "part 1: 142\n",
	},
	{
		name:     "comments",
		prog:     "BEGIN { print 'hello' } # prints hello\n# goodbye",
		expected: "hello\n",
	},
	{
		name: "short circuiting",
		prog: `
			function a(r) {
				print 'a'
				return r
			}

			function b() {
				print 'b'
				return true
			}

			BEGIN {
				a(true) || b()
				a(false) || b()
				a(false) && b()
				a(true) && b()
			}
		`,
		expected: "a\na\nb\na\na\nb\n",
	},
	{
		name:     "dot",
		prog:     "{ print $.name }",
		json:     `[{ "name": "gate" }, { "name": "sponge" }]`,
		expected: "gate\nsponge\n",
	},
	{
		name:     "subscript",
		prog:     "{ print $['name'] }",
		json:     `[{ "name": "gate" }, { "name": "sponge" }]`,
		expected: "gate\nsponge\n",
	},
	{
		name:     "subscript array",
		prog:     "{ print $[0] }",
		json:     `[[1, 2], [10, 20], [100, 200]]`,
		expected: "1\n10\n100\n",
	},
	{
		name:     "subscript array negative index",
		prog:     "{ print $[-1] }",
		json:     `[[1, 2], [10, 20], [100, 200]]`,
		expected: "2\n20\n200\n",
	},
	{
		name:          "subscript array with string",
		prog:          "{ A = []; A.A = 2; }",
		json:          `[1]`,
		expectedError: "array indices must be numbers",
	},
	{
		name:     "subscript string",
		prog:     "{ print $[0], $[-1] }",
		json:     `["hello"]`,
		expected: "h o\n",
	},
	{
		name:     "unknown variable comparison",
		prog:     "$ > max { max = $ } $ < min { min = $ } END { print min, max }",
		json:     `[1, 2, 3, 4, 3, 2, 1]`,
		expected: "1 4\n",
	},
	{
		name:     "semicolon statement separator",
		prog:     "{ print 'a'; print 'b' }",
		json:     `[1]`,
		expected: "a\nb\n",
	},
	{
		name:     "pretty print",
		prog:     "{ print }",
		json:     `[[1, 2], { "name": "alligator" }]`,
		expected: "[1, 2]\n{\"name\": \"alligator\"}\n",
	},
	{
		name:     "string concatenation",
		prog:     "{ print 'name: ' + $.name, 'age: ' + $.age }",
		json:     `[{ "name": "gate", "age": 1 }, { "name": "sponge", "age": 2 }]`,
		expected: "name: gate age: 1\nname: sponge age: 2\n",
	},
	{
		name: "equal, not equal",
		prog: `
			$.name == 'gate' { print 'eq', $.name }
			$.name != 'gate' { print 'neq', $.name }
		`,
		json:     `[{ "name": "gate", "age": 1 }, { "name": "sponge", "age": 2.300 }]`,
		expected: "eq gate\nneq sponge\n",
	},
	{
		name: "regex match, not match",
		prog: `
			$.name ~ 'gate' { print 'eq', $.name }
			$.name !~ 'gate' { print 'neq', $.name }
		`,
		json:     `[{ "name": "gate", "age": 1 }, { "name": "sponge", "age": 2.300 }]`,
		expected: "eq gate\nneq sponge\n",
	},
	{
		name:     "order of operations",
		prog:     "$.age + 1 > 2 { print $.name }",
		json:     `[{ "name": "gate", "age": 1 }, { "name": "sponge", "age": 2.300 }]`,
		expected: "sponge\n",
	},
	{
		name: "functions",
		prog: `
			function add(a, b) {
				return a + b;
			}

			function empty_return(a, b) {
				print a;
				return;
				print b;
			}

			function log(a) { print a }

			function missing_args(a, b, c) { print a, b, c }

			BEGIN {
				function in_rule() {
					print 'in rule'
				}

				first_class = function() {
					print 'first class'
				}

				print add(3, 4)
				log("hello")
				empty_return(5, 6)
				in_rule()
				first_class()

				missing_args(1)
			}
		`,
		json:     "[]",
		expected: "7\nhello\n5\nin rule\nfirst class\n1 null null\n",
	},
	{
		name: "if",
		prog: `
			{
				if ($ > 5) {
					print $;
				}
			}
		`,
		json:     `[2, 7, 3, 12, 87, -3, 0]`,
		expected: "7\n12\n87\n",
	},
	{
		name: "else",
		prog: `
			{
				if ($ > 5) {
					print $;
				} else {
					printf("%f <= 5\n", $);
				}
			}
		`,
		json:     `[2, 7, 3, 12, 87, -3, 0]`,
		expected: "2 <= 5\n7\n3 <= 5\n12\n87\n-3 <= 5\n0 <= 5\n",
	},
	{
		name:     "root object",
		prog:     "{ print $.name }",
		json:     `{ "name": "alligator", "location": "uk" }`,
		expected: "alligator\n",
	},
	{
		name:     "root number",
		prog:     "{ print }",
		json:     `45.67`,
		expected: "45.67\n",
	},
	{
		name: "while",
		prog: `
			BEGIN {
				i = 0;
				while (i < 3) {
					print i;
					i += 1;
				}
			}
		`,
		json:     "[]",
		expected: "0\n1\n2\n",
	},
	{
		name: "for",
		prog: `
			BEGIN {
				for (i = 0; i < 3; i += 1) {
					printf("%d", i)
				}
				printf("\n")

				j = 0
				for (; j < 3; j++) {
					printf("%d", j)
				}
				printf("\n")

				k = 0
				for(;;k++) {
					if (k >= 3)
						break
					printf("%d", k)
				}
				printf("\n")

				l = 0
				for (;;) {
					if (l >= 3)
						break
					printf("%d", l)
					l++
				}
				printf("\n")

				for (m = 0;;m++) {
					if (m >= 3)
						break
					printf("%d", m)
				}
			}
		`,
		json: "[]",
		expected: `012
012
012
012
012`,
	},
	{
		name: "for in",
		prog: `
			{
				for (x in $) {
					print x;
				}

				for (x, i in $) {
					print i, x;
				}
			}

			END {
				for (c, i in 'bye') {
					print i, c;
				}
			}
		`,
		json:     "[[1, 2], [3, 4]]",
		expected: "1\n2\n0 1\n1 2\n3\n4\n0 3\n1 4\n0 b\n1 y\n2 e\n",
	},
	{
		name: "for in object",
		prog: `
			{
				for (k, v in $) {
					print k, v;
				}
			}
		`,
		json:     `[{ "a": 1 }]`,
		expected: "a 1\n",
	},
	{
		name: "for in break and continue",
		prog: `
			function test(x) {
				let i = 0
				for (v in x) {
					i++;
					if (i == 1) continue;
					if (i == 3) break;
					print v;
				}
			}

			BEGIN {
				test([1, 2, 3])
				test({ a: 1, b: 2, c: 3 })
				test('hjkl')
			}
		`,
		json:     "",
		expected: "2\nb\nj\n",
	},
	{
		name: "match",
		prog: `
			{
				print match ($) {
					1 => 'one',
					2 => 'two',
					3, 4 => 'more',
					_ => '?',
				}

				match ($) {
					2 => {
						print '2'
					}
				}
			}
		`,
		json:     "[1, 2, 3, 4, 5]",
		expected: "one\ntwo\n2\nmore\nmore\n?\n",
	},
	{
		name: "match array",
		prog: `
			{
				print match ($) {
					[1, x] => x * 2,
					[2, x] => x + 10,
				}
			}
		`,
		json:     "[[1, 1], [1, 2], [2, 1]]",
		expected: "2\n4\n11\n",
	},
	{
		name: "match nested array",
		prog: `
			{
				print match ($) {
					[x, [2, y]] => y,
					[x, [5, y]] => x,
				}
			}
		`,
		json:     "[[1, [2, 3]], [4, [5, 6]]]",
		expected: "3\n4\n",
	},
	{
		name:     "length methods",
		prog:     "{ print $.obj.length(), $.array.length(); }",
		json:     `[{ "obj": { "key1": 1, "key2": 2 }, "array": [1, 2, 3, 4] }]`,
		expected: "2 4\n",
	},
	{
		name:     "implicit object creation",
		prog:     "BEGIN { new_obj.name = 'hi'; print new_obj.name; }",
		json:     `[]`,
		expected: "hi\n",
	},
	{
		name:     "optional chaining",
		prog:     "{ print $.a.b.c.d.e }",
		json:     `[{ "a": 1 }]`,
		expected: "null\n",
	},
	{
		name:     "deep implicit object creation",
		prog:     "BEGIN { new_obj.a.b.c = 'hi'; print new_obj; }",
		json:     `[]`,
		expected: `{"a": {"b": {"c": "hi"}}}` + "\n",
	},
	{
		name:     "implicit array creation",
		prog:     "BEGIN { a[0] = 1; a[2] = 'hello'; print a; }",
		json:     "[]",
		expected: "[1, null, \"hello\"]\n",
	},
	{
		name:     "deep implicit array creation",
		prog:     "BEGIN { a[0][0] = 1; a[2][1] = 2; print a; }",
		json:     "[]",
		expected: "[[1], null, [null, 2]]\n",
	},
	{
		name:     "implicit object-in-array creation",
		prog:     "BEGIN { a[0]['a'] = 1; a[2]['b'] = 'hello'; print a; }",
		json:     "[]",
		expected: "[{\"a\": 1}, null, {\"b\": \"hello\"}]\n",
	},
	{
		name:     "groupings",
		prog:     "BEGIN { print (1 + 2) * 3; }",
		json:     "[]",
		expected: "9\n",
	},
	{
		name: "array literal",
		prog: `
			BEGIN {
				x = [];
				y = [1, 2, 3];
				print x, y;
			}
		`,
		json:     "[]",
		expected: "[] [1, 2, 3]\n",
	},
	{
		name: "object literal",
		prog: `
			BEGIN {
				x = { a: 1, 'b': '2' };
				print x.a, x.b;
			}
		`,
		json:     "[]",
		expected: "1 2\n",
	},
	{
		name: "break",
		prog: `
			BEGIN {
				for (i = 0; i < 10; i++) {
					print i;
					if (i > 2) {
						break;
					}
				}
			}
		`,
		json:     "[]",
		expected: "0\n1\n2\n3\n",
	},
	{
		name: "break nested",
		prog: `
			BEGIN {
				for (i = 0; i < 3; i++) {
					for (j = 0; j < 3; j++) {
						if (j == 2)
							break;
						print i, j;
					}
				}
			}
		`,
		json: "[]",
		expected: `0 0
0 1
1 0
1 1
2 0
2 1
`,
	},
	{
		name: "continue",
		prog: `
			BEGIN {
				for (i = 0; i < 4; i++) {
					if (i == 2) {
						continue;
					}
					print i;
				}
			}
		`,
		json:     "[]",
		expected: "0\n1\n3\n",
	},
	{
		name: "continue nested",
		prog: `
			BEGIN {
				for (i = 0; i < 5; i++) {
					if (i < 2)
						continue
					print i
				}
			}
		`,
		expected: "2\n3\n4\n",
	},
	{
		name:     "next",
		prog:     "{ print $; next } { print $ }",
		json:     "[1, 2, 3, 4]",
		expected: "1\n2\n3\n4\n",
	},
	{
		name:     "printing circular references",
		prog:     "BEGIN { a.a = 3; a.a = a; print a; b = []; b[0] = 1; b[1] = b; print b; }",
		json:     "[]",
		expected: "{\"a\": <circular reference>}\n[1, <circular reference>]\n",
	},
	{
		name:          "converting circular references to JSON",
		prog:          "BEGIN { a.a = 3; a.a = a; print json(a) }",
		json:          "[]",
		expectedError: "error creating JSON: circular reference",
	},
	{
		name: "string methods",
		prog: `
			BEGIN {
				print "aBc".upper()
				print "aBc".lower()
				print "aBc".split("B")
				print "aBc".length()
				print "aBCd".split(/BC/)
				print "  aBc ".trim()
			}
		`,
		json: "[]",
		expected: `ABC
abc
["a", "c"]
3
["a", "d"]
aBc
`,
	},
	{
		name:     "pluck",
		prog:     "{ print $.pluck('a') }",
		json:     `[{ "a": 1, "b": 2}]`,
		expected: "{\"a\": 1}\n",
	},
	{
		name:     "exit",
		prog:     `{ print $; exit }`,
		json:     "[1, 2]",
		expected: "1\n",
	},
	{
		name: "exit BEGINFILE",
		prog: `
			BEGINFILE { print 'bf1'; exit }
			BEGINFILE { print 'bf2' }
			{ print $ }
		`,
		json:     "[1, 2]",
		expected: "bf1\n",
	},
	{
		name: "exit ENDFILE",
		prog: `
			{ print $ }
			ENDFILE { print 'ef1'; exit }
			ENDFILE { print 'ef2' }
		`,
		json:     "[1, 2]",
		expected: "1\n2\nef1\n",
	},
	{
		name: "null comparison",
		prog: `
			$.a == null { print 'lhs null' }
			$.a != null { print 'lhs not null' }

			null == $.a { print 'rhs null' }
			null != $.a { print 'rhs not null' }

			END {
				if (null > 0) print 'oh no'
				if (0 < null) print 'oh no'
			}
		`,
		json: `[{ "a": null }, { "a": "not null" }, { "a": 1 }]`,
		expected: `lhs null
rhs null
lhs not null
rhs not null
lhs not null
rhs not null
`,
	},
	{
		name:     "multiple inputs",
		prog:     "{ print $.a }",
		json:     `[{ "a": 1 }]`,
		json2:    `[{ "a": 2 }]`,
		expected: "1\n2\n",
	},
	{
		name:     "$file",
		prog:     "{ print $file, $.a }",
		json:     `[{ "a": 1 }]`,
		json2:    `[{ "a": 2 }]`,
		expected: "<test1> 1\n<test2> 2\n",
	},
	{
		name:     "truthiness",
		prog:     "BEGIN { print !![], !!{}, !!0, !!1, !!'', !!'abc' }",
		json:     "[]",
		expected: "true true false true false true\n",
	},
	{
		name:     "BEGIN and END with multiple inputs",
		prog:     "BEGIN { print 'hi' } END { print 'bye' }",
		json:     `[{ "a": 1 }]`,
		json2:    `[{ "a": 2 }]`,
		expected: "hi\nbye\n",
	},
	{
		name:     "floating point",
		prog:     "BEGIN { print 0.2 + 0.3 + num('1.0') }",
		json:     "",
		expected: "1.5\n",
	},
	{
		name: "is operator",
		prog: `
			function fn() {}

			{
				if ($ is string) print 'string';
				if ($ is bool) 	 print 'bool';
				if ($ is number) print 'number';
				if ($ is array)  print 'array';
				if ($ is object) print 'object';
				if ($ is null)   print 'null';
			}

			END {
				if (fn is function)  print 'function';
				if (/123/ is regex)  print 'regex';
				if (x is unknown)    print 'unknown';
				if (num is function) print 'nativefunction'
			}
		`,
		json:     "[\"1\", false, 2, [3], { \"n\": 4 }, null]",
		expected: "string\nbool\nnumber\narray\nobject\nnull\nfunction\nregex\nunknown\nnativefunction\n",
	},
	{
		name: "array sort",
		prog: `
			BEGIN {
				a = [4, 5, 3, 1, 2];
				b = ['clown', {a: 1}, 'bee', [1], 'dog'];
				print a.sort();  # numbers
				print b.sort();  # strings and other things
				print a;         # original array is unmodified

				function s(a, b) {
					if (a < b) { return -1 }
					if (a > b) { return 1 }
					return 0
				}

				print a.sort(s) # sort function
			}
		`,
		json: "[]",
		expected: `[1, 2, 3, 4, 5]
[{"a": 1}, [1], "bee", "clown", "dog"]
[4, 5, 3, 1, 2]
[1, 2, 3, 4, 5]
`,
	},
	{
		name: "array sortKey()",
		prog: `
			BEGIN {
				a = [{ n: 1 }, { n: 8 }, { n: 0 }]
				print a.sortKey("n")
				print a.sortKey(function(x) { return x.n })
			}
		`,
		json: "[]",
		expected: `[{"n": 0}, {"n": 1}, {"n": 8}]
[{"n": 0}, {"n": 1}, {"n": 8}]
`,
	},
	{
		name: "array map()",
		prog: `BEGIN {
			a = [1, 2, 3]
			print a.map(function(x) { return x * x })
		}`,
		json:     "",
		expected: "[1, 4, 9]\n",
	},
	{
		name: "array map() nativefn",
		prog: `BEGIN {
			a = ['1', '2', '3']
			print a.map(num)
		}`,
		json:     "",
		expected: "[1, 2, 3]\n",
	},
	{
		name: "array filter()",
		prog: `BEGIN {
			a = [1, 2, 3]
			print a.filter(function(x) { return x > 1 })
		}`,
		json:     "",
		expected: "[2, 3]\n",
	},
	{
		name: "array filter() nativefn",
		prog: `BEGIN {
			a = ["1", "2", "a", "3"]
			print a.filter(num)
		}`,
		json:     "",
		expected: "[\"1\", \"2\", \"3\"]\n",
	},
	{
		name: "beginfile endfile",
		prog: `
			BEGINFILE { print 'beginfile', $ }
			ENDFILE { print 'endfile', $ }
		`,
		json:     "123",
		json2:    "456",
		expected: "beginfile 123\nendfile 123\nbeginfile 456\nendfile 456\n",
	},
	{
		name: "$ is the root value in endfile",
		prog: `
			BEGINFILE { $ = $.stuff }
			{ print $ }
			ENDFILE { print $ }
		`,
		json:     `{ "stuff": [1, 2, 3] }`,
		expected: "1\n2\n3\n[1, 2, 3]\n",
	},
	{
		name: "num methods",
		prog: `
			BEGIN {
				a = 2.5
				print a.floor()
				print a.ceil()
				print a.round()
				print (3.5).round()
				print (10).mod(8)
				print (-10).mod(8)
				print (10).abs()
				print (-10).abs()
				for (i = 1; i < 10000000; i *= 10) {
					print i.format(), (i + 0.23).format()
					print (i + 0.23).format('.', ',')
				}
			}
		`,
		json: "[]",
		expected: `2
3
3
4
2
6
10
10
1 1.23
1,23
10 10.23
10,23
100 100.23
100,23
1,000 1,000.23
1.000,23
10,000 10,000.23
10.000,23
100,000 100,000.23
100.000,23
1,000,000 1,000,000.23
1.000.000,23
`,
	},
	{
		name:     "jsonl",
		prog:     "{ print $ }",
		json:     "[1, 2]\n[3, 4]",
		expected: "1\n2\n3\n4\n",
	},
	{
		name:     "escape chars",
		prog:     `BEGIN { print 'one\ntwo\tthree\\four' }`,
		json:     "[]",
		expected: "one\ntwo\tthree\\four\n",
	},
	{
		name:          "invalid escape chars",
		prog:          `BEGIN { print '\z' }`,
		json:          "[]",
		expectedError: "unknown escape char 'z'",
	},
	{
		name: "maintain key order",
		prog: `{ print $; print json($) }`,
		json: `[{ "b": 3, "a": 4 }]`,
		expected: `{"b": 3, "a": 4}
{
  "b": 3,
  "a": 4
}
`,
	},
	{
		name: "array methods",
		prog: `
			BEGIN {
				a = [1, 2]
				a.push(3)
				a.push(4)
				print a, a.length()
				print a.pop(), a
				print a.popfirst(), a
				print a.contains(1), a.contains(2)
			}
		`,
		json: "[]",
		expected: `[1, 2, 3, 4] 4
4 [1, 2, 3]
1 [2, 3]
false true
`,
	},
	{
		name:     "object.pairs",
		prog:     "{ print $.pairs() }",
		json:     `[{ "a": 1, "b": "two" }]`,
		expected: "[[\"a\", 1], [\"b\", \"two\"]]\n",
	},
	{
		name:     "object.keys",
		prog:     "{ print $.keys() }",
		json:     `[{ "a": 1, "b": "two" }]`,
		expected: "[\"a\", \"b\"]\n",
	},
	{
		name:     "object.values",
		prog:     "{ print $.values() }",
		json:     `[{ "a": 1, "b": "two" }]`,
		expected: "[1, \"two\"]\n",
	},
	{
		name: "closures",
		prog: `
			function outer() {
				a = 3;
				return function() {
					return a;
				}
			}

			BEGIN {
				print outer()()
			}
		`,
		expected: "3\n",
	},
	{
		name: "recursion",
		prog: `
			function fib(a) {
				if (a == 1 || a == 2) {
					return 1;
				}
				return fib(a - 1) + fib(a - 2);
			}

			BEGIN {
				print fib(8);
			}
		`,
		json:     "",
		expected: "21\n",
	},
	{
		name: "slice",
		prog: `
			function check(actual, expected) {
				if (expected is array) {
					same = true
					for (item, i in actual) {
						if (actual[i] != expected[i]) {
							same = false
							break
						}
					}
					if (!same) {
						print 'failed', actual, '!=', expected
					}
					return
				}

				if (actual != expected) {
					print 'failed', actual, '!=', expected
				}
			}

			BEGIN {
				s = 'abcd'
				a = [1, 2, 3, 4]

				check(s[1:2],  'b')
				check(s[:1],   'a')
				check(s[2:],   'cd')
				check(s[:],    'abcd')
				check(s[1:-1], 'bc')
				check(s[:-2],  'ab')
				check(s[3:7],  'd')

				check(a[1:2],  [2])
				check(a[:1],   [1])
				check(a[2:],   [3, 4])
				check(a[:],    [1, 2, 3, 4])
				check(a[1:-1], [2, 3])
				check(a[:-2],  [1, 2])
				check(a[3:7],  [4])
			}`,
		expected: "",
	},
	{
		name:          "invalid slice",
		prog:          `{ print $[2:1] }`,
		json:          "[[1, 2, 3, 4]]",
		expectedError: "index out of range",
	},
	{
		name: "scoping",
		prog: `
			function fn() {
				let a = 3;
				b = 4;
			}

			BEGIN {
				a = 1;
				if (true) {
					b = 2;
				}
				print a, b;
				fn();
				print a;
				print b;
			}
		`,
		json:     "",
		expected: "1 2\n1\n4\n",
	},
	{
		name: "parseJson",
		prog: "{ b = parseJson($.json); print b.name; }",
		json: `[
			{ "json": "{ \"name\": \"beep\" }" },
			{ "json": "{ \"name\": \"boop\" }" }
		]`,
		expected: "beep\nboop\n",
	},
	{
		name: "num native fn",
		prog: `BEGIN {
			print num(1.23), num('1.23'), num(true), num('nope'), num([1,2,3])
		}`,
		json:     "",
		expected: "1.23 1.23 1 null null\n",
	},
	{
		name: "assignment semantics",
		prog: `

		function check(ok, name) {
			if (!ok) {
				print 'failed:', name
			}
		}
			
		BEGIN {
			# references
			{
				obj = {}
				arr = []

				obj.name = 'obj ref'
				obj2 = obj
				check(obj.name == 'obj ref', 'obj ref')

				arr[0] = obj
				arr[0].name = 'obj in array'
				check(obj.name == 'obj in array', 'obj ref')

				obj.a = arr
				obj.a[0] = 'array in obj'
				check(arr[0] == 'array in obj', 'array in obj')
			}

			# values
			{
				obj = {}
				arr = []
				n = 3

				arr[0] = n
				arr[0]++
				check(n == 3, 'original in array is unmodified')
				check(arr[0] == 4, 'array item is modified')

				obj.n = n
				obj.n--
				check(n == 3, 'original in object is unmodified')
				check(obj.n == 2, 'object property is modified')
			}

			# array literals
			{
				obj = { 'name': 'test' }
				n = 5
				arr = [obj, n]

				arr[0].name = 'testing'
				arr[1] += 2

				check(arr[0].name == 'testing', 'obj in array literal')
				check(arr[1] == 7, 'var in array literal')
				check(n == 5, 'original var unmodified in array literal')
			}

			# object literals
			{
				arr = [1]
				n = 7
				obj = { 'arr': arr, 'n': n }

				obj.arr[0]++
				obj.n -= 2

				check(obj.arr[0] == 2, 'array in obj literal')
				check(obj.n == 5, 'var in obj literal')
				check(n == 7, 'original var unmodified in obj literal')
			}

			# function calls
			{
				function modifyObj(o) { o.name = 'ken' }
				function modifyArr(a) { a[0] = 'ben' }
				function modifyNum(n) { n += 5 }

				obj = {}
				modifyObj(obj)

				arr = []
				modifyArr(arr)

				n = 5
				modifyNum(n)

				check(obj.name == 'ken', 'modify obj in function')
				check(arr[0] == 'ben', 'modify arr in function')
				check(n == 5, 'cannot modify num in function')
			}
		}`,
		expected: "",
	},
	{
		name:          "$ in begin",
		prog:          "BEGIN { $.x = 1; print $; }",
		expectedError: "unknown variable $",
	},
	{
		name:          "$ in begin with json",
		prog:          "BEGIN { $.x = 1; print $; }",
		json:          "[1,2,3]",
		expectedError: "unknown variable $",
	},
	{
		name:          "$ in end",
		prog:          "END { $.x = 1 }",
		expectedError: "unknown variable $",
	},
	{
		name:          "$ in end with json",
		prog:          "END { $.x = 1 }",
		json:          "[1,2,3]",
		expectedError: "unknown variable $",
	},
	{
		name:         "$ mutation",
		prog:         "{ $ = $ * 2 }",
		json:         "[1,2,3]",
		expectedJson: "[2,4,6]",
	},
	{
		name:         "$ mutation (nested)",
		prog:         "{ $.x[0].y = 2 }",
		json:         "[{}]",
		expectedJson: `[{"x":[{"y":2}]}]`,
	},
	{
		name:         "$ mutation (via method)",
		prog:         "{ $.items.push(3) }",
		json:         `{"items":[1,2]}`,
		expectedJson: `{"items":[1,2,3]}`,
	},
	{
		name: "bug: statement after block",
		prog: `
			{
				if ($ > 10) {
					print $;
				}
				print "after if";
			}
		`,
		json:     `[2, 12, 87 ,0]`,
		expected: "after if\n12\nafter if\n87\nafter if\nafter if\n",
	},
	{
		name:          "bug: unclosed regexes",
		prog:          "$ ~ /abc",
		json:          "[1]",
		expectedError: "unexpected EOF while reading regex",
	},
	{
		name:          "bug: unclosed strings",
		prog:          "$ ~ 'abc",
		json:          "[1]",
		expectedError: "unexpected EOF while reading string",
	},
	{
		name: "bug: nested return",
		prog: `
			function add_while(a, b) {
				while (true) {
					return a + b;
				}
			}

			function add_for(a, b) {
				for (i = 0; i < 5; i++) {
					return a + b;
				}
			}

			function add_for_in(a, b) {
				for (x in [1, 2, 3]) {
					return a + b;
				}
			}

			function add_if(a, b) {
				if (true) {
					return a + b;
				}
			}

			function add_else(a, b) {
				if (false) {
					return 0;
				} else {
					return a + b;
				}
			}

			BEGIN {
				print add_while(1, 2);
				print add_for(3, 4);
				print add_for_in(5, 6);
				print add_if(7, 8);
				print add_if(9, 10);
			}
		`,
		json:     "[]",
		expected: "3\n7\n11\n15\n19\n",
	},
	{
		name:     "bug: unary precedence",
		prog:     "BEGIN { print -1 + 2; }",
		json:     "[]",
		expected: "1\n",
	},
	{
		name:     "bug: !! precedence",
		prog:     "{ print !!$[0] }",
		json:     "[[0], [1]]",
		expected: "false\ntrue\n",
	},
	{
		name:          "bug: divide by 0",
		prog:          "BEGIN { print 0 / 0 }",
		json:          "[]",
		expectedError: "divide by zero",
	},
	{
		name:          "bug: modulo by 0",
		prog:          "BEGIN { print 0 % 0 }",
		json:          "[]",
		expectedError: "divide by zero",
	},
	{
		name:     "bug: null comparison",
		prog:     "BEGIN { a = []; print a == null }",
		json:     "[]",
		expected: "false\n",
	},
	{
		name:     "bug: create implicit arrays/objects with ++",
		prog:     "BEGIN { a[0]++; ++b['zero']; print a, b }",
		json:     "[]",
		expected: "[1] {\"zero\": 1}\n",
	},
	{
		name:          "bug: return outside of function",
		prog:          "{ return }",
		json:          "[1]",
		expectedError: "can only return inside a function",
	},
	{
		name:          "bug: break outside of loop",
		prog:          "{ break }",
		json:          "[1]",
		expectedError: "can only break inside a loop",
	},
	{
		name:          "bug: continue outside of loop",
		prog:          "{ continue }",
		json:          "[1]",
		expectedError: "can only continue inside a loop",
	},
	{
		name: "bug: pushing arrays to arrays",
		prog: `
			BEGIN { a = [] }
			{ a.push($) }
			END {
				b = []
				for (v in a) {
					b.push([v])
				}
				print b
			}
		`,
		json:     "[1, 2, 3]",
		expected: "[[1], [2], [3]]\n",
	},
	{
		name: "bug: 2d arrays",
		prog: `
			BEGIN {
				a = []
				for (x = 0; x < 3; x++) {
					for (y = 0; y < 3; y++) {
						a[x][y] = 0
					}
				}
				print a
			}
		`,
		expected: "[[0, 0, 0], [0, 0, 0], [0, 0, 0]]\n",
	},
	{
		name:     "bug: range expression end",
		prog:     "BEGIN { print 'abc'[:3] }",
		expected: "abc\n",
	},
	{
		name:     "bug: printf negative %s precision",
		prog:     "BEGIN { printf('%.-2s\n', 'abcde') }",
		expected: "abcde\n",
	},
	{
		name: "bug: chained compound assignment causes syntax error",
		prog: `BEGIN {
			a = 4
			b = 3
			c = 2
			a -= b -= c
			print a, b, c
		}`,
		expected: "3 1 2\n",
	},
	{
		name:          "bug: is accept invalid type names",
		prog:          "BEGIN { print 1 is beep }",
		expectedError: "expected a type name",
	},
}

func FuzzJqawkWithJson(f *testing.F) {
	seedTests := []struct {
		prog string
		json string
	}{
		{`{ print $ }`, `[1,2,3]`},
		{`BEGIN { a = []; a[0]++; print a }`, `[]`},
		{`BEGIN { a = {}; a.x.y = 1; print a }`, `[]`},
		{`{ print $.a, $[0], $["x"] }`, `[{"a":1}, [2], {"x":3}]`},
		{`{ for (x in $) print x }`, `[[1,2], {"a":1}, "abc"]`},
		{`function f(x) { return x + 1 } { print f($) }`, `[1,2,3]`},

		{`{ return }`, `[1]`},
		{`{ break }`, `[1]`},
		{`BEGIN { ++(1 + 2) }`, `[]`},
		{`BEGIN { print '\z' }`, `[]`},
		{`$ ~ /abc`, `[1]`},
	}

	for _, seed := range seedTests {
		f.Add(seed.prog, seed.json)
	}

	for _, tc := range basicTests {
		f.Add(tc.prog, "")
	}

	for _, tc := range tests {
		if len(tc.prog) > 1000 {
			continue
		}
		f.Add(tc.prog, tc.json)
	}

	f.Fuzz(func(t *testing.T, src string, jsonSrc string) {
		inputReader := strings.NewReader(jsonSrc)
		inputFiles := []lang.InputFile{
			lang.NewStreamingInputFile("<test>", inputReader),
		}
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_, err := lang.EvalProgramContext(src, inputFiles, nil, io.Discard, true, ctx)

		if err != nil {
			switch err.(type) {
			case lang.SyntaxError, lang.RuntimeError, lang.JsonError, lang.ErrorGroup:
				return
				// don't fail
			default:
				t.Errorf("%#v", err)
			}
		}
	})
}

func testInternal(t testing.TB, tc testCase) {
	checkError := func(err error) {
		if len(tc.expectedError) > 0 {
			if err == nil {
				t.Fatalf("expected error %q but got none", tc.expectedError)
			}

			if err.Error() != tc.expectedError {
				t.Fatalf("expected error %q\ngot %q\n", tc.expectedError, err.Error())
			}
		} else if err != nil {
			lang.PrintError(err, t.Output())
			t.Fatalf("unexpected error %q\n", err)
		}
	}

	inputFiles := make([]lang.InputFile, 0)
	if tc.json != "" {
		inputReader := strings.NewReader(tc.json)
		inputFiles = append(inputFiles, lang.NewStreamingInputFile("<test1>", inputReader))
	}
	if tc.json2 != "" {
		inputReader := strings.NewReader(tc.json2)
		inputFiles = append(inputFiles, lang.NewStreamingInputFile("<test2>", inputReader))
	}

	var sb strings.Builder
	ev, err := lang.EvalProgram(tc.prog, inputFiles, nil, &sb, false)
	checkError(err)

	if tc.expectedJson != "" {
		j, err := ev.GetUglyRootJson()
		checkError(err)

		if j != tc.expectedJson {
			t.Fatalf("output json %s did not match %s\n", j, tc.expectedJson)
		}
		return
	}

	if sb.String() != tc.expected {
		t.Logf("expected\n")
		for line := range strings.Lines(tc.expected) {
			t.Logf("  \x1b[92m%q\x1b[0m\n", line)
		}

		t.Logf("actual\n")
		for line := range strings.Lines(sb.String()) {
			t.Logf("  \x1b[91m%q\x1b[0m\n", line)
		}

		t.Fatalf("expected does not match actual")
	}
}

func test(t *testing.T, tc testCase) {
	t.Run(tc.name, func(t *testing.T) {
		testInternal(t, tc)
	})
}

func bench(b *testing.B, tc testCase) {
	b.Run(tc.name, func(b *testing.B) {
		for b.Loop() {
			testInternal(b, tc)
		}
	})
}

func testCli(t *testing.T, tc testCase) {
	t.Run(tc.name, func(t *testing.T) {
		stdin := strings.NewReader(tc.json)
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		returnCode := cli.Run("test", tc.args, stdin, &stdout, &stderr, false)

		if len(tc.expectedError) > 0 {
			if stderr.String() != tc.expectedError {
				t.Logf("expected stderr: %q\n", tc.expectedError)
				t.Logf("     got stderr: %q\n", stderr.String())
				t.Logf("         stdout: %q\n", stdout.String())
				t.Fatalf("unexpected error")
			}
		}

		if stdout.String() != tc.expected {
			t.Logf("expected: %q\n", tc.expected)
			t.Logf("     got: %q\n", stdout.String())
			t.Logf("  stderr: %q\n", stderr.String())
			t.Logf("exitcode: %d\n", returnCode)
			t.Fatalf("unexpected result")
		}
	})
}

func TestJqawk(t *testing.T) {
	for _, bt := range basicTests {
		tc := testCase{
			name:     bt.prog,
			prog:     bt.prog,
			expected: bt.expected,
		}
		test(t, tc)
	}

	for _, tc := range tests {
		test(t, tc)
	}
}

type printfTest struct {
	fmt      string
	arg      string
	expected string
}

var printfTests []printfTest = []printfTest{
	{"no fmt", "", "no fmt"},
	{"%%", "", "%"},

	{"%c", "65", "A"},
	{"%3c", "65", "  A"},
	{"%03c", "65", "00A"},

	{"%i", "123", "123"},
	{"%d", "123", "123"},
	{"%d", "123.456", "123"},
	{"%4d", "123.456", " 123"},
	{"%-4d", "123.456", "123 "},
	{"%04d", "123.456", "0123"},

	{"%o", "123", "173"},
	{"%06o", "97", "000141"},
	{"%x", "123", "7b"},

	{"%f", "123.456", "123.456"},
	{"%8f", "123.456", " 123.456"},
	{"%-8f", "123.456", "123.456 "},
	{"%08f", "123.456", "0123.456"},
	{"%08.2f", "123.456", "00123.46"},
	{"%-8.2f", "123.456", "123.46  "},
	{"%20.10f", "123.1234567890123456789", "      123.1234567890"},

	{"%s", "'beep'", "beep"},
	{"%s boop", "'beep'", "beep boop"},
	{"%5s", "'beep'", " beep"},
	{"%5.2s", "'beep'", "   be"},
	{"%-5s", "'beep'", "beep "},
	{"%.3s", "'January'", "Jan"},
	{"%-10.3s", "'January'", "Jan       "},

	{"%v", "'beep'", "beep"},
	{"%v", "123.456", "123.456"},
	{"%v", "[1,2,3]", "[1, 2, 3]"},
	{"%v", "{'a':2}", "{\"a\": 2}"},

	// errors
	{"%", "", "error"},
	{"%10000000000f", "1.23", "error"},
	{"%-10000000000f", "1.23", "error"},
	{"%-10000000000f", "1.23", "error"},
	{"%0.10000000000f", "1.23", "error"},
	{"%0", "1.23", "error"},
	{"%c", "'aaa'", "error"},
	{"%d", "'aaa'", "error"},
	{"%f", "'aaa'", "error"},
	{"%s", "123", "error"},
	{"%v", "", "error"},
	{"%z", "", "error"},
}

func TestJqawkPrintf(t *testing.T) {
	for _, tc := range printfTests {
		t.Run(tc.fmt, func(t *testing.T) {
			prog := fmt.Sprintf("BEGIN { printf('%s', %s) }", tc.fmt, tc.arg)
			var sb strings.Builder
			_, err := lang.EvalProgram(prog, []lang.InputFile{}, nil, &sb, false)
			if tc.expected == "error" {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}

			if err != nil {
				t.Fatalf("error: %v", err)
			}

			if sb.String() != tc.expected {
				t.Fatalf("expected %#v got %#v", tc.expected, sb.String())
			}
		})
	}
}

func FuzzJqawkPrintf(f *testing.F) {
	for _, seed := range printfTests {
		f.Add(seed.fmt)
	}

	f.Fuzz(func(t *testing.T, fmts string) {
		prog := fmt.Sprintf("BEGIN { printf(%q, 123.456) }", fmts)
		var sb strings.Builder
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_, err := lang.EvalProgramContext(prog, []lang.InputFile{}, nil, &sb, false, ctx)
		if err != nil {
			switch err.(type) {
			case lang.SyntaxError, lang.RuntimeError, lang.JsonError, lang.ErrorGroup:
				return
				// don't fail
			default:
				t.Fatalf("%#v", err)
			}
		}
	})
}

func BenchmarkJqawk(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for _, tc := range tests {
		bench(b, tc)
	}
}

func TestJqawkCli(t *testing.T) {
	testCli(t, testCase{
		name:     "root selector",
		args:     []string{"-r", "$.items", "{ print }"},
		json:     `{ "items": [1, 2, 3] }`,
		expected: "1\n2\n3\n",
	})
	testCli(t, testCase{
		name:     "root selector (array)",
		args:     []string{"-r", "$[0]", "{ print }"},
		json:     `[[2, 3], [0, 1]]`,
		expected: "2\n3\n",
	})

	testCli(t, testCase{
		name:     "root selector (multiple)",
		args:     []string{"-r", "$.a", "-r", "$.b", "{ print }"},
		json:     `{ "a": [2, 3], "b": [0, 1] }`,
		expected: "2\n3\n0\n1\n",
	})

	testCli(t, testCase{
		name: "json output",
		args: []string{"-o", "-", "{ $.x++ }"},
		json: `[{ "x": 1 }, { "x": 2 }]`,
		expected: `[
  {
    "x": 2
  },
  {
    "x": 3
  }
]`,
	})

	testCli(t, testCase{
		name:     "no arguments",
		args:     []string{},
		json:     "[]",
		expected: "",
	})

	testCli(t, testCase{
		name:     "expr",
		args:     []string{"-e", "$.length()"},
		json:     "[1, 2, 3]",
		expected: "3\n",
	})

	testCli(t, testCase{
		name:          "json error",
		args:          []string{},
		json:          "[1, 2, 3",
		expectedError: "could not parse <stdin>: unexpected end of JSON input\n",
	})

	testCli(t, testCase{
		name: "syntax error",
		args: []string{"BEGIN { print [1, 2, 3 }"},
		json: "",
		expectedError: `  BEGIN { print [1, 2, 3 }
                         ^
syntax error on line 1: expected ]
  BEGIN { print [1, 2, 3 }
                         ^
syntax error on line 1: unexpected end of input
`,
	})

	testCli(t, testCase{
		name: "runtime error",
		args: []string{"BEGIN { 1[1:2] }"},
		json: "",
		expectedError: `  BEGIN { 1[1:2] }
             ^
runtime error on line 1: cannot slice a number
`,
	})
}

func TestJqawkCliReadWriteFiles(t *testing.T) {
	json := "[1, 2, 3]"
	prog := "{ $ = $ * 2}"
	expected := "[\n  2,\n  4,\n  6\n]"

	defer func() {
		os.Remove("_cli_test.json")
		os.Remove("_cli_test.jqawk")
		os.Remove("_cli_test_output.json")
	}()

	err := os.WriteFile("_cli_test.json", []byte(json), 0666)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile("_cli_test.jqawk", []byte(prog), 0666)
	if err != nil {
		t.Fatal(err)
	}

	var stdin bytes.Buffer
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	args := []string{"-f", "_cli_test.jqawk", "-o", "_cli_test_output.json", "_cli_test.json"}
	returnCode := cli.Run("test", args, &stdin, &stdout, &stderr, false)

	if returnCode != 0 {
		t.Log("jqawk failed")
		t.Logf("stdout: %s\n", stdout.String())
		t.Logf("stderr: %s\n", stderr.String())
		t.Fatal("jqawk failed")
	}

	output, err := os.ReadFile("_cli_test_output.json")
	if err != nil {
		t.Fatal(err)
	}

	if string(output) != expected {
		t.Logf("expected: %s\n", expected)
		t.Logf("     got: %s\n", string(output))
		t.Logf("exitcode: %d\n", returnCode)
		t.Fatal("unexpected result")
	}
}

func TestJqawkStreamingJson(t *testing.T) {
	program := "BEGINFILE { sum = 0 } { sum += $ } ENDFILE { print sum }"

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	var stderr bytes.Buffer

	done := make(chan int, 1)

	go func() {
		exitCode := cli.Run("test", []string{program}, stdinR, stdoutW, &stderr, false)
		stdoutW.Close()
		done <- exitCode
	}()

	out := bufio.NewReader(stdoutR)

	write := func(s string) {
		t.Helper()
		_, err := io.WriteString(stdinW, s)
		if err != nil {
			t.Fatalf("write stdin: %v", err)
		}
	}

	checkLine := func(expected string) {
		t.Helper()
		ch := make(chan string, 1)
		errCh := make(chan error, 1)

		go func() {
			line, err := out.ReadString('\n')
			if err != nil {
				errCh <- err
				return
			}
			ch <- line
		}()

		select {
		case line := <-ch:
			if line != expected {
				t.Fatalf("output line %q did not match %q", line, expected)
			}
		case err := <-errCh:
			t.Fatalf("read stdout: %v", err)
		case <-time.After(time.Second):
			t.Fatal("read stdin timed out")
		}
	}

	write("[1, 2, 3]")
	checkLine("6\n")

	write("[4, 5, 6]")
	checkLine("15\n")

	if err := stdinW.Close(); err != nil {
		t.Fatalf("failed closing stdin: %v", err)
	}

	select {
	case exitCode := <-done:
		if exitCode != 0 {
			t.Fatalf("non-zero exit code: %d", exitCode)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestJqawkInteractive(t *testing.T) {
	input := []string{
		"print $",
		"n = $[1]",
		"print n * 10",
		":mode program",
		"{ print $ }",
	}

	expectedOutputLines := []string{
		"[1, 2, 4]",
		"2",
		"20",
		"current mode: program (run as full program)",
		"1",
		"2",
		"4",
		"",
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	files := []lang.InputFile{
		lang.NewBufferedInputFile("test.json", []byte("[1, 2, 4]")),
	}

	cli.RunRepl(
		"test",
		files,
		[]string{},
		strings.NewReader(strings.Join(input, "\n")),
		&stdout,
		&stderr,
	)

	outputLines := strings.Split(stdout.String(), "\n")

	// skip the header
	outputLines = outputLines[2:]

	if len(expectedOutputLines) != len(outputLines) {
		t.Fatalf("expected %d output lines but got %d\n", len(expectedOutputLines), len(outputLines))
	}

	for i, expected := range expectedOutputLines {
		actual := outputLines[i]
		if actual != expected {
			t.Logf("  actual: %q\n", actual)
			t.Logf("expected: %q\n", expected)
			t.Fatalf("repl output line %d did not match", i)
		}
	}
}

func TestJqawkOneTrueAwk(t *testing.T) {
	countries := `[
		["Russia", 8650, 262, "Asia"],
		["Canada", 3852, 24, "North America"],
		["China", 3692, 866, "Asia"],
		["USA", 3615, 219, "North America"],
		["Brazil", 3286, 116, "South America"],
		["Australia", 2968, 14, "Australia"],
		["India", 1269, 637, "Asia"],
		["Argentina", 1072, 26, "South America"],
		["Sudan", 968, 19, "Africa"],
		["Algeria", 920, 18, "Africa"]
	]`

	test(t, testCase{
		name:     "p1",
		prog:     "{ print }",
		json:     "[1, 2, 3]",
		expected: "1\n2\n3\n",
	})

	test(t, testCase{
		name:     "p2",
		prog:     "{ print $[0], $[2] }",
		json:     "[[1, 2, 3], [10, 20, 30]]",
		expected: "1 3\n10 30\n",
	})

	test(t, testCase{
		name:     "p4",
		prog:     "{ print $index, $ }",
		json:     "[2, 4, 6, 8]",
		expected: "0 2\n1 4\n2 6\n3 8\n",
	})

	test(t, testCase{
		name:     "p7",
		prog:     "$ > 100",
		json:     "[1, 2, 300, 400, 5]",
		expected: "300\n400\n",
	})

	test(t, testCase{
		name:     "p8",
		prog:     "$[3] == 'Asia' { print $[0] }",
		json:     countries,
		expected: "Russia\nChina\nIndia\n",
	})

	test(t, testCase{
		name:     "p9",
		prog:     "$[0] >= 'S' { print $[0] }",
		json:     countries,
		expected: "USA\nSudan\n",
	})

	test(t, testCase{
		name:     "p10",
		prog:     "$[0] == $[3] { print $[0] }",
		json:     countries,
		expected: "Australia\n",
	})

	test(t, testCase{
		name:     "p11",
		prog:     "$[3] ~ /Asia/ { print $[0] }",
		json:     countries,
		expected: "Russia\nChina\nIndia\n",
	})

	test(t, testCase{
		name:     "p13",
		prog:     "$[3] !~ /Asia/ { print $[0] }",
		json:     countries,
		expected: "Canada\nUSA\nBrazil\nAustralia\nArgentina\nSudan\nAlgeria\n",
	})

	test(t, testCase{
		name: "p19",
		prog: `
			BEGIN { digits = "^[0-9]+$" }
			$[1] !~ digits
		`,
		json:     countries,
		expected: "",
	})

	test(t, testCase{
		name:     "p20",
		prog:     "$[3] == 'Asia' && $[2] > 500 { print $[0] }",
		json:     countries,
		expected: "China\nIndia\n",
	})

	test(t, testCase{
		name:     "p21",
		prog:     "$[3] == 'Asia' || $[3] == 'Europe' { print $[0] }",
		json:     countries,
		expected: "Russia\nChina\nIndia\n",
	})

	test(t, testCase{
		name:     "p22",
		prog:     "$[3] ~ /^(Asia|Europe)$/ { print $[0] }",
		json:     countries,
		expected: "Russia\nChina\nIndia\n",
	})

	test(t, testCase{
		name: "p25",
		prog: `{ printf("%10s %6.1f\n", $[0], 1000 * $[2] / $[1]) }`,
		json: countries,
		expected: `    Russia   30.3
    Canada    6.2
     China  234.6
       USA   60.6
    Brazil   35.3
 Australia    4.7
     India  502.0
 Argentina   24.3
     Sudan   19.6
   Algeria   19.6
`,
	})

	test(t, testCase{
		name: "p26",
		prog: `
			$[3] ~ /Asia/ { pop = pop + $[2]; n = n + 1 }
			END { print "population of", n, "Asian countries in millions is", pop }
		`,
		json:     countries,
		expected: "population of 3 Asian countries in millions is 1765\n",
	})

	test(t, testCase{
		name: "p26a",
		prog: `
			$[3] ~ /Asia/ { pop += $[2]; ++n }
			END { print "population of", n, "Asian countries in millions is", pop }
		`,
		json:     countries,
		expected: "population of 3 Asian countries in millions is 1765\n",
	})

	test(t, testCase{
		name: "p27",
		prog: `
			maxpop < $[2] { maxpop = $[2]; country = $[0] }
			END { print country, maxpop }
		`,
		json:     countries,
		expected: "China 866\n",
	})

	test(t, testCase{
		name: "p32",
		prog: "{ $[0] = $[0][:3]; print $[0] }",
		json: countries,
		expected: `Rus
Can
Chi
USA
Bra
Aus
Ind
Arg
Sud
Alg
`,
	})

	test(t, testCase{
		name: "p34",
		prog: "{ $[1] /= 1000; print }",
		json: countries,
		expected: `["Russia", 8.65, 262, "Asia"]
["Canada", 3.852, 24, "North America"]
["China", 3.692, 866, "Asia"]
["USA", 3.615, 219, "North America"]
["Brazil", 3.286, 116, "South America"]
["Australia", 2.968, 14, "Australia"]
["India", 1.269, 637, "Asia"]
["Argentina", 1.072, 26, "South America"]
["Sudan", 0.968, 19, "Africa"]
["Algeria", 0.92, 18, "Africa"]
`,
	})

	test(t, testCase{
		name: "p35",
		prog: `
			$[3] ~ /^North America$/ { $[3] = 'NA' }
			$[3] ~ /^South America$/ { $[3] = 'SA' }
			{ print }
		`,
		json: countries,
		expected: `["Russia", 8650, 262, "Asia"]
["Canada", 3852, 24, "NA"]
["China", 3692, 866, "Asia"]
["USA", 3615, 219, "NA"]
["Brazil", 3286, 116, "SA"]
["Australia", 2968, 14, "Australia"]
["India", 1269, 637, "Asia"]
["Argentina", 1072, 26, "SA"]
["Sudan", 968, 19, "Africa"]
["Algeria", 920, 18, "Africa"]
`,
	})

	test(t, testCase{
		name: "p36",
		prog: "{ $[4] = 1000 * $[2] / $[1]; print $[0], $[1], $[2], $[3], $[4] }",
		json: countries,
		expected: `Russia 8650 262 Asia 30.289017341040463
Canada 3852 24 North America 6.230529595015576
China 3692 866 Asia 234.56121343445287
USA 3615 219 North America 60.58091286307054
Brazil 3286 116 South America 35.30127814972611
Australia 2968 14 Australia 4.716981132075472
India 1269 637 Asia 501.9700551615445
Argentina 1072 26 South America 24.253731343283583
Sudan 968 19 Africa 19.628099173553718
Algeria 920 18 Africa 19.565217391304348
`,
	})

	test(t, testCase{
		name: "p36",
		prog: `
			{
				if (maxpop < $[2]) {
					maxpop = $[2]
					country = $[0]
				}
			}
			END { print country, maxpop }
		`,
		json:     countries,
		expected: "China 866\n",
	})

	test(t, testCase{
		name: "p40",
		prog: `
			{
				for (i = 0; i < $.length(); i++) {
					print $[i]
				}
			}
		`,
		json: countries,
		expected: `Russia
8650
262
Asia
Canada
3852
24
North America
China
3692
866
Asia
USA
3615
219
North America
Brazil
3286
116
South America
Australia
2968
14
Australia
India
1269
637
Asia
Argentina
1072
26
South America
Sudan
968
19
Africa
Algeria
920
18
Africa
`,
	})

	test(t, testCase{
		name: "p41",
		prog: `
			{ n++ }
			n >= 10 { exit }
			END { print n }
		`,
		json:     countries,
		expected: "10\n",
	})

	test(t, testCase{
		name: "p42",
		prog: `
			$[3] ~ /Asia/   { pop["Asia"] += $[2] }
			$[3] ~ /Africa/ { pop["Africa"] += $[2] }
			END {
				print "Asian population in millions is", pop["Asia"]
				print "African population in millions is", pop["Africa"]
			}
		`,
		json:     countries,
		expected: "Asian population in millions is 1765\nAfrican population in millions is 37\n",
	})

	test(t, testCase{
		name: "p43",
		prog: `
			{ area[$[3]] += $[1] }
			END {
				for (name in area)
					print name + ":" + area[name]
			}
		`,
		json: countries,
		expected: `Asia:13611
North America:7467
South America:4358
Australia:2968
Africa:1888
`,
	})

	test(t, testCase{
		name: "p44",
		prog: `
			function fact(n) {
				if (n <= 1)
					return 1
				else
					return n * fact(n-1)
			}

			BEGIN {
				print fact(5)
				print fact(10)
			}
		`,
		json:     countries,
		expected: "120\n3628800\n",
	})

	test(t, testCase{
		name: "t.incr",
		prog: `
			{ ++i; --j; k++; l-- }
			ENDFILE { print $.length(), i, j, k, l }
		`,
		json:     "[1, 2, 3, 4, 5]",
		expected: "5 5 -5 5 -5\n",
	})
}
