import subprocess
subprocess.run(['go', 'test', '-coverprofile=coverage.out', '-coverpkg=./...'], check=True)
subprocess.run(['go', 'tool', 'cover', '-html=coverage.out'], check=True)
