import { expect, signIn, test } from '../../helpers';

test.describe('Organizer', () => {
  test('workspace overview and every event hub tab', async ({ page }) => {
    await signIn(page, 'organizer@biletflow.kz', 'en');
    await page.goto('/dashboard');
    await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible();
    await expect(page.getByText('Astana Jazz Night 2026')).toBeVisible();

    await page.getByRole('link', { name: /Astana Jazz Night 2026/ }).click();
    await expect(page).toHaveURL(/\/dashboard\/events\/[0-9a-f-]+/);
    const hub = page.url().split('?')[0];
    const expectations: Record<string, RegExp> = {
      overview: /Tickets sold/, tickets: /Standard/, paid: /Paid sales/, orders: /BF-\d+/,
      promo: /NU25/, staff: /staff@biletflow\.kz/, history: /Event published/, details: /Save changes/,
    };
    for (const [tab, text] of Object.entries(expectations)) {
      await page.goto(`${hub}?tab=${tab}`);
      await expect(page.getByText(text).first(), `tab ${tab}`).toBeVisible();
    }
  });
});
