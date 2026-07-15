import subprocess
import argparse

parser = argparse.ArgumentParser()
parser.add_argument('-p', '--parallel', type=int, required=True)
parser.add_argument('-t', '--fuzztime', default='30m')

args = parser.parse_args()

goargs = ['go', 'test', f'-parallel={args.parallel}', '-fuzz=FuzzJqawkWithJson']
if args.fuzztime is not None:
    goargs.append(f'-fuzztime={args.fuzztime}')

try:
    subprocess.check_call(goargs)
except KeyboardInterrupt:
    pass
