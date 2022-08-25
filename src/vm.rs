use std::fmt;
use std::collections::HashMap;
use std::io;
use std::cell::RefCell;
use serde_json;
use regex::Regex;
use crate::compiler::{JqaRule, JqaRuleKind};

#[derive(Clone, Debug)]
pub enum OpCode {
  GetField(String),
  PushImmediate(Value),
  GetMember,
  GetGlobal(String),
  SetGlobal(String),
  Equal,
  And,
  Or,
  Add,
  Subtract,
  Multiply,
  Divide,
  Greater,
  Match,
  Negate,
  Print(usize),
}

#[derive(PartialEq, Clone, Debug)]
pub enum Value {
  Str(String),
  Regex(String),
  Num(f64),
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
    if v.is_number() {
      return Value::Num(v.as_f64().unwrap());
    }

    return Value::Num(0.0);
  }

  fn from_opt(v: Option<&serde_json::Value>) -> Value {
    match v {
      Some(v) => Value::from(v.clone()),
      None => Value::Num(0.0),
    }
  }

  fn compare(&self, other: Value) -> bool {
    match (self, other) {
      (Value::Str(a), Value::Str(b)) => a.eq(&b),
      (Value::Num(a), Value::Num(b)) => a.eq(&b),
      _ => false,
    }
  }

  fn as_f64(&self) -> f64 {
    match self {
      Value::Num(n) => *n,
      Value::Str(s) => s.parse().unwrap_or(0.0),
      _ => 0.0
    }
  }

  fn truthy(self) -> bool {
    match self {
      Value::Str(s) => s.len() > 0,
      Value::Num(n) => n != 0.0,
      _ => false,
    }
  }

  fn display_type(self) -> &'static str {
    match self {
      Value::Str(_) => "string",
      Value::Num(_) => "number",
      Value::Array(_) => "array",
      Value::Object(_) => "object",
      Value::Regex(_) => "regex",
    }
  }
}

impl fmt::Display for Value {
  fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
    write!(f, "{}", match self {
      Value::Str(s) => String::from(s),
      Value::Regex(r) => format!("/{}/", r),
      Value::Num(n) => format!("{}", n),
      Value::Array(v) | Value::Object(v) => format!("{}", v),
    })
  }
}


fn for_each_in<F: FnMut(Value) -> Result<(), RuntimeError>>(v: Value, mut func: F) -> Result<(), RuntimeError> {
  match v {
    Value::Array(a) => {
      let arr = a.as_array().unwrap();
      for item in arr {
        let val = Value::from(item.clone());
        func(val)?;
      }
    },
    Value::Object(o) => {
      let obj = o.as_object().unwrap();
      for (_k, v) in obj.iter() {
        let val = Value::from(v.clone());
        func(val)?;
      }
    },
    _ => return Err(RuntimeError{ msg: format!("JSON must be an object or an array, got {:?}", v) }),
  }
  return Ok(());
}

#[derive(Debug)]
pub struct RuntimeError {
  pub msg: String,
}

pub struct Vm {
  fields: HashMap<String, Value>,
  variables: RefCell<HashMap<String, Value>>,
  stack: Vec<Value>,
  dbg: bool,
}

impl Vm {
  pub fn new(dbg: bool) -> Vm {
    let mut variables = HashMap::new();
    variables.insert(String::from("NR"), Value::Num(0.0));
    Vm {
      fields: HashMap::new(),
      variables: RefCell::new(variables),
      stack: Vec::new(),
      dbg,
    }
  }

