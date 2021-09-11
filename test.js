import { writeAll, readAll } from "https://deno.land/std@0.106.0/io/util.ts";

const tests = [
  {
    name: 'begin',
    prog: 'BEGIN { print "hi"; }',
    expected: 'hi\n',
  },
  {
    name: 'print',
    prog: 'BEGIN { print "hi", "there", 123; }',
    expected: 'hi there 123\n',
  },
  {
    name: 'end',
    prog: 'BEGIN { print "start"; } END { print "end"; }',
    expected: 'start\nend\n',
  },
  {
    name: 'variables',
    prog: 'BEGIN { name = "alligator"; print name; }',
    expected: 'alligator\n',
  },
  {
    name: 'member',
    prog: '{ print $.name; }',
    input: '[{ "name": "one" }, { "name": "two" }]',
    expected: 'one\ntwo\n',
  },
  {
    name: 'key field',
    prog: '{ print $key; }',
    input: '{ "key one": 1, "key two": 2 }',
    expected: 'key one\nkey two\n',
  },
];

async function run(prog, input) {
  const p = Deno.run({
    cmd: ['deno', 'run', '-A', 'jqawk.js', prog],
    stdin: 'piped',
    stdout: 'piped',
    stderr: 'piped',
  });

  const buf = new TextEncoder().encode(input || '{}');
  await writeAll(p.stdin, buf);
  p.stdin.close();

  const outputBuf = await readAll(p.stdout);
  const output = new TextDecoder().decode(outputBuf);
  const status = await p.status();

  const errBuf = await readAll(p.stderr);
  const err = new TextDecoder().decode(errBuf);

  return {
    output,
    err,
    success: status.success,
  };
}

for (let i = 0; i < tests.length; i++) {
  const test = tests[i];
  const { output, success, err } = await run(test.prog, test.input);

  if (!success) {
    console.log(`!! - ${test.name} failed`);
    console.log(`  ${err.trim()}`);
    continue;
  }

  if (output === test.expected) {
    console.log(`ok - ${test.name}`);
  } else {
    console.log(`!! - ${test.name} failed`);
    console.log(`  expected: ${JSON.stringify(test.expected)}`);
    console.log(`       got: ${JSON.stringify(output)}`);
  }
}
