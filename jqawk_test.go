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
		}`,
		expected: "5\n-1\n6\n2\n",
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
		prog:     "{ printf('%s: %s\n', $key, $) }",
		json:     `{ "name": "alligator", "country": "uk" }`,
		expected: "name: alligator\ncountry: uk\n",
	})

	test(t, testCase{
		name:     "root number",
		prog:     "{ print }",
		json:     `45.67`,
		expected: "45.67\n",
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