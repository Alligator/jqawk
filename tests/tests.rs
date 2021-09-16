use std::env;
use std::path::PathBuf;
use std::process::Command;

fn jqawk_exe() -> PathBuf {
  env::current_exe().unwrap()
    .parent()
    .expect("exe dir")
    .join("jqawk")
}

fn run(args: &[&str]) -> String {
  let mut cmd = Command::new(jqawk_exe());
  for arg in args {
    cmd.arg(arg);
  }
  let o = cmd.output().expect("Failed to execute jqawk");
  return String::from_utf8_lossy(&o.stdout).to_string();
}

#[test]
fn begin_and_end() {
  let program = "\
END { print \"end\"; }
BEGIN { print \"begin\"; }";
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