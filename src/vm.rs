use std::fmt;
use std::collections::HashMap;
use serde_json;
use crate::compiler::JqaRule;

#[derive(Clone, Debug)]
pub enum OpCode {
  GetField(String),
  PushImmediate(Value),
  GetMember,
  Equal,
  Print,
}

#[derive(PartialEq, Clone, Debug)]
pub enum Value {
  Str(String),
  Num(i64),
  Object(serde_json::Value),
  Array(serde_json::Value),
}

impl Value {
  fn from(v: serde_json::Value) -> Value {
    if v.is_array() {
      return Value::Array(v);
    }
    if v.is_object() {
      return Value::Object(v);
    }
    if v.is_string() {
      return Value::Str(v.as_str().unwrap().to_string());
    }

    return Value::Num(0);
  }

  fn compare(&self, other: Value) -> bool {
    match (self, other) {
      (Value::Str(a), Value::Str(b)) => a.eq(&b),
      _ => false,
    }
  }

  fn truthy(self) -> bool {
    match self {
      Value::Str(s) => s.len() > 0,
      Value::Num(n) => n != 0,
      _ => false,
    }
  }
}

impl fmt::Display for Value {
  fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
    write!(f, "{}", match self {
      Value::Str(s) => String::from(s),
      Value::Num(n) => format!("{}", n),
      _ => format!("{:?}", self),
    })
  }
}

pub struct Vm {
  rules: Vec<JqaRule>,
  fields: HashMap<String, Value>,
  stack: Vec<Value>,
  dbg: bool,
}

impl Vm {
  pub fn new(rules: Vec<JqaRule>, dbg: bool) -> Vm {
    Vm {
      rules,
      fields: HashMap::new(),
      stack: Vec::new(),
      dbg,
    }
  }

  fn push(&mut self, val: Value) {
    self.stack.push(val);
  }
  fn pop(&mut self) -> Value {
    self.stack.pop().unwrap()
  }

  fn dbg(&mut self, op_code: &OpCode) {
    if self.dbg {
      println!("> {:?}", op_code);
    }
  }

  fn dbg_stack(&mut self) {
    if self.dbg {
      println!("--> {:?}", self.stack);
    }
  }

  fn eval(&mut self, prog: Vec<OpCode>) {
    for op_code in prog.iter() {
      self.dbg(op_code);
      self.dbg_stack();
      match op_code {
        OpCode::GetField(s) => {
          if s.len() == 0 {
            let field = self.fields.get("root").unwrap().clone();
            self.push(field);
          } else {
            if !self.fields.contains_key(s) {
              panic!("unknown field: {}", s);
            }
            let field = self.fields.get(s).unwrap().clone();
            self.push(field);
          }
        },
        OpCode::PushImmediate(v) => {
          self.push(v.clone());
        },
        OpCode::GetMember => {
          let member = self.pop();
          let obj = self.pop();

          let key = match member {
            Value::Str(s) => s,
            _ => panic!("nope"),
          };

          match obj {
            Value::Object(o) => {
              let obj = o.as_object().unwrap();
              let val = obj.get(&key).unwrap();
              self.push(Value::from(val.clone()));
            },
            _ => panic!("can only access members on objects"),
          }
        },
        OpCode::Equal => {
          let left = self.pop();
          let right = self.pop();
          let result = left.compare(right);
          self.push(Value::Num(if result { 1 } else { 0 }));
        },
        OpCode::Print => {
          let val = self.pop();
          println!("{}", val);
        },
      }
      self.dbg_stack();
    }
  }

  fn eval_rules(&mut self, root: Value) {
    self.fields.insert(String::from("root"), root);
    let rules = self.rules.clone();
    for rule in rules.iter() {
      self.eval(rule.pattern.clone());
      match self.stack.pop() {
        Some(v) => {
          if v.truthy() {
            self.eval(rule.body.clone());
          }
        }
        _ => panic!("expected one number on the stack after pattern"),
      }
    }
  }

  pub fn run(&mut self, content: &str) {
    let v: serde_json::Value = serde_json::from_str(content).unwrap();
    match v {
      serde_json::Value::Array(a) => {
        for item in a {
          let val = Value::from(item);
          self.eval_rules(val);
        }
      },
      _ => panic!("JSON must be an object or an array"),
    }
  }
}