// Lists every interface phrase that lacks a Kazakh or Russian translation.
// Phrases come from: t('…') calls; label/helpText/placeholder/title/hint props; label: '…' and
// 'key': '…' maps in components; and the English messages the Go and Python services can return.
// Usage: node scripts/i18n-check.mjs [--list] [--json]
import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const repo = join(root, '..');

function walk(dir, exts, out = []) {
  if (!existsSync(dir)) return out;
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (name === 'node_modules' || name.startsWith('.')) continue;
    if (statSync(p).isDirectory()) walk(p, exts, out);
    else if (exts.some((e) => p.endsWith(e)) && !p.endsWith('_test.go') && !p.includes('/tests/')) out.push(p);
  }
  return out;
}

const unescape = (s) => s.replace(/\\'/g, "'").replace(/\\"/g, '"');
const found = new Map();
const add = (phrase, where) => {
  const p = unescape(phrase).trim();
  if (!p || ['muted', 'success', 'default', 'error'].includes(p) || !/[A-Za-z]/.test(p) || /^[A-Z0-9_]+$/.test(p) || p.startsWith('/') || p.startsWith('http')) return;
  if (!found.has(p)) found.set(p, where);
};

const ui = [
  /\bt\(\s*'((?:[^'\\]|\\.)+)'/g, /\bt\(\s*"((?:[^"\\]|\\.)+)"/g,
  /\b(?:label|helpText|placeholder|title|hint|submitLabel|detailsLabel|emptyTitle|emptyDescription|eyebrow|description)="([^"]+)"/g,
  /\b(?:label|title|hint|message|emptyTitle|emptyDescription|description)=\{'((?:[^'\\]|\\.)+)'\}/g,
  /\blabel:\s*'((?:[^'\\]|\\.)+)'/g,
  // Default values of label-like props: ({ detailsLabel = 'View details' }).
  /\b(?:\w*Label|emptyTitle|emptyDescription)\s*=\s*'((?:[^'\\]|\\.)+)'/g,
  // Ternaries inside t(...) or label: …: both branches are phrases.
  /(?:\bt\(|\blabel:)[^\n]*?\?\s*'((?:[^'\\]|\\.)+)'\s*:/g,
  /(?:\bt\(|\blabel:)[^\n]*?\?\s*'(?:[^'\\]|\\.)+'\s*:\s*'((?:[^'\\]|\\.)+)'/g,
  /\bt\([^\n]*?\?\s*'(?:[^'\\]|\\.)+'\s*:\s*[^'\n]*?\?\s*'((?:[^'\\]|\\.)+)'\s*:\s*'((?:[^'\\]|\\.)+)'/g,
  /'[a-z_]+(?:\.[a-z_]+)+':\s*'((?:[^'\\]|\\.)+)'/g,
  /\[\s*'[a-z_]*',\s*'([A-Z][^']+)'\s*\]/g,
  /\[\s*'([A-Z][a-z][^']*)',\s*(?:type|String|money|c\.)/g,
  /errors\.\w+\s*=\s*'((?:[^'\\]|\\.)+)'/g,
  /return\s+'([A-Z][^']+\.)';/g,
  /setFormError\('((?:[^'\\]|\\.)+)'\)/g, /setMessage\('((?:[^'\\]|\\.)+)'\)/g,
  /^\s+\d{3}:\s*'((?:[^'\\]|\\.)+)',/gm,
  /new ApiRequestError\('((?:[^'\\]|\\.)+)'/g,
  /\b(?:ticket_delivery|payment|refund|event_information|check_in|account|activation|technical|upcoming|active|draft|completed|cancelled|attendee|organizer|event_admin|platform_admin):\s*'((?:[^'\\]|\\.)+)'/g,
];
for (const file of walk(join(root, 'src'), ['.ts', '.tsx'])) {
  if (file.endsWith('phrases.ts') || file.endsWith('api-types.ts')) continue;
  const src = readFileSync(file, 'utf8');
  for (const re of ui) for (const m of src.matchAll(re)) for (const g of m.slice(1)) if (g) add(g, file.replace(root + '/', ''));
}
// Statuses and enum values shown through Badge (snake_case → "Sentence case").
for (const v of ['pending', 'completed', 'failed', 'expired', 'cancelled', 'refunded', 'valid', 'checked_in', 'published', 'draft', 'public', 'unlisted', 'private',
  'open', 'in_progress', 'waiting_for_customer', 'resolved', 'active', 'disabled', 'inactive', 'suspended', 'succeeded', 'actioned', 'dismissed', 'upcoming']) {
  add(v.replace(/_/g, ' ').replace(/^./, (c) => c.toUpperCase()), 'enum');
}
// Server messages the clients display.
for (const file of walk(join(repo, 'backend-go', 'internal'), ['.go'])) {
  const src = readFileSync(file, 'utf8');
  for (const m of src.matchAll(/(?:BadRequest|Conflict|Forbidden|Err\([^,]+)\(?\s*"[A-Z_]+",\s*"((?:[^"\\]|\\.)+)"/g)) add(m[1], 'go');
  for (const m of src.matchAll(/NotFound\("((?:[^"\\]|\\.)+)"\)/g)) add(m[1], 'go');
  for (const m of src.matchAll(/Err\(http\.\w+,\s*"[A-Z_]+",\s*"((?:[^"\\]|\\.)+)"/g)) add(m[1], 'go');
  for (const m of src.matchAll(/"(?:CARD_DECLINED|INSUFFICIENT_FUNDS)":\s*return\s*"([^"]+)"/g)) add(m[1], 'go');
  for (const m of src.matchAll(/return "((?:Card declined|Insufficient funds|Invalid card)[^"]*)"/g)) add(m[1], 'go');
}
for (const file of walk(join(repo, 'backend-python', 'app'), ['.py'])) {
  const src = readFileSync(file, 'utf8');
  for (const m of src.matchAll(/HTTPException\(\s*\d{3},\s*"((?:[^"\\]|\\.)+)"/g)) add(m[1], 'python');
  for (const m of src.matchAll(/"(Invalid or missing access token|Insufficient permissions|Request validation failed)"/g)) add(m[1], 'python');
}

const { phrases } = await import(join(root, 'src/lib/phrases.ts')).catch(async () => {
  // Node cannot import .ts directly: evaluate the object literal.
  const text = readFileSync(join(root, 'src/lib/phrases.ts'), 'utf8');
  const body = text.slice(text.indexOf('= {') + 2, text.lastIndexOf('};') + 1);
  return { phrases: Function(`return (${body});`)() };
});

// Visible JSX text that is not wrapped in t() would stay English in every language.
const unwrapped = [];
const SKIP_TEXT = new Set(['BiletFlow', 'B', 'BF', '₸', '×', '−', '|', '→', '←', '—', '·', '✓', '+', '-']);
for (const file of walk(join(root, 'src'), ['.tsx'])) {
  const src = readFileSync(file, 'utf8');
  for (const m of src.matchAll(/(?<![=-])>\s*([^<>{}\n]*[A-Za-zА-Яа-я][^<>{}\n]*?)\s*<(?=[/A-Za-z])/g)) {
    const text = m[1].trim();
    if (/[?:;=()'"`&|]/.test(text) || /^[A-Z]\w*$/.test(text) && /Promise|Record|Array/.test(text) || SKIP_TEXT.has(text) || /^[\d\s.,:%₸()+\-–—·×/]*$/.test(text) || text.includes('=>') || text.startsWith('//')) continue;
    unwrapped.push(`${file.replace(root + '/', '')}: "${text}"`);
  }
}
if (unwrapped.length) {
  console.log(`Untranslated JSX text (${unwrapped.length}):`);
  for (const u of unwrapped) console.log('  ' + u);
}

const missing = [...found.keys()].filter((p) => !phrases[p] || !phrases[p].kk || !phrases[p].ru).sort();
const unused = Object.keys(phrases).filter((p) => !found.has(p));
if (process.argv.includes('--json')) {
  console.log(JSON.stringify(missing, null, 1));
} else {
  if (process.argv.includes('--list')) for (const m of missing) console.log(`${m}\t(${found.get(m)})`);
  console.log(`${found.size} phrases in use, ${found.size - missing.length} translated, ${missing.length} missing, ${unused.length} not found in code.`);
}
process.exit(missing.length || unwrapped.length ? 1 : 0);
