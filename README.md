# jqawk

jqawk is an awk-inspired programming language for wrangling JSON.

## Examples

In these examples, `gh.json` contains the response from https://api.github.com/users/alligator/repos

```shell
# Count the repos
jqawk "{ count++ } END { print count }" gh.json

# Find the most used language
jqawk "$.language != null { langs[$.language]++ } END { for (k, v in langs) print v, k }" gh.json | sort

# Find repos with open issues
jqawk "$.open_issues_count > 0 { print $.name, $.open_issues_count }" gh.json

# Find the year the most repos were created
jqawk "{ years[$.created_at.split('-')[0]]++ } END { for (k, v in years) print v, k }" gh.json | sort -n | tail -1

# Find the biggest repo
jqawk "$.size > max.size { max = $ } END { print max.name }" gh.json

# Remove all but the id and name properties, writing the result to stdout
jqawk -o - "{ $ = $.pluck('id', 'name') }" gh.json
```

While you can write full programs in jqawk, it's most useful for one-liners or in a pipeline.

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

If the input is an array, each rule will run for each item in the array, with `$` set to that item. If the input is an object, each rule will run for each key/value pair in the object, with `$` set to the value, and `$key` set to the value.

If the input is any other type, each rule is run once with `$` set to the input.

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

The argument to `-r` can be any valid expression. The flag can also be provided multiple times, in which case each root selector is run for each file.

> [!TIP]
> If you need a more complicated root selector, you can use a `BEGINFILE` rule and reassign `$`. This command is equivalent to the one above:
> ```console
> $ jqawk "BEGINFILE { $ = $.result } { print $.name }" file.json
> alligator
> someone else
> ```

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
--------------------------------
```

## Match expressions

With a match expression you can test a variable against a set of values:

```awk
BEGIN {
  a = 10
  print match(a) {
    10 => 'ten',
    20 => 'twenty',
  }

  # the body can be a block
  match(a) {
    100, 200, 300 => {
      print 'one, two or three hundred'
    },
    a => {
      print 'default case', a
    },
  }

  # arrays can also be destructured
  b = [1, 2, 3]
  match(b) {
    [1, x, y] => {
      print x
      print y
    }
  }
}```

```shellsessions
$ jqawk -f match.jqawk
ten
default case 10
2
3
```

## Regex

Regex literals can be matched against with the `~` operator:

```shellsession
$ jqawk "$.name ~ /^M/ { print $.name }" emp.json
Mark
Mary
```

Use `!~` to invert the match.

## Functions

Functions can be defined both inside and outside of rules:

```awk
function pay(rate, hours) {
  return rate * hours
}

{
  function get_name(emp) {
    return emp.name
  }

  print get_name($), pay($.rate, $.hours)
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

They are also first-class, and can be assigned to variables and passed as parameters:

```awk
BEGIN {
  myfunc = function() {
    print 'hello'
  }

  myfunc()
}
```

## `$` Variables

If the input is an array, jqawk sets two variables:

- `$` is the current item in the array
- `$index` is the array index of that item

and, if the input is an object:

- `$` is the value of the current key/value pair
- `$key` is the key

## Multiple files

You can pass jqawk multiple JSON files on the command line.
The `BEGIN` and `END` rules are run before and after processing all files.
The `BEGINFILE` and `ENDFILE` rules are run at the start and end of processing each file.
All other rules run for each value in the file.

`$file` is set to the current file name.

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

## Interactive REPL

The `-i` flag will start the interactive REPL. The REPL can only be run with files, not JSON read from stdin.

## Language reference

A rough jqawk language reference.

### Operators

```
!      unary not
&&     logical and
||     logical or

==     equal
!=     not equal
<      less
<=     less or equal
>      greater
>=     greater or equal
is     is of type. value type names are:
       string, bool, number, array, object, function, regex, unknown

+      add
-      subtract
/      divide
*      multiply
%      modulo

++     pre/postfix increment
--     pre/postfix decrement

~      regex match
!~     regex not match

=      assign

a[x]   index
a[-x]  index backwards from the end of the array
a[x:y] slice
a.x    property
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
null              null
```

### Rules

```
pattern { body }    runs when pattern is truthy
BEGIN { body }      runs before the input is processed
END { body}         runs after the input is processed
BEGINFILE { body }  runs before each file is processed
ENDFILE { body}     runs after each file is processed
```

If the input is an array, each pattern rule runs for each item in the array.

If the input is not an array, each pattern rule runs once.

### Statements

```
print <expression>, <expression>, ...
  print <expression>s, separated by spaces

return <expression>
  return from the current function, optionally returning the value of <expression>

break
  exit the enclosing for or while loop

continue
  immediately begin the next iteration of the enclosing loop

next
  immediately exit the rule and process no further rules for the current item

exit
  immediately exit the program

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
  value.
  
  if <expression> is an array, <identifier> is each item.
  if it's an object, <identifier> is each key.
  if it's a string, <identifier> is each character.

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

num(arg)
  convert arg to a number (64-bit float). only works on strings
```

### Methods

```
string.length()
  return the length

string.upper()
  return an uppercase copy of the string

string.lower()
  return a lowercase copy of the string

string.split(separator)
  split the string into substrings on a separator. the separator may be a
  string or a regex literal

strings.trim()
  return the string with leading and trailing whitespace removed


number.floor()
number.ceil()
number.round()
  floor, ceil or round the given number. halves are rounded away from zero

number.abs()
  return tbe absolute value of the number

number.mod(n)
  return the number wrapped into the range [0, n]. unlike the modulo
  operator, the result is never negative

number.format(thousandsSeparator, decimalSeparator)
  format a number with thousands and decimal separators. if no separators
  are given ',' is used for thousands and '.' is used for decimals


object.length()
  return the number of keys

object.pluck(k1, k2, ...)
  return a shallow copy of the object containing only the given keys


array.length()
  return the length

array.push(value)
  push value to the end of the array

array.pop()
  remove and return the last value in the array

array.popfirst()
  remove and return the first value in the array

array.contains(value)
  return true if value is in the array

array.sort(fn)
  return a sorted copy of the array.

  with no fn given the array is sorted by string value or numerically if all
  the values are nunbers.

  with fn(a, b) provided, the array is sorted according to fn's return value:
    -1 means a < b
     0 means a == b
     1 means a > b
```
