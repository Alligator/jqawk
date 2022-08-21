use std::io::Write;
use std::env;
use std::path::PathBuf;
use std::process::{Command, Stdio};

fn jqawk_exe() -> PathBuf {
  env::current_exe().unwrap()
    .parent()
    .expect("exe dir")
    .join("jqawk")
}

fn run(args: &[&str]) -> String {
  let output = Command::new(jqawk_exe())
    .args(args)
    .output()
    .expect("Failed to execute jqawk");

  if !output.status.success() {
    panic!("jqawk failed: {}", String::from_utf8_lossy(&output.stderr));
  }

  return String::from_utf8_lossy(&output.stdout).to_string();
}

fn run_stdin(args: &[&str], stdin: &str) -> String {
  let mut child = Command::new(jqawk_exe())
    .args(args)
    .stdin(Stdio::piped())
    .stdout(Stdio::piped())
    .stderr(Stdio::piped())
    .spawn()
    .expect("error spawning jqawk");

  child.stdin.as_mut().unwrap().write_all(stdin.as_bytes())
    .expect("could not write to child stdin");

  let output = child.wait_with_output().expect("error reading child stdout");

  if !output.status.success() {
    panic!("jqawk failed: {}", String::from_utf8_lossy(&output.stderr));
  }

  return String::from_utf8_lossy(&output.stdout).to_string();
}

#[test]
fn begin_and_end() {
  let program = "\
END { print \"end\" }
BEGIN { print \"begin\" }";
  let output = run(&[program, "test.json"]);
  
  assert_eq!(output, "begin\nend\n".to_string());
}

#[test]
fn members() {
  let program = "{ print $.age; }";
  let output = run(&[program, "test.json"]);
  
  let expected = "\
10
56
72
";
  assert_eq!(output, expected.to_string());
}

#[test]
fn root_param() {
  let program = "{ print $; }";
  let output = run(&["-r", "$[0]", program, "test.json"]);
  
  let expected = "\
10
tiny tony
";
  assert_eq!(output, expected.to_string());
}

#[test]
fn stdin_input() {
  let program = "{ print $ }";
  let output = run_stdin(&[program], "[1, 2, 3]");
  let expected = "\
1
2
3
";
  assert_eq!(output, expected.to_string());
}

#[test]
fn variables() {
  let program = "\
BEGIN { total = 0 }
{ total = total + $ }
END { print total }";
  let input = "[1, 2, 3, 4, 5, 6, 7, 8, 9, 10]";
  let output = run_stdin(&[program], input);
  assert_eq!(output, "55\n");
}

#[test]
fn operators() {
  let program = "\
BEGIN {
  print 2 + 4;
  print 2 - 1;
  print 2 * 4;
  print 6 / 3;
}
";
  let output = run_stdin(&[program], "[]");
  assert_eq!(output, "6\n1\n8\n2\n");
}

// one true awk inspired tests
macro_rules! jqawk_test {
  ($name:ident, $program:expr, $input:expr, $expected:expr) => {
    #[test]
    fn $name() {
      assert_eq!(run_stdin(&[$program], $input), $expected);
    }
  }
}

