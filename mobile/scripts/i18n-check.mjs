// Lists English phrases in the check-in app without a Kazakh or Russian translation (src/i18n.tsx).
// Phrases come from t('…') calls, result titles and status labels, error messages raised with
// new ApiError(code, '…'), and the error messages of the Go check-in endpoints.
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const files = (dir) => readdirSync(dir).flatMap((f) => {
  const p = join(dir, f);
  if (statSync(p).isDirectory()) return f === '__tests__' ? [] : files(p);
  return /\.tsx?$/.test(f) ? [p] : [];
});

const used = new Map();
const add = (p, where) => { if (!used.has(p)) used.set(p, where.replace(root + '/', '')); };
for (const f of files(join(root, 'src'))) {
  if (f.endsWith('mock.ts')) continue; // fake data for demos and tests
  const src = readFileSync(f, 'utf8');
  for (const m of src.matchAll(/\bt\(\s*'((?:[^'\\]|\\.)+)'/g)) add(m[1].replace(/\\'/g, "'"), f);
  for (const m of src.matchAll(/\b(?:title|label):\s*'([^']+)'/g)) add(m[1], f);
  for (const m of src.matchAll(/new ApiError\(\s*'[^']*',\s*'((?:[^'\\]|\\.)+)'/g)) add(m[1], f);
  for (const m of src.matchAll(/^const [A-Z_]+ =\s*\n?\s*'([A-Z][^']* [^']+)';/gm)) add(m[1], f); // message constants
}
const go = join(root, '../backend-go/internal/checkin/checkin.go');
if (existsSync(go)) {
  for (const m of readFileSync(go, 'utf8').matchAll(/platform\.(?:NotFound|Conflict|Forbidden)\((?:"[A-Z_]+",\s*)?"([^"]+)"\)/g)) add(m[1], 'checkin.go');
}

const table = readFileSync(join(root, 'src/i18n.tsx'), 'utf8');
const known = new Set([...table.matchAll(/^\s*'((?:[^'\\]|\\.)+)':\s*\{\s*kk:/gm)].map((m) => m[1].replace(/\\'/g, "'")));
const missing = [...used.keys()].filter((p) => !known.has(p)).sort();
console.log(`${used.size} phrases in use, ${used.size - missing.length} translated, ${missing.length} missing.`);
for (const m of missing) console.log(`  ${m}\t(${used.get(m)})`);
process.exit(missing.length ? 1 : 0);
