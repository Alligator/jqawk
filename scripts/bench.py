import subprocess
import sys
import os.path

benchmarks = [
    'README_example',
    'advent_of_code_example',
    'pre/postfix_operators',
    'compound_operators',
    'dot',
    'subscript',
    'subscript_array',
    'unknown_variable_comparison',
    'for',
    'for_in',
    'match_array',
    'match_nested_array',
    'deep_implicit_object_creation',
    'implicit_array_creation',
    'deep_implicit_array_creation',
    'implicit_object-in-array_creation',
    'printing_circular_references',
    'converting_circular_references_to_JSON',
    'is_operator',
    'array_sort',
    'recursion',
    'parseJson',
    'bug:_2d_arrays',
]

testpattern = f'BenchmarkJqawk/({"|".join(benchmarks)})$'

def run_bench(file):
    f = open(file, 'w')
    subprocess.run(['go', 'test', '-run=^$', f'-bench={testpattern}', '-count=10', '-benchmem'], check=True, stdout=f)

if len(sys.argv) < 3:
    print('usage: bench.py oldrev newrev')

oldrev = sys.argv[1]
newrev = sys.argv[2]

p = subprocess.run('git status --porcelain', shell=True, encoding='utf-8', stdout=subprocess.PIPE)
lines = p.stdout.strip().split('\n')
modlines = 0
for l in lines:
    if 'bench.py' in l:
        continue
    if l.strip().startswith('??'):
        continue
    modlines += 1

if modlines > 0:
    print('git changed detected, aborting')
    sys.exit(1)

oldfile = f'_bench_{oldrev}.txt'
newfile = f'_bench_{newrev}.txt'

if not os.path.isfile(oldfile):
    print(f'checking out {oldrev}')
    subprocess.run(f'git checkout {oldrev}', shell=True, check=True)
    print(f'benchmarking {oldrev}')
    run_bench(oldfile)
    print(f'done')
else:
    print(f'{oldfile} exists, skipping')

if not os.path.isfile(newfile):
    print(f'checking out {newrev}')
    subprocess.run(f'git checkout {newrev}', shell=True, check=True)
    print(f'benchmarking {newrev}')
    run_bench(newfile)
    print(f'done')
else:
    print(f'{newfile} exists, skipping')

subprocess.run(f'benchstat {oldfile} {newfile}', shell=True, check=True)
