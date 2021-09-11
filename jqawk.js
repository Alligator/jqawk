import { parse } from "https://deno.land/std@0.106.0/flags/mod.ts";

class Lexer {
  constructor(src) {
    this.src = src;
    this.pos = 0;
    this.line = 1;
  }

  advance() {
    if (this.pos < this.src.length) {
      this.pos++;
    }
    return this.src[this.pos - 1];
  }
  peek() {
    return this.src[this.pos];
  }
  atEOF() {
    return this.pos >= this.src.length;
  }

  simpleToken(type) {
    return { type, line: this.line };
  }
  stringToken(type, str) {
    return { type, str, line: this.line };
  }

  isAlpha(c) {
    return /^[a-zA-Z]+/.test(c);
  }
  identifier() {
    while (this.isAlpha(this.peek())) {
      this.advance();
    }

    const word = this.src.substring(this.start, this.pos);
    switch (word) {
      case 'print':
        return this.simpleToken('print');
      case 'BEGIN':
        return this.simpleToken('begin');
      case 'END':
        return this.simpleToken('end');
      default:
        return this.stringToken('identifier', word);
    }
  }

  isNumeric(c) {
    return /^[0-9]+/.test(c);
  }
  number() {
    while (this.isNumeric(this.peek())) {
      this.advance();
    }

    const num = this.src.substring(this.start, this.pos);
    return this.stringToken('number', num);
  }
  
  string() {
    while (this.peek() !== '"') {
      this.advance();
      if (this.atEOF()) {
        throw new Error('unexpected EOF in string');
      }
    }
    this.advance();

    const str = this.src.substring(this.start + 1, this.pos - 1);
    return this.stringToken('string', str);
  }

  skipWhitespace() {
    while (true) {
      switch (this.peek()) {
        case ' ': {
          this.advance();
          break;
        }
        case '\n': {
          this.advance();
          this.line++;
          break;
        }
        default:
          return;
      }
    }
  }

  nextToken() {
    this.skipWhitespace();
    if (this.pos >= this.src.length) {
      return this.simpleToken('eof');
    }

    this.start = this.pos;
    const c = this.advance();

    if (this.isAlpha(c)) {
      return this.identifier();
    }

    if (this.isNumeric(c)) {
      return this.number();
    }

    if (c === '"') {
      return this.string();
    }

    switch (c) {
      case '$': return this.simpleToken('dollar');
      case '+': return this.simpleToken('plus');
      case '>': return this.simpleToken('greater');
      case '{': return this.simpleToken('lcurly');
      case '}': return this.simpleToken('rcurly');
      case '[': return this.simpleToken('lsquare');
      case ']': return this.simpleToken('rsquare');
      case '.': return this.simpleToken('dot');
      case ',': return this.simpleToken('comma');
      case ';': return this.simpleToken('semicolon');
      case '=': {
        if (this.peek() === '=') {
          this.advance();
          return this.simpleToken('equalequal');
        }
        return this.simpleToken('equal');
      }
    }

    throw new Error(`unexpected character ${JSON.stringify(c)}`);
  }
}

// this "parser" does the bare minimum. it parses a list of these:
//   <stream of tokens> { <stream of tokens }
//
// the actual parser for the language is inside the evaluator
class Parser {
  constructor(src) {
    this.src = src;
    this.lexer = new Lexer(src);
  }

  advance() {
    this.current = this.lexer.nextToken();
  }

  consume(type) {
    if (this.current.type !== type) {
      throw new Error(`found ${this.current.type} but expected ${type}`);
    }
    this.advance();
  }

  parseProgram() {
    this.advance();

    const rules = {
      begin: [],
      main: [],
      end: [],
    };

    while (this.current.type !== 'eof') {
      const pattern = [];
      while (this.current.type !== 'lcurly') {
        pattern.push(this.current);
        this.advance();
      }

      this.consume('lcurly');

      const body = [];
      while (this.current.type !== 'rcurly') {
        body.push(this.current);
        this.advance();
      }

      this.consume('rcurly');

      if (pattern.length && pattern[0].type === 'begin') {
        rules.begin.push({ pattern, body });
      } else if (pattern.length && pattern[0].type === 'end') {
        rules.end.push({ pattern, body });
      } else {
        rules.main.push({ pattern, body });
      }
    }

    return rules;
  }
}

function val(value) {
  if (value === null) {
    return { type: null };
  }
  switch (typeof value) {
    case 'number':
      return { type: 'number', value };
    case 'string':
      return { type: 'string', value };
    case 'boolean':
      return { type: 'bool', value };
    case 'object': {
      if (Array.isArray(value)) {
        return { type: 'array', value };
      }
      return { type: 'object', value };
    }
  }

  throw new Error(`cannot create a value from ${value}`);
}

class Evaluator {
  constructor(prog) {
    this.prog = prog;
    this.pos = 0;
    this.environment = {};
    this.fields = {};
  }

