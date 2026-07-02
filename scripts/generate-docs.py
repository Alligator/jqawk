import subprocess
import os
from pathlib import Path

root = Path(__file__).resolve().parents[1]
docs_root = root / 'docs'

# gen docs for --docs
cli_out = root / 'cli' / 'docs'
docs_src = docs_root / 'docs.md'
subprocess.check_call(['pandoc', '-s', '-t', 'man', str(docs_src), '-o', str(cli_out / 'jqawk.1')])
subprocess.check_call([
    'pandoc',
    '-s',
    '-t', 'html',
    '-M', 'document-css=true',
    '--embed-resources=true',
    '--css', str(docs_root / 'style.css'),
    str(docs_src),
    '-o', str(cli_out / 'jqawk.html'),
])

# gen website index
os.chdir(root)
subprocess.check_call([
    'pandoc',
    '-s',
    '-t', 'html',
    '-f', 'gfm',
    '-M', 'document-css=true',
    '-V', 'header-includes=<link rel="icon" href="docs/favicon.svg" type="image/svg+xml">',
    '-V', 'pagetitle=jqawk - awk + JSON',
    '--embed-resources=true',
    '--syntax-highlighting=zenburn',
    '--css', str(docs_root / 'style.css'),
    str(root / 'README.md'),
    '-o', str(docs_root / 'index.html'),
])
