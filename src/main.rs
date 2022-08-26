mod lexer;
mod compiler;
mod vm;
mod debug;

use crate::debug::print_rules;
use lexer::Lexer;
use compiler::Compiler;
use vm::Vm;

use clap::{App, Arg, ArgMatches};
use atty;
use std::fs;
use std::fs::File;
use std::io;

fn run_program_file<T>(path: &str, rdr: T, selector: &str)
    where T: std::io::Read {
    let content = fs::read_to_string(path)
        .expect("error reading program file");

    let lexer = Lexer::new(content.as_str());
    let mut compiler = Compiler::new(lexer);
    let rules = compiler.compile_rules().unwrap();

    let s_lexer = Lexer::new(selector);
    let mut s_compiler = Compiler::new(s_lexer);
    let selector_program = s_compiler.compile_expression().unwrap();

    let mut vm = Vm::new(false);
    let result = vm.run(rdr, selector_program, rules);
    if result.is_err() {
        let err = result.unwrap_err();
        eprintln!("runtime error: {}", err.msg);
    }
}

fn get_input(matches: &ArgMatches) -> Box<dyn io::Read> {
    if matches.is_present("INPUT") {
        let file = File::open(matches.value_of("INPUT").unwrap())
            .expect("error opening input file");
        return Box::new(file);
    }

    if atty::isnt(atty::Stream::Stdin) {
        return Box::new(io::stdin());
    }

    return Box::new("{}".as_bytes());
}

fn main() {
    let matches = App::new("jqawk")
        .about("JSON and awk together at last")
        .version("v1")
        .arg(Arg::with_name("root")
            .help("an expression evaluated to find the root value")
            .short("r")
            .long("root")
            .takes_value(true)
            .default_value("$")
            .hide_default_value(true))
        .arg(Arg::with_name("debug")
            .long("debug"))
        .arg(Arg::with_name("program_file")
            .short("f")
            .help("a script file to run")
            .takes_value(true))
        .arg(Arg::with_name("PROGRAM")
            .help("the jqawk program to run")
            .conflicts_with("program_file"))
        .arg(Arg::with_name("INPUT")
            .help("the input file"))
        .get_matches();

    let selector = matches.value_of("root").unwrap();
    let reader = io::BufReader::new(get_input(&matches));
    
    if matches.is_present("program_file") {
        run_program_file(matches.value_of("program_file").unwrap(), reader, selector);
    } else {
        let lexer = Lexer::new(matches.value_of("PROGRAM").unwrap());
        let mut compiler = Compiler::new(lexer);
        let rules = compiler.compile_rules();
        if !rules.is_ok() {
            let err = rules.unwrap_err();
            eprintln!("error on line {}: {}", err.line, err.msg);
            return;
        }

        let s_lexer = Lexer::new(selector);
        let mut s_compiler = Compiler::new(s_lexer);
        let selector_program = s_compiler.compile_expression();
        if !selector_program.is_ok() {
            let err = selector_program.unwrap_err();
            eprintln!("error on line {}: {}", err.line, err.msg);
            return;
        }

        let unwrapped_rules = rules.unwrap();
        if matches.is_present("debug") {
            print_rules(&unwrapped_rules);
        }

        let mut vm = Vm::new(false);
        let result = vm.run(reader, selector_program.unwrap(), unwrapped_rules);
        if result.is_err() {
            let err = result.unwrap_err();
            eprintln!("runtime error: {}", err.msg);
        }
    }
}