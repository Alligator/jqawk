mod lexer;
mod compiler;
mod vm;

use lexer::Lexer;
use compiler::Compiler;
use vm::Vm;

fn main() {
    let src = "
        $.name == \"alligator\" { print \"its alligator\"; }
        { print $.name; }";
    let lexer = Lexer::new(src);

    let mut compiler = Compiler::new(lexer);
    let rules = compiler.compile_rules();

    let mut vm = Vm::new(rules, false);
    vm.run("[{ \"name\": \"alligator\" }, { \"name\": \"tony\" }]");
}