jqawk_test!(p1, "{ print }", "[1, 2, 3]", "1\n2\n3\n");
jqawk_test!(p2, "{ print $[0], $[2] }", "[[1, 2, 3], [10, 20, 30]]", "1 3\n10 30\n");
// p3 omitted until printf
jqawk_test!(p4, "{ print NR, $ }", "[2, 4, 6, 8]", "1 2\n2 4\n3 6\n4 8\n");
// p5 ommitted until printf
jqawk_test!(p6, "END { print NR }", "[1, 2, 3, 4]", "4\n");
jqawk_test!(p7, "$[1] > 100", "[[10, 20], [100, 200], [1000, 2000]]", "[100,200]\n[1000,2000]\n");
jqawk_test!(p8,
  "$.continent == \"Asia\" { print $.country }",
  "[{ \"continent\": \"Asia\", \"country\": \"Japan\" },
    { \"continent\": \"Europe\", \"country\": \"Sweden\" }]",
  "Japan\n");
jqawk_test!(p9, "$ > \"S\"", "[\"Clive\", \"Tony\"]", "Tony\n");
jqawk_test!(p10, "$[0] == $[1]", "[[1, 2], [3, 3], [4, 5]]", "[3,3]\n");

jqawk_test!(p11, "$ ~ /Asia/", "[\"Asia\", \"Europe\"]", "Asia\n");
jqawk_test!(p12,
  "$.continent ~ /Europe/ { print $.country }",
  "[{ \"continent\": \"Asia\", \"country\": \"Japan\" },
    { \"continent\": \"Europe\", \"country\": \"Sweden\" }]",
  "Sweden\n");
jqawk_test!(p13,
  "$.continent !~ /Europe/ { print $.country }",
  "[{ \"continent\": \"Asia\", \"country\": \"Japan\" },
    { \"continent\": \"Europe\", \"country\": \"Sweden\" }]",
  "Japan\n");
jqawk_test!(p14, "$ ~ /\\$/", "[\"£gbp\", \"$usd\"]", "$usd\n");
jqawk_test!(p15, "$ ~ /\\\\/", "[\"C:\\\\\"]", "C:\\\n");
jqawk_test!(p16, "$ ~ /^.$/", "[\"a\", \"abc\"]", "a\n");

/*
==> p.17 <==
$2 !~ /^[0-9]+$/

==> p.18 <==
/(apple|cherry) (pie|tart)/

==> p.19 <==
BEGIN	{ digits = "^[0-9]+$" }
$2 !~ digits
*/

jqawk_test!(p20,
  "$.name == \"alligator\" && $.age > 30 { print $.id }",
  "[{ \"id\": 1, \"name\": \"alligator\", \"age\": 25 },
    { \"id\": 2, \"name\": \"alligator\", \"age\": 35 },
    { \"id\": 3, \"name\": \"clive\", \"age\": 35 }]",
  "2\n");
jqawk_test!(p21,
  "$.name == \"alligator\" || $.age > 30 { print $.id }",
  "[{ \"id\": 1, \"name\": \"not alligator\", \"age\": 25 },
    { \"id\": 2, \"name\": \"alligator\", \"age\": 35 },
    { \"id\": 3, \"name\": \"clive\", \"age\": 35 }]",
  "2\n3\n");

/*
p.21a
/Asia/ || /Africa/

p.22
$4 ~ /^(Asia|Europe)$/

p.23
/Canada/, /Brazil/

p.24
FNR == 1, FNR == 5 { print FILENAME, $0 }

p.25
{ printf "%10s %6.1f\n", $1, 1000 * $3 / $2 }

p.26
/Asia/	{ pop = pop + $3; n = n + 1 }
END	{ print "population of", n,\
		"Asian countries in millions is", pop }

p.26a
/Asia/	{ pop += $3; ++n }
END	{ print "population of", n,\
		"Asian countries in millions is", pop }

p.27
maxpop < $3	{ maxpop = $3; country = $1 }
END		{ print country, maxpop }

p.28
{ print NR ":" $0 }

p.29
	{ gsub(/USA/, "United States"); print }

p.30
{ print length, $0 }

p.31
length($1) > max	{ max = length($1); name = $1 }
END			{ print name }

p.32
{ $1 = substr($1, 1, 3); print }

p.33
	{ s = s " " substr($1, 1, 3) }
END	{ print s }

p.34
{ $2 /= 1000; print }

p.35
BEGIN			{ FS = OFS = "\t" }
$4 ~ /^North America$/	{ $4 = "NA" }
$4 ~ /^South America$/	{ $4 = "SA" }
			{ print }

p.36
BEGIN	{ FS = OFS = "\t" }
	{ $5 = 1000 * $3 / $2 ; print $1, $2, $3, $4, $5 }

p.37
$1 "" == $2 ""

p.38
{	if (maxpop < $3) {
		maxpop = $3
		country = $1
	}
}
END	{ print country, maxpop }

p.39
{	i = 1
	while (i <= NF) {
		print $i
		i++
	}
}

p.40
{	for (i = 1; i <= NF; i++)
		print $i
}

p.41
NR >= 10	{ exit }
END		{ if (NR < 10)
			print FILENAME " has only " NR " lines" }

p.42
/Asia/		{ pop["Asia"] += $3 }
/Africa/	{ pop["Africa"] += $3 }
END		{ print "Asian population in millions is", pop["Asia"]
		  print "African population in millions is", pop["Africa"] }

p.43
BEGIN	{ FS = "\t" }
	{ area[$4] += $2 }
END	{ for (name in area)
		print name ":" area[name] }

p.44
function fact(n) {
	if (n <= 1)
		return 1
	else
		return n * fact(n-1)
}
{ print $1 "! is " fact($1) }

p.45
BEGIN	{ OFS = ":" ; ORS = "\n\n" }
	{ print $1, $2 }

p.46
	{ print $1 $2 }

p.47
$3 > 100	{ print >"tempbig" }
$3 <= 100	{ print >"tempsmall" }

p.48
BEGIN	{ FS = "\t" }
	{ pop[$4] += $3 }
END	{ for (c in pop)
		print c ":" pop[c] | "sort" }

p.48a
BEGIN {
	for (i = 1; i < ARGC; i++)
		printf "%s ", ARGV[i]
	printf "\n"
	exit
}

p.48b
BEGIN	{ k = 3; n = 10 }
{	if (n <= 0) exit
	if (rand() <= k/n) { print; k-- }
	n--
}

p.49
$1 == "include" { system("cat " $2) }

p.50
BEGIN	{ FS = "\t" }
	{ pop[$4 ":" $1] += $3 }
END	{ for (cc in pop)
		print cc ":" pop[cc] | "sort -t: -k 1,1 -k 3nr" }

p.51
BEGIN	{ FS = ":" }
{	if ($1 != prev) {
		print "\n" $1 ":"
		prev = $1
	}
	printf "\t%-10s %6d\n", $2, $3
}

p.52
BEGIN	{ FS = ":" }
{
	if ($1 != prev) {
		if (prev) {
			printf "\t%-10s\t %6d\n", "total", subtotal
			subtotal = 0
		}
		print "\n" $1 ":"
		prev = $1
	}
	printf "\t%-10s %6d\n", $2, $3
	wtotal += $3
	subtotal += $3
}
END	{ printf "\t%-10s\t %6d\n", "total", subtotal
	  printf "\n%-10s\t\t %6d\n", "World Total", wtotal }
*/
