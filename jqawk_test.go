package main

import (
	"encoding/json"
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
	expected      string
	expectedError string
	args          []string
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

		lex := lang.NewLexer(tc.prog)
		parser := lang.NewParser(&lex)

		rules, err := parser.Parse()
		if err != nil {
			handleError(err)
		}

		var sb strings.Builder
		ev := lang.NewEvaluator(rules, &lex, &sb)

		var j interface{}
		rootValue := lang.NewValue([]interface{}{})
		if len(tc.json) > 0 {
			err = json.Unmarshal([]byte(tc.json), &j)
			if err != nil {
				panic(err)
			}
			rootValue = lang.NewValue(j)
		}

		err = ev.Eval(lang.NewCell(rootValue))
		if err != nil {
			handleError(err)
		}

		if sb.String() != tc.expected {
			t.Fatalf("expected %q\ngot %q\n", tc.expected, sb.String())
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
	test(t, testCase{
		name:     "begin",
		prog:     "BEGIN { print 'hello' } BEGIN { print 'other hello' }",
		expected: "hello\nother hello\n",
	})

	test(t, testCase{
		name:     "comments",
		prog:     "BEGIN { print 'hello' } # prints hello\n# goodbye",
		expected: "hello\n",
	})

	test(t, testCase{
		name: "operators",
		prog: `BEGIN {
			print 2 + 3;
			print 2 - 3;
			print 2 * 3;
			print 6 / 3;
			print 6 / 2 - 1 * 3;
		}`,
		expected: "5\n-1\n6\n2\n0\n",
	})

	test(t, testCase{
		name: "postfix operators",
		prog: `BEGIN {
			for (i = 0; i < 4; i++) {
				print a++, b--, b--, ++c, --d, --d;
			}
		}`,
		expected: "0 0 -1 1 -1 -2\n1 -2 -3 2 -3 -4\n2 -4 -5 3 -5 -6\n3 -6 -7 4 -7 -8\n",
	})

	test(t, testCase{
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
	})

	test(t, testCase{
		name: "unary operators",
		prog: `BEGIN {
			print !0, !1
		}`,
		expected: "true false\n",
	})

	test(t, testCase{
		name:     "dot",
		prog:     "{ print $.name }",
		json:     `[{ "name": "gate" }, { "name": "sponge" }]`,
		expected: "gate\nsponge\n",
	})

	test(t, testCase{
		name:     "subscript",
		prog:     "{ print $['name'] }",
		json:     `[{ "name": "gate" }, { "name": "sponge" }]`,
		expected: "gate\nsponge\n",
	})

	test(t, testCase{
		name:     "subscript array",
		prog:     "{ print $[0] }",
		json:     `[[1, 2], [10, 20], [100, 200]]`,
		expected: "1\n10\n100\n",
	})

	test(t, testCase{
		name:     "unknown variable comparison",
		prog:     "$ > max { max = $ } $ < min { min = $ } END { print min, max }",
		json:     `[1, 2, 3, 4, 3, 2, 1]`,
		expected: "1 4\n",
	})

	test(t, testCase{
		name:     "semicolon statement separator",
		prog:     "{ print 'a'; print 'b' }",
		json:     `[1]`,
		expected: "a\nb\n",
	})

	test(t, testCase{
		name:     "pretty print",
		prog:     "{ print }",
		json:     `[[1, 2], { "name": "alligator" }]`,
		expected: "[1, 2]\n{\"name\": \"alligator\"}\n",
	})

	test(t, testCase{
		name:     "string concatenation",
		prog:     "{ print 'name: ' + $.name, 'age: ' + $.age }",
		json:     `[{ "name": "gate", "age": 1 }, { "name": "sponge", "age": 2 }]`,
		expected: "name: gate age: 1\nname: sponge age: 2\n",
	})

	test(t, testCase{
		name:     "printf",
		prog:     "{ printf('name: %s\\nage: %f\\n', $.name, $.age) }",
		json:     `[{ "name": "gate", "age": 1 }, { "name": "sponge", "age": 2.300 }]`,
		expected: "name: gate\nage: 1\nname: sponge\nage: 2.3\n",
	})

	test(t, testCase{
		name: "equal, not equal",
		prog: `
			$.name == 'gate' { print 'eq', $.name }
			$.name != 'gate' { print 'neq', $.name }
		`,
		json:     `[{ "name": "gate", "age": 1 }, { "name": "sponge", "age": 2.300 }]`,
		expected: "eq gate\nneq sponge\n",
	})

	test(t, testCase{
		name: "match, not match",
		prog: `
			$.name ~ 'gate' { print 'eq', $.name }
			$.name !~ 'gate' { print 'neq', $.name }
		`,
		json:     `[{ "name": "gate", "age": 1 }, { "name": "sponge", "age": 2.300 }]`,
		expected: "eq gate\nneq sponge\n",
	})

	test(t, testCase{
		name:     "order of operations",
		prog:     "$.age + 1 > 2 { print $.name }",
		json:     `[{ "name": "gate", "age": 1 }, { "name": "sponge", "age": 2.300 }]`,
		expected: "sponge\n",
	})

	test(t, testCase{
		name: "functions",
		prog: `
			function add(a, b) {
				return a + b;
			}

			function log(a) { print a }

			BEGIN {
				print add(3, 4);
				log("hello");
			}
		`,
		json:     "[]",
		expected: "7\nhello\n",
	})

	test(t, testCase{
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
	})

	test(t, testCase{
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
	})

	test(t, testCase{
		name:     "root object",
		prog:     "{ printf('%s\n', $.name) }",
		json:     `{ "name": "alligator" }`,
		expected: "alligator\n",
	})

	test(t, testCase{
		name:     "root number",
		prog:     "{ print }",
		json:     `45.67`,
		expected: "45.67\n",
	})

	test(t, testCase{
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
	})

	test(t, testCase{
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
	})

	test(t, testCase{
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
	})

	test(t, testCase{
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
	})

	test(t, testCase{
		name:     "length methods",
		prog:     "{ print $.obj.length(), $.array.length(); }",
		json:     `[{ "obj": { "key1": 1, "key2": 2 }, "array": [1, 2, 3, 4] }]`,
		expected: "2 4\n",
	})

	test(t, testCase{
		name:     "implicit object creation",
		prog:     "BEGIN { new_obj.name = 'hi'; print new_obj.name; }",
		json:     `[]`,
		expected: "hi\n",
	})

	test(t, testCase{
		name:     "groupings",
		prog:     "BEGIN { print (1 + 2) * 3; }",
		json:     "[]",
		expected: "9\n",
	})

	test(t, testCase{
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
	})

	test(t, testCase{
		name: "object literal",
		prog: `
			BEGIN {
				x = { a: 1, 'b': '2' };
				print x.a, x.b;
			}
		`,
		json:     "[]",
		expected: "1 2\n",
	})

	test(t, testCase{
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
	})

	test(t, testCase{
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
	})

	test(t, testCase{
		name:     "next",
		prog:     "{ print $; next } { print $ }",
		json:     "[1, 2, 3, 4]",
		expected: "1\n2\n3\n4\n",
	})

	test(t, testCase{
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
	})

	test(t, testCase{
		name:          "bug: unclosed regexes",
		prog:          "$ ~ /abc",
		json:          "[]",
		expectedError: "unexpected EOF while reading regex",
	})

	test(t, testCase{
		name:          "bug: unclosed strings",
		prog:          "$ ~ 'abc",
		json:          "[]",
		expectedError: "unexpected EOF while reading string",
	})

	test(t, testCase{
		name:          "bug: $ in BEGIN",
		prog:          "BEGIN { print $ }",
		json:          "[]",
		expectedError: "unknown variable $",
	})

	test(t, testCase{
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
	})

	test(t, testCase{
		name:     "bug: unary precedence",
		prog:     "BEGIN { print 1 == -1 + 2; }",
		json:     "[]",
		expected: "true\n",
	})

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
