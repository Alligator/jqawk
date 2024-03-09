package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	lang "github.com/alligator/jqawk/src"
)

type testCase struct {
	name          string
	prog          string
	json          string
	json2         string
	expected      string
	expectedError string
	args          []string
}

var tests []testCase = []testCase{
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
		name:     "begin",
		prog:     "BEGIN { print 'hello' } BEGIN { print 'other hello' }",
		expected: "hello\nother hello\n",
	},
	{
		name:     "comments",
		prog:     "BEGIN { print 'hello' } # prints hello\n# goodbye",
		expected: "hello\n",
	},
	{
		name: "operators",
		prog: `BEGIN {
			print 2 + 3
			print 2 - 3
			print 2 * 3
			print 6 / 3
			print 6 / 2 - 1 * 3

			obj = { a: 1 }
			print !obj.a
			print -obj.a + 2
			print -(obj.a + 2)
		}`,
		expected: "5\n-1\n6\n2\n0\nfalse\n1\n-3\n",
	},
	{
		name: "pre/postfix operators",
		prog: `BEGIN {
			for (i = 0; i < 4; i++) {
				print a++, b--, b--, ++c, --d, --d;
			}
		}`,
		expected: "0 0 -1 1 -1 -2\n1 -2 -3 2 -3 -4\n2 -4 -5 3 -5 -6\n3 -6 -7 4 -7 -8\n",
	},
	{
		name: "compound operators",
		prog: `
		BEGIN {
			prod = 1;
			div = 8;
			sub = 16
		}

		{
			sum += $;
			prod *= $;
		}

		$ > 3 {
			div /= $;
			sub -= $;
		}

		END {
			print sum;
			print prod;
			print div;
			print sub;
		}`,
		json:     "[2, 3, 4]",
		expected: "9\n24\n2\n12\n",
	},
	{
		name: "unary operators",
		prog: `BEGIN {
			print !0, !1
		}`,
		expected: "true false\n",
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
		name: "printf",
		prog: `
		{
			printf('name: %s\nage: %f\n', $.name, $.age)
			printf('string lpad: %10s %1s\n', $.name, $.name)
			printf('string rpad: %-10s %-1s\n', $.name, $.name)
			printf(' float lpad: %6f %06f\n', $.age, $.age)
		}`,
		json: `[{ "name": "gate", "age": 1 }, { "name": "sponge", "age": 2.300 }]`,
		expected: `name: gate
age: 1
string lpad:       gate gate
string rpad: gate       gate
 float lpad:      1 000001
name: sponge
age: 2.3
string lpad:     sponge sponge
string rpad: sponge     sponge
 float lpad:    2.3 0002.3
`,
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

			BEGIN {
				print add(3, 4);
				log("hello");
				empty_return(5, 6);
			}
		`,
		json:     "[]",
		expected: "7\nhello\n5\n",
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
		prog:     "{ printf('%s\n', $.name) }",
		json:     `{ "name": "alligator" }`,
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
				for (i = 0; i < 5; i += 1) {
					print i;
				}
			}
		`,
		json:     "[]",
		expected: "0\n1\n2\n3\n4\n",
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
		`,
		json:     "[[1, 2], [3, 4]]",
		expected: "1\n2\n0 1\n1 2\n3\n4\n0 3\n1 4\n",
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
		json:     `{ "a": 1 }`,
		expected: "a 1\n",
	},
	{
		name: "match",
		prog: `
			{
				print match ($) {
					1 => 'one',
					2 => 'two',
					_ => '?',
				}
			}
		`,
		json:     "[1, 2, 3, 4]",
		expected: "one\ntwo\n?\n?\n",
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
		name:     "next",
		prog:     "{ print $; next } { print $ }",
		json:     "[1, 2, 3, 4]",
		expected: "1\n2\n3\n4\n",
	},
	{
		name:     "printing circular references",
		prog:     "BEGIN { a.a=a; print a; b = []; b[0] = 1; b[1] = b; print b; }",
		json:     "[]",
		expected: "{\"a\": <circular reference>}\n[1, <circular reference>]\n",
	},
	{
		name:          "converting circular references to JSON",
		prog:          "BEGIN { a.a=a; print json(a) }",
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
			}
		`,
		json: "[]",
		expected: `ABC
abc
["a", "c"]
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
		prog:     "{ print $; exit }",
		json:     "[1, 2]",
		expected: "1\n",
	},
	{
		name: "null comparison",
		prog: `
			$.a == null { print 'lhs null' }
			$.a != null { print 'lhs not null' }

			null == $.a { print 'rhs null' }
			null != $.a { print 'rhs not null' }
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
		prog:     "BEGIN { print !![], !!{} }",
		json:     "[]",
		expected: "true true\n",
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
		json:          "[]",
		expectedError: "unexpected EOF while reading regex",
	},
	{
		name:          "bug: unclosed strings",
		prog:          "$ ~ 'abc",
		json:          "[]",
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
		prog:          "{ print 0 / 0 }",
		json:          "[]",
		expectedError: "divide by zero",
	},
	{
		name:          "bug: modulo by 0",
		prog:          "{ print 0 % 0 }",
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
}

