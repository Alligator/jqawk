mod lexer;
mod compiler;
mod vm;

use lexer::Lexer;
use compiler::Compiler;
use vm::Vm;

use clap::{App, AppSettings, Arg};
use std::fs;
use std::fs::File;
use std::io::BufReader;

fn run_program_file<T>(path: &str, rdr: T, selector: &str)
    where T: std::io::Read {
    let content = fs::read_to_string(path)
        .expect("error reading program file");

    let lexer = Lexer::new(content.as_str());
    let mut compiler = Compiler::new(lexer);
    let rules = compiler.compile_rules();

    let s_lexer = Lexer::new(selector);
    let mut s_compiler = Compiler::new(s_lexer);
    let selector_program = s_compiler.compile_expression();

    let mut vm = Vm::new(false);
    vm.run(rdr, selector_program, rules);
}

fn main() {
    let matches = App::new("jqawk")
        .setting(AppSettings::AllowMissingPositional)
        .about("JSON and awk together at last")
        .version("v1")
        .arg(Arg::with_name("root")
            .short("r")
            .long("root")
            .help("an expression evaluated to find the root value")
            .takes_value(true)
            .default_value("$")
            .hide_default_value(true))
        .arg(Arg::with_name("program_file")
            .short("f")
            .help("a script file to run")
            .takes_value(true))
        .arg(Arg::with_name("PROGRAM")
            .help("the jawk program to run"))
        .arg(Arg::with_name("INPUT")
            .help("the input file"))
        .get_matches();

    let file = File::open(matches.value_of("INPUT").unwrap())
        .expect("error opening input file");
    let reader = BufReader::new(file);

    let selector = matches.value_of("root").unwrap();
    
    if matches.is_present("program_file") {
        run_program_file(matches.value_of("program_file").unwrap(), reader, selector);
    } else {
        let lexer = Lexer::new(matches.value_of("PROGRAM").unwrap());
        let mut compiler = Compiler::new(lexer);
        let rules = compiler.compile_rules();

        let s_lexer = Lexer::new(selector);
        let mut s_compiler = Compiler::new(s_lexer);
        let selector_program = s_compiler.compile_expression();

        let mut vm = Vm::new(false);
        vm.run(reader, selector_program, rules);
    }
}