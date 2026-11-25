import { readFileSync } from 'node:fs';
import { expect, test as base, type Page } from '@playwright/test';

export const WEB_URL = process.env.WEB_URL ?? 'http://localhost:3000';
export const PASSWORD = process.env.DEMO_PASSWORD ?? 'demo12345';
export type Locale = 'kk' | 'ru' | 'en';

type Table = Record<string, { kk: string; ru: string }>;

/** The web app's phrase table (frontend/src/lib/phrases.ts), so tests can find labels in any language. */
export const webPhrases: Table = (() => {
  const src = readFileSync(new URL('../frontend/src/lib/phrases.ts', import.meta.url), 'utf8');
  const out: Table = {};
  for (const m of src.matchAll(/^ {2}("(?:[^"\\]|\\.)*"): \{ kk: ("(?:[^"\\]|\\.)*"), ru: ("(?:[^"\\]|\\.)*") \},$/gm)) {
    out[JSON.parse(m[1])] = { kk: JSON.parse(m[2]), ru: JSON.parse(m[3]) };
  }
  return out;
})();

/** The check-in app's phrase table (mobile/src/i18n.tsx). */
export const doorPhrases: Table = (() => {
  const src = readFileSync(new URL('../mobile/src/i18n.tsx', import.meta.url), 'utf8');
  const out: Table = {};
  const str = `'((?:[^'\\\\]|\\\\.)*)'`;
  for (const m of src.matchAll(new RegExp(`^\\s*${str}:\\s*\\{\\s*kk:\\s*${str},\\s*ru:\\s*${str}\\s*\\}`, 'gm'))) {
    out[m[1]] = { kk: m[2], ru: m[3] };
  }
  return out;
})();

export function tr(table: Table, locale: Locale, en: string, params: Record<string, string | number> = {}): string {
  let s = locale === 'en' ? en : table[en]?.[locale];
  if (s === undefined) throw new Error(`No ${locale} translation for "${en}"`);
  for (const [k, v] of Object.entries(params)) s = s.split(`{${k}}`).join(String(v));
  return s;
}
export const w = (locale: Locale, en: string, params?: Record<string, string | number>) => tr(webPhrases, locale, en, params);

/** Signs in through the web form, then switches the interface (and the account) to `locale`. */
export async function signIn(page: Page, email: string, locale: Locale) {
  await page.context().addCookies([{ name: 'bf_locale', value: locale, url: WEB_URL }]);
  await page.goto('/login');
  await page.locator('input[type=email]').fill(email);
  await page.locator('input[type=password]').fill(PASSWORD);
  await page.locator('form button[type=submit]').click();
  await expect(page).not.toHaveURL(/\/login/);
  await page.locator('header select').first().selectOption(locale);
}

export async function apiToken(email: string): Promise<string> {
  const r = await fetch(`${WEB_URL}/api/auth/login`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ email, password: PASSWORD }) });
  if (!r.ok) throw new Error(`Cannot sign in ${email} (${r.status}). Is the demo data loaded (scripts/seed_demo.py)?`);
  return (await r.json()).token;
}

export async function apiGet<T>(token: string, path: string): Promise<T> {
  const r = await fetch(`${WEB_URL}${path}`, { headers: { Authorization: `Bearer ${token}` } });
  if (!r.ok) throw new Error(`GET ${path}: ${r.status}`);
  return r.json() as Promise<T>;
}

export async function apiPost<T>(token: string, path: string, body: unknown): Promise<T> {
  const r = await fetch(`${WEB_URL}${path}`, { method: 'POST', headers: { Authorization: `Bearer ${token}`, 'content-type': 'application/json' }, body: JSON.stringify(body) });
  if (!r.ok) throw new Error(`POST ${path}: ${r.status}`);
  return r.json() as Promise<T>;
}

/** Every test fails if the page throws or logs an error (expected 4xx responses aside). */
export const test = base.extend<{ pageErrors: string[] }>({
  pageErrors: [async ({ page }, use) => {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(`pageerror: ${e.message}`));
    page.on('console', (m) => {
      if (m.type() === 'error' && !/status of 4\d\d/.test(m.text())) errors.push(`console: ${m.text().slice(0, 300)}`);
    });
    await use(errors);
    expect(errors, 'browser errors').toEqual([]);
  }, { auto: true }],
});
export { expect };
