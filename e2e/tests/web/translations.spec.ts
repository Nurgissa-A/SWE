import type { Page } from '@playwright/test';
import { expect, signIn, test, WEB_URL, webPhrases, type Locale } from '../../helpers';

// English phrases that have a different Kazakh/Russian translation must never show on kk/ru pages.
const english = new Set(Object.entries(webPhrases).filter(([en, v]) => v.kk !== en || v.ru !== en).map(([en]) => en));
// Organizer-entered names in the demo data that happen to match a UI phrase; they are not UI text.
for (const name of ['Free registration']) english.delete(name);

async function visibleEnglish(page: Page): Promise<string[]> {
  await page.waitForLoadState('networkidle');
  const texts = await page.evaluate(() => {
    const out: string[] = [];
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    while (walker.nextNode()) {
      const el = walker.currentNode.parentElement;
      if (el && !el.closest('script,style,nextjs-portal') && el.offsetParent !== null) out.push(walker.currentNode.textContent!.trim());
    }
    for (const el of document.querySelectorAll('[placeholder],[aria-label],option')) out.push(el.getAttribute('placeholder') ?? el.getAttribute('aria-label') ?? el.textContent!.trim());
    return out;
  });
  return [...new Set(texts.filter((t) => english.has(t)))];
}

const journeys: { who: string | null; paths: string[] }[] = [
  { who: null, paths: ['/', '/events', '/login', '/register', '/forgot-password'] },
  { who: 'aigerim@example.kz', paths: ['/events', '/orders', '/support', '/account'] },
  { who: 'organizer@biletflow.kz', paths: ['/dashboard', '/dashboard/events', '/dashboard/events/new', '/support'] },
  { who: 'admin@biletflow.kz', paths: ['/admin'] },
];

for (const locale of ['kk', 'ru'] as Locale[]) {
  for (const { who, paths } of journeys) {
    test(`no English left on ${who ?? 'public'} pages in ${locale}`, async ({ page }) => {
      if (who) await signIn(page, who, locale);
      else await page.context().addCookies([{ name: 'bf_locale', value: locale, url: WEB_URL }]);
      const leaks: string[] = [];
      for (const path of paths) {
        await page.goto(path);
        for (const t of await visibleEnglish(page)) leaks.push(`${path}: ${t}`);
      }
      if (who === 'organizer@biletflow.kz') {
        await page.goto('/dashboard/events');
        const hub = await page.locator('a[href^="/dashboard/events/"]:not([href$="/new"])').first().getAttribute('href');
        for (const tab of ['overview', 'tickets', 'paid', 'orders', 'promo', 'staff', 'history', 'details']) {
          await page.goto(`${hub}?tab=${tab}`);
          for (const t of await visibleEnglish(page)) leaks.push(`${hub}?tab=${tab}: ${t}`);
        }
      }
      if (who === 'admin@biletflow.kz') {
        for (const tab of await page.getByRole('tab').all()) {
          await tab.click();
          for (const t of await visibleEnglish(page)) leaks.push(`/admin tab: ${t}`);
        }
      }
      expect(leaks).toEqual([]);
    });
  }
}

test.afterAll(async ({ browser }) => {
  // Put the demo accounts back to their seeded languages.
  const page = await browser.newPage({ baseURL: WEB_URL });
  for (const [email, locale] of [['aigerim@example.kz', 'kk'], ['organizer@biletflow.kz', 'en'], ['admin@biletflow.kz', 'en']] as const) {
    await signIn(page, email, locale);
    await page.waitForTimeout(300);
    await page.evaluate(() => sessionStorage.clear());
  }
  await page.close();
});
