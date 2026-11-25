import { expect, signIn, test, w } from '../../helpers';

test.describe('Attendee', () => {
  test('browses events without signing in', async ({ page }) => {
    await page.context().addCookies([{ name: 'bf_locale', value: 'en', url: process.env.WEB_URL ?? 'http://localhost:3000' }]);
    await page.goto('/events');
    await expect(page.getByText('Astana Jazz Night 2026')).toBeVisible();
    await page.locator('article', { hasText: 'Astana Jazz Night 2026' }).getByRole('link').click();
    await expect(page.getByRole('button', { name: 'More Standard' })).toBeVisible();
  });

  test('buys a paid ticket in Kazakh: declined card, then success with QR', async ({ page }) => {
    const kk = (en: string, p?: Record<string, string | number>) => w('kk', en, p);
    await signIn(page, 'aigerim@example.kz', 'kk');
    await page.goto('/events');
    await page.locator('article', { hasText: 'Astana Jazz Night 2026' }).getByRole('link').click();
    await expect(page.getByRole('heading', { name: 'Astana Jazz Night 2026' })).toBeVisible();

    await page.getByRole('button', { name: kk('More {name}', { name: 'Standard' }) }).click();
    await page.getByRole('button', { name: kk('Continue to payment') }).click();
    await expect(page.getByText(kk('Payment'), { exact: true })).toBeVisible();
    await expect(page.getByText(new RegExp(kk('Tickets held for {time}', { time: '\\d+:\\d\\d' })))).toBeVisible();

    const card = page.getByLabel(kk('Card number'));
    await card.fill('4000 0000 0000 0002');
    await page.getByRole('button', { name: new RegExp(kk('Pay {amount}', { amount: '' })) }).click();
    await expect(page).not.toHaveURL(/\/orders\//);
    await expect(page.getByRole('alert').filter({ hasText: kk('Card declined (simulated)') })).toBeVisible();

    // A declined payment releases the hold; trying again starts a new payment.
    await page.getByRole('button', { name: kk('Continue to payment') }).click();
    await card.fill('4242 4242 4242 4242');
    await page.getByRole('button', { name: new RegExp(kk('Pay {amount}', { amount: '' })) }).click();
    await expect(page).toHaveURL(/\/orders\/[0-9a-f-]+/);
    await expect(page.getByText(/BF-\d+/).first()).toBeVisible();
    await expect(page.getByRole('img', { name: new RegExp(kk('Admission QR code for ticket {code}', { code: '' })) }).first()).toBeVisible();
    // Dates use our own Kazakh month names rather than the browser's ICU data.
    await expect(page.getByText(/\d{4} ж\. \d{1,2} (қаңтар|ақпан|наурыз|сәуір|мамыр|маусым|шілде|тамыз|қыркүйек|қазан|қараша|желтоқсан)/).first()).toBeVisible();
  });
});
