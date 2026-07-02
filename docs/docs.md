% JQAWK(1)

# NAME
jqawk - awk + JSON

# SYNOPSIS
`jqawk [flags] <program> <file>...`

# OPTIONS
**`-e`** `EXPRESSION`
: Evaluate `EXPRESSION` and print the result.
**`$`** is set to the root value of the input file.

**`-f`** `FILE`
: Run the jqawk program in `FILE`

**`-i`**
: Start the interactive REPL

**`-o`** `FILE`
: Write JSON to `FILE`. Use `-` to write to stdout

**`-c`**
: Output compact JSON when the **`-o`** flag is given

**`-r`** `SELECTOR`
: Select the root value. `SELECTOR` is any valid expression

**`--version`**
: Print version information

**`--profile`**
: Record a cpu profile in `jqawk.prof`

**`--dbg-ast`**
: Print the AST of the program and exit

**`--dbg-lex`**
: Print the parsed tokens of the program and exit

# DESCRIPTION
**jqawk** is an awk-inspired programming language for wrangling JSON.

## Program structure
A jqawk program is a series of rules, of the form

```
pattern { body }
```

`pattern` can be any expression, and `body` is a sequence of statements.
If `pattern` evaluates to a truthy value, `body` is evaluated.

There are also some special patterns:

**`BEGIN`** and **`END`** run before and after processing all files  
**`BEGINFILE`** and **`ENDFILE`** run before after processing each file

All other rules are run for each value in the file.

## Variables

Some variables are pre-defined in the body of a rule. If the input is an array:

**`$`** is the current item  
**`$index`** is the array index of that item  

If the input is anything else:

**`$`** is the root value

## Statements

The following statements are valid:

`print <expression>, <expression>, ...`  
: print <expression>s, separated by spaces
<br><br>

`return <expression>`  
: return from the current function, optionally returning the value of <expression>
<br><br>

`break`
: exit the enclosing for or while loop
<br><br>

`continue`
: immediately begin the next iteration of the enclosing loop
<br><br>

`next`
: immediately exit the rule and process no further rules for the current item
<br><br>

`exit`
: immediately exit the program
<br><br>

`if (<expression>) <body> else <elsebody>`
: execute `<body>` if `<expression>` is truthy. If the value is falsy and the else
  is present, execute `<elsebody>`
<br><br>

`while (<expression>) <body>`
: execute the body until `<expression>` is falsy
<br><br>

`for (<preexpression>; <checkexpression>; <postexpression>) <body>`
: execute `<preexpression>`, then execute `{ <body>; <postexpression> }` until
   `<checkexpression>` is falsy

  ```{=man}
  .sp
  ```
  ```{=html}
  <br><br>
  ```
  all three expressions are optional. if `<checkexpression>` is missing, it is
  taken to always be true. `for (;;)` is an infinite loop
<br><br>

`for (<identifier> in <expression>) <body>`
: execute `<body>` for each item in `<expression>` with `<identifier>` set to it's
  value.
  
  ```{=man}
  .sp
  ```
  ```{=html}
  <br><br>
  ```
  if `<expression>` is an array, `<identifier>` is each item.  
  if it's an object, `<identifier>` is each key.  
  if it's a string, `<identifier>` is each character.  
<br><br>

`for (<identifier>, <indexidentifier> in <expression>) <body>`
: same as above, with `<indexindeitifer>` set to the index of the item for arrays
  or the value for objects

## Built-in functions

`printf(format_string, args...)`
: format and print a string according to the format specifier.
  the following format codes are supported:
  ```
    %% - a literal %
    %c - the character represented by the given number
    %d - integer
    %i - integer
    %o - octal integer
    %x - hexidecimal integer
    %f - floating-point number
    %s - string
    %v - the value in its default format
  ```

`json(arg)`
: return arg converted to an indented JSON string
<br><br>

`parseJson(string)`
: return the string parsed as a JSON value
<br><br>

`num(arg)`
: convert arg to a number (64-bit float). returns null if the conversion fails
<br><br>

## Methods

### String

`string.length()`
: return the length
<br><br>

`string.upper()`
: return an uppercase copy of the string
<br><br>

`string.lower()`
: return a lowercase copy of the string
<br><br>

`string.split(separator)`
: split the string into substrings on a separator. the separator may be a
  string or a regex literal
<br><br>

`strings.trim()`
: return the string with leading and trailing whitespace removed

### Number

`number.floor()`
: floor the given number
<br><br>

`number.ceil()`
: ceil or round the given number
<br><br>

`number.round()`
: round the given number. halves are rounded away from zero
<br><br>

`number.abs()`
: return tbe absolute value of the number
<br><br>

`number.mod(n)`
: return the number wrapped into the range [0, n]. unlike the modulo
  operator, the result is never negative
<br><br>

`number.format(thousandsSeparator, decimalSeparator)`
: format a number with thousands and decimal separators. if no separators
  are given `,` is used for thousands and `.` is used for decimals

### Object

`object.length()`
: return the number of keys
<br><br>

`object.pluck(k1, k2, ...)`
: return a shallow copy of the object containing only the given keys
<br><br>

`object.keys()`
: return the keys of an object as a list
<br><br>

`object.values()`
: return the values of an object as a list
<br><br>

`object.pairs()`
: return a list of [key, value] for each entry in the object

### Array

`array.length()`
: return the length
<br><br>

`array.push(value)`
: push value to the end of the array
<br><br>

`array.pop()`
: remove and return the last value in the array
<br><br>

`array.popfirst()`
: remove and return the first value in the array
<br><br>

`array.contains(value)`
: return true if value is in the array
<br><br>

`array.sort(fn)`
: return a sorted copy of the array.
  ```{=man}
  .sp
  ```
  ```{=html}
  <br><br>
  ```
  with no fn given the array is sorted by string value, or numerically if all
  the values are nunbers.
  ```{=man}
  .sp
  ```
  ```{=html}
  <br><br>
  ```
   with fn(a, b) provided, the array is sorted according to fn's return value:  
   1 means a < b  
   0 means a == b  
   1 means a > b
<br><br>

`array.sortKey(string|fn)`
: return a sorted copy of the array sorted by a key taken from each element.

  ```{=man}
  .sp
  ```
  ```{=html}
  <br><br>
  ```
  with a string argument, the key is `item[string]`
  with a function argument, the key is the result of `fn(item)`
<br><br>

`array.reverse()`
: return a reversed copy of the array

# SEE ALSO

Project homepage [http://github.com/alligator/jqawk](http://github.com/alligator/jqawk)
