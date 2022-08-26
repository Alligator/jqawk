use crate::vm::OpCode;
use crate::compiler::JqaRule;

fn format_op(op: &OpCode) -> String {
  match op {
    OpCode::GetField(s) =>
      format!("GetField {}", s),
    OpCode::PushImmediate(v) =>
      format!("PushImmediate {:?}", v),
    OpCode::GetMember =>
      format!("GetMember"),
    OpCode::GetGlobal(s) =>
      format!("GetGlobal {}", s),
    OpCode::SetGlobal(s) =>
      format!("SetGlobal {}", s),
    OpCode::Equal =>
      format!("Equal"),
    OpCode::And =>
      format!("And"),
    OpCode::Or =>
      format!("Or"),
    OpCode::Add =>
      format!("Add"),
    OpCode::Subtract =>
      format!("Subtract"),
    OpCode::Multiply =>
      format!("Multiply"),
    OpCode::Divide =>
      format!("Divide"),
    OpCode::Greater =>
      format!("Greater"),
    OpCode::Match =>
      format!("Match"),
    OpCode::Negate =>
      format!("Negate"),
    OpCode::Print(n) =>
      format!("Print {}", n),
  }
}

pub fn print_rules(rules: &Vec<JqaRule>) {
  println!("rule");
  for rule in rules.iter() {
    println!("  kind: {:?}", rule.kind);

    println!("  pattern");
    for op in &rule.pattern {
      println!("    {}", format_op(op));
    }

    println!("body");
    for op in &rule.body {
      println!("    {}", format_op(op));
    }
  }
}