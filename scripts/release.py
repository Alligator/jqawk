# release script
# usage:
#   1. update the version in jqawk.go
#   2. push a version commit
#   3. create and push a tag
#   4. python scripts/release.py
#   5. create a release on github
#   6. upload the files in ./build

import subprocess
import os
import tarfile
import zipfile
import argparse
import sys
import re

targets = [
    ('darwin', 'arm64'),
    ('darwin', 'amd64'),

    ('linux', 'arm64'),
    ('linux', 'amd64'),
    ('linux', '386'),

    ('windows', 'arm64'),
    ('windows', 'amd64'),
    ('windows', '386'),
]

matches = re.search(r'var version = "([^"]*)"', open('jqawk.go', 'r').read())
if matches == None:
    print('couldn\'t find version')
    sys.exit(1)

version = matches.group(1)

def create_tar(file_to_tar, tar_name):
    t = tarfile.open(tar_name + '.tar.gz', 'w:gz')
    t.add(os.path.join('build', file_to_tar), arcname=file_to_tar)
    t.close()

def create_zip(file_to_zip, zip_name):
    with zipfile.ZipFile(zip_name + '.zip', 'w') as z:
        z.write(os.path.join('build', file_to_zip), arcname=file_to_zip)

def build_target(target):
    plat, arch = target

    binary = 'jqawk'
    if plat == 'windows':
        binary += '.exe'

    print(f'building {plat}-{arch}')

    env = os.environ.copy()
    env['GOOS'] = plat
    env['GOARCH'] = arch

    subprocess.run(['go', 'build', '-o', os.path.join('build', binary), '.'], check=True, env=env)
    if plat == 'windows':
        create_zip(binary, os.path.join('build', f'jqawk-{version}-{plat}-{arch}'))
    else:
        create_tar(binary, os.path.join('build', f'jqawk-{version}-{plat}-{arch}'))

try:
    os.mkdir('build')
except FileExistsError:
    pass

for target in targets:
    build_target(target)
