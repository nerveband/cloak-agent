import { execFile } from 'node:child_process';
import { access, chmod, mkdir, mkdtemp, readFile, rm, stat, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { promisify } from 'node:util';
import { afterEach, describe, expect, it } from 'vitest';
import { loadCertificates, prepareCATrust } from '../src/ca-trust.js';
import type { PreparedCATrust } from '../src/ca-trust.js';

const execFileAsync = promisify(execFile);
const cleanupPaths: string[] = [];

afterEach(async () => {
  await Promise.all(cleanupPaths.splice(0).map(async (path) => {
    await rm(path, { recursive: true, force: true });
  }));
});

async function createCertificate(directory: string, name: string): Promise<string> {
  const keyPath = join(directory, `${name}.key`);
  const certPath = join(directory, `${name}.pem`);
  await execFileAsync('openssl', [
    'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
    '-subj', `/CN=${name}`, '-keyout', keyPath, '-out', certPath,
  ]);
  return certPath;
}

async function installFakeCertutil(directory: string): Promise<string> {
  const binDir = join(directory, 'bin');
  await mkdir(binDir);
  const executable = join(binDir, 'certutil');
  await writeFile(executable, '#!/bin/sh\nexit 0\n');
  await chmod(executable, 0o755);
  return binDir;
}

describe('CA trust', () => {
  it('loads PEM bundles and fingerprints certificate content', async () => {
    const directory = await mkdtemp(join(tmpdir(), 'cloak-ca-test-'));
    cleanupPaths.push(directory);
    const first = await createCertificate(directory, 'first');
    const second = await createCertificate(directory, 'second');
    const bundle = join(directory, 'bundle.pem');
    await writeFile(bundle, Buffer.concat([await readFile(first), await readFile(second)]));

    const loaded = await loadCertificates(bundle);
    expect(loaded.certificates).toHaveLength(2);
    expect(loaded.fingerprint).toMatch(/^[a-f0-9]{64}$/);
    expect((await loadCertificates(bundle)).fingerprint).toBe(loaded.fingerprint);
  });

  it('accepts DER certificates and rejects malformed input', async () => {
    const directory = await mkdtemp(join(tmpdir(), 'cloak-ca-test-'));
    cleanupPaths.push(directory);
    const pem = await createCertificate(directory, 'der');
    const der = join(directory, 'ca.der');
    await execFileAsync('openssl', ['x509', '-in', pem, '-outform', 'DER', '-out', der]);
    expect((await loadCertificates(der)).certificates).toHaveLength(1);

    const malformed = join(directory, 'bad.pem');
    await writeFile(malformed, 'not a certificate');
    await expect(loadCertificates(malformed)).rejects.toThrow('DER certificate or PEM certificate bundle');
  });

  it('creates a private isolated NSS home and removes it on cleanup', async () => {
    const directory = await mkdtemp(join(tmpdir(), 'cloak-ca-test-'));
    cleanupPaths.push(directory);
    const certPath = await createCertificate(directory, 'private');
    const binDir = await installFakeCertutil(directory);
    const previousPath = process.env.PATH;
    process.env.PATH = `${binDir}:${previousPath ?? ''}`;
    let prepared: PreparedCATrust | undefined;
    try {
      prepared = await prepareCATrust(certPath);
      expect((await stat(prepared.homeDir)).mode & 0o777).toBe(0o700);
      await access(join(prepared.homeDir, '.local', 'share', 'pki', 'nssdb'));
      const homeDir = prepared.homeDir;
      await prepared.cleanup();
      await expect(access(homeDir)).rejects.toThrow();
    } finally {
      process.env.PATH = previousPath;
      await prepared?.cleanup();
    }
  });

  it('reports a missing certutil without retaining the temporary home', async () => {
    const directory = await mkdtemp(join(tmpdir(), 'cloak-ca-test-'));
    cleanupPaths.push(directory);
    const certPath = await createCertificate(directory, 'missing-tool');
    const previousPath = process.env.PATH;
    process.env.PATH = directory;
    try {
      await expect(prepareCATrust(certPath)).rejects.toThrow('certutil is not installed');
    } finally {
      process.env.PATH = previousPath;
    }
  });
});
