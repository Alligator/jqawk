# jqawk

AWK for JSON. This is very early and unfinished.

## what is this

Suppose you have some JSON

```json
[
  { "name": "Beth", "rate": 4, "hours": 0 },
  { "name": "Dan", "rate": 3.75, "hours": 0 },
  { "name": "Kathy", "rate": 4, "hours": 10 },
  { "name": "Mark", "rate": 5, "hours": 20 },
  { "name": "Mary", "rate": 5.50, "hours": 22 },
  { "name": "Susie", "rate": 4.25, "hours": 18 }
]
```

Now you want to print the name and pay (rate times hours) of everyone who worked any hours. You can do that with jqawk:

```shell
$ jqawk "$.hours > 0 { print $.name, $.rate * $.hours }" file.json
Kathy 40
Mark 100
Mary 121
Susie 76.5
```

If you know awk, this should look familiar. Like awk, a jqawk program is a series of rules. Each rule has a pattern and an action. The pattern above is `$.hours > 0` and the action is `{ print $.name, $.rate * $.hours }`.

It supports `BEGIN` and `END` patterns, and variables. This program demonstrates both:

```awk
BEGIN { print 'Pay'; print '------' }
$.hours > 0 {
  print $.name, $.rate * $.hours;
  total += $.rate * $.hours;
}
END { print '------'; print 'Total', total }
```

The `-f` option runs a program from a file:

```shell
$ jqawk -f prog.jqawk file.json
Pay
------
Kathy 40
Mark 100
Mary 121
Susie 76.5
------
Total 337.5
```

## TODO

- good syntax/runtime errors
- regex matching
- if statements
- for loops
- match statements with pattern matching
- in place modification à la `sed -i`
  - `jqawk -i { $.version += 1 }`