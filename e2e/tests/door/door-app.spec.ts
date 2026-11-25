import { existsSync, readFileSync } from 'node:fs';
import type { Page } from '@playwright/test';
import { doorPhrases, expect, test, tr, type Locale } from '../../helpers';

// The check-in app (mobile/) as a phone-sized web build, against the real backend.
// Earlier screens stay in the page (hidden) under the current one, so text lookups use visible: true.
const d = (locale: Locale, en: string, p?: Record<string, string | number>) => tr(doorPhrases, locale, en, p);
const ticket = existsSync('.cache/ticket.json') ? JSON.parse(readFileSync('.cache/ticket.json', 'utf8')) as { name: string; code: string } : null;
const LANG = { kk: 'Қаз', ru: 'Рус', en: 'Eng' } as const;

// The web barcode decoder fetches its WebAssembly from a CDN; serve the installed copy so the
// test does not depend on outside network access.
test.beforeEach(async ({ context }) => {
  await context.route(/zxing_reader\.wasm/, (r) => r.fulfill({ path: `${process.env.MOBILE_DIR ?? '../mobile'}/node_modules/zxing-wasm/dist/reader/zxing_reader.wasm`, contentType: 'application/wasm' }));
});

async function signIn(page: Page, email: string, locale: Locale) {
  await page.goto('/');
  await page.getByRole('radio', { name: LANG[locale] }).click();
  await page.getByPlaceholder(d(locale, 'Email')).fill(email);
  await page.getByPlaceholder(d(locale, 'Password')).fill(process.env.DEMO_PASSWORD ?? 'demo12345');
  await page.getByText(d(locale, 'Sign in'), { exact: true }).filter({ visible: true }).click();
}

test('sign-in errors are clear and translated', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('radio', { name: LANG.ru }).click();
  await page.getByText(d('ru', 'Sign in'), { exact: true }).filter({ visible: true }).click();
  await expect(page.getByText(d('ru', 'Please enter your email and password.')).filter({ visible: true })).toBeVisible();
  await page.getByPlaceholder(d('ru', 'Email')).fill('staff@biletflow.kz');
  await page.getByPlaceholder(d('ru', 'Password')).fill('wrong-password');
  await page.getByText(d('ru', 'Sign in'), { exact: true }).filter({ visible: true }).click();
  await expect(page.getByText(d('ru', 'Email or password is incorrect.')).filter({ visible: true })).toBeVisible();
});

test('attendees who are not door staff cannot use the app', async ({ page }) => {
  await signIn(page, 'aigerim@example.kz', 'en');
  await expect(page.getByText(d('en', 'This app is only for event staff. Attendees can find their tickets on the BiletFlow website.')).filter({ visible: true })).toBeVisible();
});

test('door staff see only the events they work at, with live counts', async ({ page }) => {
  await signIn(page, 'staff@biletflow.kz', 'ru');
  await expect(page.getByText('Astana Jazz Night 2026').filter({ visible: true })).toBeVisible();
  await expect(page.getByText('NU Hackathon Kickoff').filter({ visible: true })).toBeVisible();
  await expect(page.getByText('Product Design Workshop').filter({ visible: true })).toHaveCount(0);
  await expect(page.getByText(new RegExp(`${d('ru', 'Checked in')}: \\d+ / \\d+`)).first()).toBeVisible();
});

test.describe('at the door', () => {
  test.skip(!existsSync('.cache/ticket.y4m') || !ticket, 'needs ffmpeg for the fake camera');

  test('a scan checks in once; undo, check in again, and a second scan is refused', async ({ page }) => {
    const ru = (en: string, p?: Record<string, string | number>) => d('ru', en, p);
    await signIn(page, 'staff@biletflow.kz', 'ru');
    await page.getByText('Astana Jazz Night 2026').filter({ visible: true }).click();
    await page.getByText(ru('Scan ticket'), { exact: true }).filter({ visible: true }).click();

    // The fake camera shows Arman's real ticket QR: the server admits it in one step.
    await expect(page.getByText(ru('Checked in'), { exact: true }).filter({ visible: true })).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText(ticket!.name).filter({ visible: true })).toBeVisible();
    await expect(page.getByText(ticket!.code).filter({ visible: true })).toBeVisible();

    await page.getByText(ru('Undo check-in'), { exact: true }).filter({ visible: true }).click();
    await expect(page.getByText(ru('Valid ticket'), { exact: true }).filter({ visible: true })).toBeVisible();
    await page.getByText(ru('Check in'), { exact: true }).filter({ visible: true }).click();
    await expect(page.getByText(ru('Checked in'), { exact: true }).filter({ visible: true })).toBeVisible();

    await page.getByText(ru('Scan next'), { exact: true }).filter({ visible: true }).click();
    await expect(page.getByText(ru('Already checked in'), { exact: true }).filter({ visible: true })).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText(new RegExp(ru('Checked in at {time}', { time: '\\d\\d:\\d\\d' })))).toBeVisible();
  });

  test('a ticket for another event is refused, in Kazakh, and undo is not offered', async ({ page }) => {
    const kk = (en: string) => d('kk', en);
    await signIn(page, 'staff@biletflow.kz', 'kk');
    await page.getByText('NU Hackathon Kickoff').filter({ visible: true }).click();
    await page.getByText(kk('Scan ticket'), { exact: true }).filter({ visible: true }).click();
    await expect(page.getByText(kk('Ticket is for a different event'), { exact: true }).filter({ visible: true })).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText(kk('Undo check-in'), { exact: true }).filter({ visible: true })).toHaveCount(0);
  });

  test('search finds the attendee and shows the same result screen', async ({ page }) => {
    const ru = (en: string) => d('ru', en);
    await signIn(page, 'staff@biletflow.kz', 'ru');
    await page.getByText('Astana Jazz Night 2026').filter({ visible: true }).click();
    await page.getByText(ru('Search attendee'), { exact: true }).filter({ visible: true }).click();
    await page.getByPlaceholder(ru('Name or ticket code')).fill(ticket!.code);
    await expect(page.getByText(ticket!.name).filter({ visible: true })).toBeVisible();
    await page.getByText(ticket!.name).filter({ visible: true }).click();
    await expect(page.getByText(ticket!.code).filter({ visible: true })).toBeVisible();
    await expect(page.getByText(ru('Already checked in'), { exact: true }).filter({ visible: true })).toBeVisible();
  });
});
