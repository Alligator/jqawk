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
    return { type };
  }
  stringToken(type, str) {
    return { type, str };
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
      case '.': return this.simpleToken('dot');
      case ';': return this.simpleToken('semicolon');
      case '=': {
        if (this.peek() === '=') {
          this.advance();
          return this.simpleToken('equalequal');
        }
        break;
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

    const rules = [];
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
      rules.push({ pattern, body });
    }

    return rules;
  }
}

function jsonVal(value) {
  return { type: 'json', value };
}

function val(value) {
  return { value };
}

class Evaluator {
  constructor(prog) {
    this.prog = prog;
    this.pos = 0;
  }

  // hand rolled pratt parser
  // this should probably be in it's own class...
  evaluate(context, tokens, startType) {
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
        current = { type: 'eof' };
      }
    };

    const consume = (type) => {
      if (current.type !== type) {
        throw new Error(`expected ${type} but got ${current.type}`);
      }
      advance();
    };

    // grammar
    const printStatement = () => {
      consume('print');
      const arg = expression(precedence.assign);
      console.log(arg.value);
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
      if (left.type !== 'json' || typeof left.value !== 'object') {
        throw new Error('cannot access member of non object or non JSON value');
      }
      return jsonVal(left.value[prev.str]);
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

      throw new Error(`unknown operator ${token.type}`);
    };

    const identifier = () => {
      return context;
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
          prefix: identifier,
        },
        equalequal: {
          prec: precedence.comp,
          infix: binary,
        },
        plus: {
          prec: precedence.add,
          infix: binary,
        },
      }[type] || { prec: precedence.none };
      return r;
    };

    const expression = (prec) => {
      const token = current;
      const rule = getRule(token.type);
      if (!rule.prefix) {
        throw new Error(`unexpected prefix ${token.type}`);
      }

      advance();
      let left = rule.prefix();

      while (prec <= getRule(current.type).prec) {
        const infixRule = getRule(current.type);
        if (!infixRule.infix) {
          throw new Error(`unexpected infix ${current.type}`);
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
      throw new Error(`dangling tokens ${JSON.stringify(tokens.slice(pos, pos + 5))}`);
    }
    return result;
  }

  forEachRecord(cb) {
    while (this.pos < this.json.length) {
      cb(this.json[this.pos++]);
    }
  }

  async run(file) {
    this.json = JSON.parse(await Deno.readTextFile(file));

    this.forEachRecord((record) => {
      this.prog.forEach(({ pattern, body }) => {
        const recordVal = jsonVal(record);
        let result;
        if (pattern.length === 0) {
          result = { value: true };
        } else {
          result = this.evaluate(recordVal, pattern, 'expression');
        }
        if (result.value) {
          this.evaluate(recordVal, body, 'statement');
        }
      });
    });
  }
}

const src = Deno.readTextFileSync(Deno.args[0]);
const p = new Parser(src);
const prog = p.parseProgram();

const e = new Evaluator(prog);
e.run(Deno.args[1]);
