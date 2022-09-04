package main

import (
	"strings"
	"testing"

	lang "github.com/alligator/jqawk/src"
)

type testCase struct {
	name     string
	prog     string
	json     string
	expected string
}

func test(t *testing.T, tc testCase) {
	t.Run(tc.name, func(t *testing.T) {
		lex := lang.NewLexer(tc.prog)
		parser := lang.NewParser(&lex)
		rules, err := parser.Parse()
		if err != nil {
			panic(err)
		}
		var sb strings.Builder
		var ev lang.Evaluator
		if tc.json == "" {
			ev = lang.NewEvaluator(rules, &lex, &sb, nil)
		} else {
			ev = lang.NewEvaluator(rules, &lex, &sb, strings.NewReader(tc.json))
		}
		err = ev.Eval()
		if err != nil {
			panic(err)
		}

		if sb.String() != tc.expected {
			t.Fatalf("expected %q\ngot %q\n", tc.expected, sb.String())
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

	// onetrueawk tests
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
}
