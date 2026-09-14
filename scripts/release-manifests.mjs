import { readFile, mkdir, writeFile } from 'node:fs/promises';
const version = process.argv[2];
if (!/^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.+-]+)?$/.test(version ?? '')) {
  throw new Error('A release or snapshot version is required');
}
const daemon = new URL('../daemon/', import.meta.url);
const output = new URL('.release/', daemon);
await mkdir(output, { recursive: true });
for (const name of ['package.json', 'package-lock.json']) {
  const manifest = JSON.parse(await readFile(new URL(name, daemon), 'utf8'));
  manifest.version = version;
  if (manifest.packages?.['']) manifest.packages[''].version = version;
  await writeFile(new URL(name, output), JSON.stringify(manifest, null, 2) + '\n');
}
