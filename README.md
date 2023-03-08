# jqawk

jqawk is an awk-inspired programming language for wrangling JSON.

See below for some examples, or the test suite in [jqawk_test.go](jqawk_test.go) for more.

## Quick guide

Here's an example command-line:

```shellsession
$ jqawk "$.hours > 15 { print $.name }" emp.json
```

If the content of `emp.json` is:

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

This will print

```
Mark
Mary
Susie
```

The first argument is the program, the second is the JSON file. JSON can be read from stdin too.

The program is a series of *rules*, which are made up of a *pattern* and a *body*.
The pattern `$.hours > 15` matches every item in the input where the `hours` property is > 15.
The body `{ print $.name }` prints the `name` property.

If you know awk, this should be familiar.

If the input is an array, each rule will run for each item in the array, with `$` set to that item. If the input is any other type, each rule is run once with `$` set to the input.

Here's a more complex program that calculates the pay (rate times hours) of each person:

```awk
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
```

`BEGIN` and `END` are special rules that run before and after the input is processed.

A more complex program like this is better placed in a file. It can be run with the `-f` flag:

```shellsession
$ jqawk -f prog.jqawk file.json
Pay
----------------
Kathy    40
Mark     100
Mary     121
Susie    76.5
----------------
Total    337.5
```

## Selecting a root value

If the input value you want isn't at the root of the JSON, you can use the `-r` flag to select it.

With this JSON:

```json
{
  "status": "success",
  "result": [
    { "name": "alligator" },
    { "name": "someone else" }
  ]
}
```

This command selects the `result` array as the root value:

```console
$ jqawk -r "$.result" "{ print $.name }" file.json
alligator
someone else
```

The argument to `-r` can be any valid expression.

## Control flow

jqawk provides `if`, `else`, `for`, and `while` control-flow statements:

```awk
BEGIN {
  columns = ['name', 'rate', 'hours']
  rows = []
}

{
  if ($.name != 'Dan') {
    rows.push([$.name, $.rate, $.hours])
  }
}

END {
  # print column headers
  for (col in columns) {
    printf('%-10s ', col)
  }

  # print divider
  printf('\n')
  i = 0
  while (i < 32) {
    printf('-')
    i++
  }
  printf('\n')

  # print rows
  for (row in rows) {
    printf('%-10s %-10f %-10f\n', row[0], row[1], row[2])
  }
  
  # print footer
  for (i = 0; i < 32; i++) {
    printf('-')
  }
  printf('\n')
}
```

```shellsession
$ jqawk -f prog.jqawk emp.json
name       rate       hours
--------------------------------
Beth       4          0
Kathy      4          10
Mark       5          20
Mary       5.5        22
Susie      4.25       18
```

## Regex

Regex literals can be matched with the `~` operator:

```shellsession
$ jqawk "$.name ~ /^M/ { print $.name }" emp.json
Mark
Mary
```

## Functions

Functions can be defined along with rules:

```awk
function pay(rate, hours) {
  return rate * hours
}

{
  print $.name, pay($.rate, $.hours)
}
```

```shellsession
$ jqawk -f prog.jqawk emp.json
Beth 0
Dan 0
Kathy 40
Mark 100
Mary 121
Susie 76.5
```

## Modifying the JSON

Modifying `$` will modify the JSON. The `-o` flag directs the JSON output to a file, or `-` for stdout:

```shellsession
$ cat example.json
[
  { "name": "alligator" },
  { "name": "someone else" }
]

$ jqawk -o - "{ $.name_length = $.name.length() }" example.json
[
  {
    "name": "alligator",
    "name_length": 9
  },
  {
    "name": "someone else",
    "name_length": 12
  }
]
```

## Language reference

A rough jqawk language reference.

### Operators

```
!     unary not
&&    logical and
||    logical or

==    equal
!=    not equal
<     less
<=    less or equal
>     greater
>=    greater or equal

+     add
-     subtract
/     divide
*     multiply
%     modulo

++    pre/postfix increment
--    pre/postfix decrement

~     regex match
!~    regex not match

=     assign

a[x]  index
a.x   property
```

### Literals

```
123               number
"hello"           string
'hello'           string
true              bool
false             bool
[1, 2]            array
{ a: 1, b: '2' }  object
/hello/           regex
```

### Rules

```
pattern { body }    runs when pattern is truthy
BEGIN { body }      runs before the input is processed
END { body}         runs after the input
```

If the input is an array, each pattern rule runs for each item in the array.

If the input is not an array, each pattern rule runs once.

### `$` Variables

If the input is an array, jqawk sets two variables:

- `$` is the current item in the array
- `$index` is the array index of that item

If the input is not an array, `$` is the input value in all rules.

### Statements

```
print <expression>, <expression>, ...
  print <expression>s, separated by spaces

return <expression>
  return from the current function, optionally returning the value of

break
  exit the enclosing for or while loop

continue
  immediately begin the next iteration of the loop

next
  immediately exit the rule and process no further rules for the current item

if (<expression>) <body> else <elsebody>
  execute <body> if <expression> is truthy. If the value is falsy and the else
  is present, execute <elsebody>

while (<expression>) <body>
  execute the body until <expression> is falsy

for (<preexpression>; <checkexpression>; <postexpression>) <body>
  execute <preexpression>, then execute { <body>; <postexpression> } until
  <checkexpression> is falsy

for (<identifier> in <expression>) <body>
  execute <body> for each item in <expression> with <identifier> set to it's
  value. if <expression> is an array, it's each item. for objects, it's each
  key. for strings, each character

for (<identifier>, <indexidentifier> in <expression>) <body>
  same as above, with <indexindeitifer> set to the index of the item for arrays
  or the value for objects
```

### Built-in functions

```
printf(format_string, args...)
  printf. supports %s and %f format codes, and width specifiers

json(arg)
  return arg converted to a pretty-printed JSON string

int(arg)
  convert arg to a number. only works on strings
```

### Methods

```
string.length()           return the length
string.split(separator)   split the string into substrings on a separator

object.length()           return the number of keys

array.length()            return the length
array.push(value)         push value to the end of the array
array.pop()               remove and return the last value in the array
array.popfirst()          remove and return the first value in the array
array.contains(value)     return true if value is in the array
```
