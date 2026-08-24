import { X509Certificate, createHash } from 'node:crypto';
import { execFile } from 'node:child_process';
import { chmod, mkdtemp, mkdir, readFile, rm, symlink, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { promisify } from 'node:util';

const execFileAsync = promisify(execFile);
const MAX_CA_BUNDLE_BYTES = 4 * 1024 * 1024;

export interface PreparedCATrust {
  homeDir: string;
  fingerprint: string;
  cleanup(): Promise<void>;
}

export async function loadCertificates(path: string): Promise<{ certificates: Buffer[]; fingerprint: string }> {
  let input: Buffer;
  try {
    input = await readFile(path);
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error);
    throw new Error(`Failed to read CA certificate "${path}": ${detail}`);
  }
  if (input.length === 0) throw new Error('CA certificate file is empty');
  if (input.length > MAX_CA_BUNDLE_BYTES) throw new Error('CA certificate file exceeds the 4 MiB limit');

  const text = input.toString('utf8');
  const pemBlocks = text.match(/-----BEGIN CERTIFICATE-----[\s\S]*?-----END CERTIFICATE-----/g);
  const candidates = pemBlocks?.length ? pemBlocks.map((pem) => Buffer.from(pem)) : [input];
  const certificates: Buffer[] = [];
  for (const candidate of candidates) {
    try {
      certificates.push(new X509Certificate(candidate).raw);
    } catch {
      throw new Error('CA certificate file must contain a DER certificate or PEM certificate bundle');
    }
  }
  if (certificates.length === 0) throw new Error('CA certificate file contains no certificates');

  const hash = createHash('sha256');
  for (const certificate of certificates) hash.update(certificate);
  return { certificates, fingerprint: hash.digest('hex') };
}

export async function prepareCATrust(path: string): Promise<PreparedCATrust> {
  if (process.platform !== 'linux') throw new Error('--ca-cert is currently supported only on Linux');
  const { certificates, fingerprint } = await loadCertificates(path);
  const homeDir = await mkdtemp(join(tmpdir(), 'cloak-agent-nss-'));
  await chmod(homeDir, 0o700);

  try {
    const pkiDir = join(homeDir, '.local', 'share', 'pki');
    const databaseDir = join(pkiDir, 'nssdb');
    await mkdir(databaseDir, { recursive: true });
    await symlink('.local/share/pki', join(homeDir, '.pki'));
    const database = `sql:${databaseDir}`;
    await runCertutil(['-N', '--empty-password', '-d', database], 'initialize the NSS database');

    for (let index = 0; index < certificates.length; index++) {
      const stagedPath = join(homeDir, `ca-${index}.der`);
      await writeFile(stagedPath, certificates[index], { mode: 0o600 });
      try {
        await runCertutil(
          ['-A', '-d', database, '-t', 'C,,', '-n', `cloak-agent-ca-${index}`, '-i', stagedPath],
          'import the CA certificate',
        );
      } finally {
        await rm(stagedPath, { force: true });
      }
    }
  } catch (error) {
    await rm(homeDir, { recursive: true, force: true });
    throw error;
  }

  return {
    homeDir,
    fingerprint,
    cleanup: () => rm(homeDir, { recursive: true, force: true }),
  };
}

async function runCertutil(args: string[], action: string): Promise<void> {
  try {
    await execFileAsync('certutil', args, { timeout: 15_000, maxBuffer: 1024 * 1024 });
  } catch (error: unknown) {
    if (isExecError(error) && error.code === 'ENOENT') {
      throw new Error(`Failed to ${action}: certutil is not installed. Install libnss3-tools on Debian/Ubuntu or nss-tools on RPM Linux.`);
    }
    const detail = isExecError(error) && typeof error.stderr === 'string' ? error.stderr.trim() : '';
    const fallback = error instanceof Error ? error.message : String(error);
    throw new Error(`Failed to ${action}: ${detail || fallback}`);
  }
}

function isExecError(error: unknown): error is Error & { code?: string; stderr?: string } {
  return error instanceof Error;
}
