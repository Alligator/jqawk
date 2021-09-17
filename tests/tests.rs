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

  return String::from_utf8_lossy(&output.stdout).to_string();
}

fn run_stdin(args: &[&str], stdin: &str) -> String {
  let mut child = Command::new(jqawk_exe())
    .args(args)
    .stdin(Stdio::piped())
    .stdout(Stdio::piped())
    .spawn()
    .expect("error spawning jqawk");

  child.stdin.as_mut().unwrap().write_all(stdin.as_bytes())
    .expect("could not write to child stdin");

  let output = child.wait_with_output().expect("error reading child stdout");
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