func TestMain(m *testing.M) {
	cmd := exec.Command("go", "build")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building: %v\n%s\n", err, output)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func FuzzJqawk(f *testing.F) {
	for _, tc := range tests {
		if tc.expectedError == "" {
			f.Add(tc.prog)
		}
	}

	f.Fuzz(func(t *testing.T, src string) {
		input := "[{ \"a\": 1 }, { \"a\": null }]"
		inputReader := strings.NewReader(input)
		inputFiles := []lang.InputFile{
			{Name: "<test>", Reader: inputReader},
		}
		var sb strings.Builder
		_, err := lang.EvalProgram(src, inputFiles, "", &sb, true)

		if err != nil {
			switch err.(type) {
			case lang.SyntaxError, lang.RuntimeError:
				// don't fail
			default:
				t.Errorf("%#v", err)
			}
		}
	})
}

func test(t *testing.T, tc testCase) {
	t.Run(tc.name, func(t *testing.T) {
		handleError := func(err error) {
			if len(tc.expectedError) > 0 {
				if err.Error() != tc.expectedError {
					t.Fatalf("expected error %q\ngot %q\n", tc.expectedError, err.Error())
				}
			} else {
				panic(err)
			}
		}

		inputFiles := make([]lang.InputFile, 0)
		if tc.json != "" {
			inputReader := strings.NewReader(tc.json)
			inputFiles = append(inputFiles, lang.InputFile{Name: "<test1>", Reader: inputReader})
		}
		if tc.json2 != "" {
			inputReader := strings.NewReader(tc.json2)
			inputFiles = append(inputFiles, lang.InputFile{Name: "<test2>", Reader: inputReader})
		}

		var sb strings.Builder
		_, err := lang.EvalProgram(tc.prog, inputFiles, "", &sb, false)
		if err != nil {
			handleError(err)
		}

		if sb.String() != tc.expected {
			actualLines := strings.Split(sb.String(), "\n")
			expectedLines := strings.Split(tc.expected, "\n")
			for i, line := range actualLines {
				if len(expectedLines) < i {
					fmt.Printf("\x1b[92m+ %s\x1b[0m\n", line)
				} else if len(expectedLines) > i && line != expectedLines[i] {
					fmt.Printf("\x1b[91m- %s\x1b[0m\n", expectedLines[i])
					fmt.Printf("\x1b[92m+ %s\x1b[0m\n", line)
				} else {
					fmt.Printf("  %s\n", line)
				}
			}
			t.Fatalf("unexpected result")
		}
	})
}

func testExe(t *testing.T, tc testCase) {
	t.Run(tc.name, func(t *testing.T) {
		cmd := exec.Command("./jqawk", tc.args...)
		rdr := strings.NewReader(tc.json)
		cmd.Stdin = rdr
		output, err := cmd.Output()
		if err != nil {
			panic(err)
		}
		if string(output) != tc.expected {
			t.Fatalf("expected %q\ngot %q\n", tc.expected, string(output))
		}
	})
}

func TestJqawk(t *testing.T) {
	for _, tc := range tests {
		test(t, tc)
	}
}

func TestJqawkExe(t *testing.T) {
	testExe(t, testCase{
		name:     "root selector",
		args:     []string{"-r", "$.items", "{ print }"},
		json:     `{ "items": [1, 2, 3] }`,
		expected: "1\n2\n3\n",
	})

	testExe(t, testCase{
		name:     "root selector (array)",
		args:     []string{"-r", "$[0]", "{ print }"},
		json:     `[[2, 3], [0, 1]]`,
		expected: "2\n3\n",
	})

	testExe(t, testCase{
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
}
