import subprocess
import argparse

parser = argparse.ArgumentParser()
parser.add_argument('-p', '--parallel', type=int, required=True)
parser.add_argument('-t', '--fuzztime', default='30m')
parser.add_argument('-f', default='FuzzJqawkWithJson')

args = parser.parse_args()

goargs = ['go', 'test', f'-parallel={args.parallel}', f'-fuzz={args.f}']
if args.fuzztime is not None:
    goargs.append(f'-fuzztime={args.fuzztime}')

try:
    subprocess.check_call(goargs)
except KeyboardInterrupt:
    pass
