import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const binary = resolve(process.argv[2] ?? './cloak-agent');
const directory = mkdtempSync(join(tmpdir(), 'cloak-audit-'));
const session = `audit-${process.pid}`;
const env = { ...process.env, CLOAK_AGENT_INSTALL_DIR: join(directory, 'state'), CLOAK_AGENT_SOCKET_DIR: join(directory, 'run') };
const cli = (args, input) => spawnSync(binary, ['--quiet', '--session', session, '--output', 'json', ...args], {
  env, input, encoding: 'utf8', timeout: 15000, maxBuffer: 8 * 1024 * 1024,
});
const data = (args, input) => {
  const result = cli(args, input);
  assert.equal(result.status, 0, result.stderr || result.error?.message);
  const response = JSON.parse(result.stdout);
  assert.equal(response.ok, true);
  return response.data;
};
let passed = 0;
const check = (name, run) => { run(); passed++; console.log(`PASS ${name}`); };
try {
  check('schema distinguishes read-only discovery from tool execution', () => {
    assert.equal(data(['schema', 'webmcp_list']).schema._meta.readonly, true);
    assert.equal(data(['schema', 'webmcp_call']).schema._meta.mutating, true);
  });
  check('invalid CLI input fails with structured error on stderr', () => {
    const result = cli(['record', 'start', 'invalid.webm', '--fps', '0']);
    assert.equal(result.status, 64);
    assert.equal(result.stdout, '');
    assert.equal(typeof JSON.parse(result.stderr).error, 'object');
  });
  check('malformed structured input fails without execution', () => {
    const result = cli(['--input', 'json'], '{');
    assert.equal(result.status, 64);
    JSON.parse(result.stderr);
  });
  check('dry-run does not launch a browser or stop the daemon', () => {
    const result = data(['--dry-run', 'record', 'start', 'unused.webm']);
    assert.equal(result.dryRun, true);
    assert.equal(data(['--dry-run', 'close']).dryRun, true);
    assert.equal(data(['{"action":"runtime_status"}']).launched, false);
  });
  check('extra stop arguments are rejected', () => {
    assert.equal(cli(['record', 'stop', 'extra']).status, 64);
  });
  check('raw JSON and stdin preserve request identifiers', () => {
    const input = JSON.stringify({ id: 'audit-proof', action: 'runtime_status' });
    for (const response of [cli([input]), cli(['--input', 'json'], input)]) {
      assert.equal(response.status, 0, response.stderr);
      assert.equal(JSON.parse(response.stdout).id, 'audit-proof');
    }
  });
  console.log(`${passed} behavioral checks passed`);
} finally {
  cli(['daemon', 'stop']);
  rmSync(directory, { recursive: true, force: true });
}