  fn push(&mut self, val: Value) {
    self.stack.push(val);
  }
  fn pop(&mut self) -> Result<Value, RuntimeError> {
    let v = self.stack.pop();
    if v.is_none() {
      return Err(RuntimeError { msg: String::from("attempted to pop an empty stack") });
    }
    return Ok(v.unwrap());
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

  fn error(&mut self, msg: String) -> Result<(), RuntimeError> {
    return Err(RuntimeError { msg });
  }

  fn eval(&mut self, prog: Vec<OpCode>) -> Result<(), RuntimeError> {
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
              return self.error(format!("unknown field: {}", s));
            }
            let field = self.fields.get(s).unwrap().clone();
            self.push(field);
          }
        },
        OpCode::PushImmediate(v) => {
          self.push(v.clone());
        },
        OpCode::GetMember => {
          let member = self.pop()?;
          let obj = self.pop()?;

          match obj {
            Value::Array(a) => {
              let idx = match member {
                Value::Num(n) => n,
                _ => return self.error(format!("cannot index an array with a {}", member.display_type())),
              };

              let arr = a.as_array().unwrap();
              let val = arr.iter().nth(idx as usize);
              self.push(Value::from_opt(val));
            },
            Value::Object(o) => {
              let key = match member {
                Value::Str(s) => s,
                Value::Num(n) => n.to_string(),
                _ => return self.error(format!("cannot access member on object with {}", member.display_type())),
              };

              let obj = o.as_object().unwrap();
              let val = obj.get(&key).ok_or(RuntimeError { msg: format!("unknown key {}", key) })?;
              self.push(Value::from(val.clone()));
            },
            _ => return self.error(format!("can only access members on objects or arrays, found {}", obj.display_type())),
          }
        },
        OpCode::Equal => {
          let right = self.pop()?;
          let left = self.pop()?;
          let result = left.compare(right);
          self.push(Value::Num(if result { 1.0 } else { 0.0 }));
        },
        OpCode::And => {
          let right = self.pop()?;
          let left = self.pop()?;
          if left.truthy() && right.truthy() {
            self.push(Value::Num(1.0));
          } else {
            self.push(Value::Num(0.0));
          }
        },
        OpCode::Or => {
          let right = self.pop()?;
          let left = self.pop()?;
          if left.truthy() || right.truthy() {
            self.push(Value::Num(1.0));
          } else {
            self.push(Value::Num(0.0));
          }
        },
        OpCode::Add => {
          let right = self.pop()?.as_f64();
          let left = self.pop()?.as_f64();
          self.push(Value::Num(left + right));
        },
        OpCode::Subtract => {
          let right = self.pop()?.as_f64();
          let left = self.pop()?.as_f64();
          self.push(Value::Num(left - right));
        },
        OpCode::Multiply => {
          let right = self.pop()?.as_f64();
          let left = self.pop()?.as_f64();
          self.push(Value::Num(left * right));
        },
        OpCode::Divide => {
          let right = self.pop()?.as_f64();
          let left = self.pop()?.as_f64();
          self.push(Value::Num(left / right));
        },
        OpCode::Greater => {
          let right = self.pop()?;
          let left = self.pop()?;

          match (left, right) {
            (Value::Str(l), Value::Str(r)) => {
              self.push(Value::Num(if l > r { 1.0 } else { 0.0 }));
            },
            (l, r) => {
              self.push(Value::Num(if l.as_f64() > r.as_f64() { 1.0 } else { 0.0 }));
            }
          }
        },
        OpCode::Match => {
          let right = self.pop()?;
          let left = self.pop()?;
          match (left, right) {
            (Value::Str(s), Value::Regex(r)) => {
              let re = Regex::new(r.as_str());
              if re.is_err() {
                let e = re.unwrap_err();
                return self.error(format!("invalid regex: {}", e));
              }
              self.push(Value::Num(if re.unwrap().is_match(s.as_str()) { 1.0 } else { 0.0 }));
            },
            _ => return self.error(format!("can only match a string against a regex")),
          }
        },
        OpCode::Negate => {
          let arg = self.pop()?;
          match arg {
            Value::Num(n) => {
              self.push(Value::Num(if n == 0.0 { 1.0 } else { 0.0 }));
            }
            _ => return self.error(format!("can only negate a number")),
          }
        },
        OpCode::Print(argc) => {
          if *argc == 0 {
            println!("{}", self.fields.get("root").unwrap().clone());
            break;
          }

          let mut args = Vec::with_capacity(*argc);
          for _ in 0..*argc {
            args.insert(0, format!("{}", self.pop()?));
          }
          println!("{}", args.join(" "));
        },
        OpCode::GetGlobal(name) => {
          let val: Option<Value>;
          {
            let variables = self.variables.borrow();
            if variables.contains_key(name) {
              val = Some(variables.get(name).unwrap().clone());
            } else {
              val = None;
            }
          }

          if val.is_none() {
            self.push(Value::Num(0.0));
          } else {
            self.push(val.unwrap());
          }
        },
        OpCode::SetGlobal(name) => {
          let val = self.pop()?;
          let mut variables = self.variables.borrow_mut();
          variables.insert(name.clone(), val);
        },
        #[allow(unreachable_patterns)]
        _ => return self.error(format!("unknown opcode {:?}", op_code)),
      }
      self.dbg_stack();
    }
    return Ok(());
  }

  fn eval_rules(&mut self, rules: &Vec<JqaRule>, kind: JqaRuleKind, root: Value) -> Result<(), RuntimeError> {
    self.fields.insert(String::from("root"), root);
    for rule in rules.iter().filter(|&rule| rule.kind == kind) {
      if rule.pattern.len() == 0 {
        self.eval(rule.body.clone())?;
        continue;
      }

      self.eval(rule.pattern.clone())?;
      match self.stack.pop() {
        Some(v) => {
          if v.truthy() {
            self.eval(rule.body.clone())?;
          }
        }
        _ => return self.error(String::from("expected one value on the stack after pattern")),
      }
    }
    return Ok(());
  }

  pub fn run<T>(&mut self, rdr:T, selector: Vec<OpCode>, rules: Vec<JqaRule>) -> Result<(), RuntimeError> where T: io::Read {
    let v: serde_json::Value = serde_json::from_reader(rdr)
      .expect("error parsing JSON");
    
    self.fields.insert(String::from("root"), Value::from(v));
    self.eval(selector)?;

    match self.stack.pop() {
      Some(v) => {
        self.eval_rules(&rules, JqaRuleKind::Begin, v.clone())?;
        let v_clone = v.clone();
        for_each_in(v, |val| {
          {
            let mut variables = self.variables.borrow_mut();
            let nr = variables.get("NR").unwrap().as_f64();

            variables.insert(String::from("NR"), Value::Num(nr + 1.0));
          }

          self.eval_rules(&rules, JqaRuleKind::Match, val)?;
          return Ok(());
        })?;
        self.eval_rules(&rules, JqaRuleKind::End, v_clone)?;
      },
      _ => return self.error(String::from("expected a value on the stack after the selector")),
    }
    return Ok(());
  }
}