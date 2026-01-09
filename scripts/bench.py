import subprocess

f = open('bench_new.txt', 'w')
subprocess.run(['go', 'test', '-bench=.', '-count=10'], check=True, stdout=f)