  // hand rolled pratt parser
  // this should probably be in it's own class...
  evaluate(tokens, startType) {
    const precedence = {
      none: 0,
      assign: 10,
      seq: 20,
      comp: 30,
      add: 40,
      mul: 50,
      fn: 60,
    };

    let current = tokens[0];
    let pos = 1;
    let prev;

    // utils
    const advance = () => {
      prev = current;
      current = tokens[pos++];
      if (!current) {
        current = { type: 'eof', line: prev.line };
      }
    };

    const consume = (type) => {
      if (current.type !== type) {
        fatal(`expected ${type} but got ${current.type}`);
      }
      advance();
    };

    const fatal = (msg) => {
      throw new Error(`error on line ${current.line}: ${msg}`);
    };

    // grammar
    const printStatement = () => {
      consume('print');
      const args = [];
      while (current.type !== 'semicolon') {
        args.push(expression());
        if (current.type === 'comma') {
          consume('comma');
        } else {
          break;
        }
      }

      console.log(args.map((arg) => {
        switch (arg.type) {
          case 'array':
            return '<array>';
          case 'object':
            return '<object>';
          default:
            return arg.value;
        }
      }).join(' '));
    };

    const statement = () => {
      let result;
      switch (current.type) {
        case 'print': {
          result = printStatement();
          break;
        }
        default: {
          result = expression();
          break;
        }
      }

      consume('semicolon');
      return result;
    };

    const number = () => {
      return val(parseInt(prev.str, 10));
    };
    const string = () => {
      return val(prev.str);
    };

    const member = (left) => {
      consume('dot');
      consume('identifier');
      if (left.type !== 'object') {
        fatal('cannot access member of non object');
      }
      return val(left.value[prev.str]);
    };

    const binary = (left) => {
      const token = current;
      advance();

      const right = expression(getRule(token.type).prec);

      switch (token.type) {
        case 'greater':
          return val(left.value > right.value);
        case 'equalequal':
          return val(left.value === right.value);
        case 'plus':
          return val(left.value + right.value);
      }

      fatal(`unknown operator ${token.type}`);
    };

    const identifier = () => {
      const key = prev.str;
      if (!(key in this.environment)) {
        this.environment[key] = val(null);
      }
      return this.environment[key];
    };

    const field = () => {
      if (current.type === 'identifier') {
        consume('identifier');
        const key = prev.str;
        return this.fields[key];
      }
      return this.fields.root;
    };

    const subscript = (left) => {
      consume('lsquare');
      const key = expression();
      consume('rsquare');

      if (left.type !== 'array') {
        fatal('cannot subscript non array value');
      }

      return val(left.value[key.value]);
    };

    const assign = (left) => {
      consume('equal');
      const value = expression();
      left.type = value.type;
      left.value = value.value;
      return left;
    };

    const getRule = (type) => {
      const r = {
        number: {
          prec: precedence.none,
          prefix: number,
        },
        string: {
          prec: precedence.none,
          prefix: string,
        },
        dot: {
          prec: precedence.fn,
          infix: member,
        },
        greater: {
          prec: precedence.comp,
          infix: binary,
        },
        dollar: {
          prec: precedence.none,
          prefix: field,
        },
        equalequal: {
          prec: precedence.comp,
          infix: binary,
        },
        plus: {
          prec: precedence.add,
          infix: binary,
        },
        lsquare: {
          prec: precedence.fn,
          infix: subscript,
        },
        identifier: {
          prec: precedence.none,
          prefix: identifier,
        },
        equal: {
          prec: precedence.assign,
          infix: assign,
        },
      }[type] || { prec: precedence.none };
      return r;
    };

    const expression = (prec = precedence.assign) => {
      const token = current;
      const rule = getRule(token.type);
      if (!rule.prefix) {
        fatal(`unexpected prefix ${token.type}`);
      }

      advance();
      let left = rule.prefix();

      while (prec <= getRule(current.type).prec) {
        const infixRule = getRule(current.type);
        if (!infixRule.infix) {
          fatal(`unexpected infix ${current.type}`);
        }
        left = infixRule.infix(left);
      }

      return left;
    };

    let result;
    if (startType === 'expression') {
      result = expression(precedence.assign);
    } else {
      while (current.type !== 'eof') {
        statement();
      }
    }

    if (pos < tokens.length) {
      fatal(`dangling tokens ${JSON.stringify(tokens.slice(pos, pos + 5))}`);
    }
    return result;
  }

  forEachRecord(cb) {
    if (Array.isArray(this.json)) {
      while (this.pos < this.json.length) {
        this.fields.key = val(this.pos);
        cb(this.json[this.pos++]);
      }
      return;
    }

    if (typeof this.json === 'object') {
      Object.keys(this.json).forEach((key) => {
        this.fields.key = val(key);
        cb(this.json[key]);
      });
      return;
    }

    throw new Error('expected top level JSON to be an array or object');
  }

  async run(json) {
    this.json = json;

    this.prog.begin.forEach(({ body }) => {
      this.evaluate(body, 'statement');
    });

    if (this.json) {
      this.forEachRecord((record) => {
        this.prog.main.forEach(({ pattern, body }) => {
          this.fields.root = val(record);
          let result;
          if (pattern.length === 0) {
            result = { value: true };
          } else {
            result = this.evaluate(pattern, 'expression');
          }
          if (result.value) {
            this.evaluate(body, 'statement');
         }
        });
      });
    }

    this.prog.end.forEach(({ body }) => {
      this.evaluate(body, 'statement');
    });
  }
}

function usage() {
  console.log('usage: jqawk -f program_file file');
  console.log('       jqawk \'program\' file');
  Deno.exit(0);
}

const args = parse(Deno.args);

if (args.h || args.help || args._.length < 1) {
  usage();
}

let src;
let file;

if (args.f) {
  src = Deno.readTextFileSync(args.f);
  file = args._[0];
} else {
  if (args._.length < 1) {
    usage()
  }
  src = args._[0];
  file = args._[1];
}

const p = new Parser(src);
const prog = p.parseProgram();

try {
  const e = new Evaluator(prog);

  if (file) {
    const json = JSON.parse(await Deno.readTextFile(file));
    await e.run(json);
  } else {
    const stdin = await Deno.readAll(Deno.stdin);
    const content = new TextDecoder().decode(stdin);
    const json = JSON.parse(content);
    await e.run(json);
  }

} catch (e) {
  console.error(e.message);
  Deno.exit(1);
}
