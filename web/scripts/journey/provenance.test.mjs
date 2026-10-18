import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { readFileSync } from 'node:fs';
import { parse } from 'yaml';
import { assertExportedSurface, assertPublishedFiles, verifyServed } from './lib/provenance.mjs';
const f = parse(readFileSync(new URL('./testdata/provenance.yaml', import.meta.url), 'utf8'));
assert.deepEqual(new Set(f.cases.map(c => c.name)), new Set(f.requiredNames));
for (const row of f.cases) test(row.name, { timeout: 5000 }, async t => {
  if (row.mode === 'package') { assert.throws(() => assertExportedSurface({}, {}), /PUBLISHED/); return; }
  const root = mkdtempSync(join(tmpdir(), 'peasant-provenance-')); t.after(() => rmSync(root, { recursive: true, force: true }));
  if (row.mode === 'bytes') {
    const a = join(root, 'installed'), b = join(root, 'published'); mkdirSync(a); mkdirSync(b);
    writeFileSync(join(a, 'ui.js'), 'source override'); writeFileSync(join(b, 'ui.js'), 'published export');
    assert.throws(() => assertPublishedFiles(a, b), /differ/); return;
  }
  mkdirSync(join(root, '_next'));
  const html = `<html>${row.mode === 'body' ? '' : 'village-page village observations'}<script src="/_next/app.js"></script></html>`;
  const asset = 'village-page /village';
  writeFileSync(join(root, 'village.html'), html); writeFileSync(join(root, '_next/app.js'), asset);
  const server = createServer((req, res) => { res.end(req.url === '/village' ? (row.mode === 'stale' ? 'old body' : html) : (row.mode === 'asset' ? 'old asset' : asset)); });
  t.after(async () => { server.closeAllConnections(); await new Promise(done => server.close(done)); });
  await new Promise(done => server.listen(0, '127.0.0.1', done));
  const check = verifyServed(`http://127.0.0.1:${server.address().port}`, root, process.execPath, process.pid);
  if (row.mode === 'exact') assert.equal((await check).assets[0], '/_next/app.js');
  else await assert.rejects(check, /differs|absent/);
});
