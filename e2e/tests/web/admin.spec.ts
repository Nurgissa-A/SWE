import { expect, signIn, test } from '../../helpers';

test('platform admin sees every console tab', async ({ page }) => {
  await signIn(page, 'admin@biletflow.kz', 'en');
  await page.goto('/admin');
  const tabs = ['Overview', 'Events', 'Users', 'Orders', 'Payments', 'Activations', 'Promo activity', 'Reports', 'Settings'];
  for (const name of tabs) {
    await page.getByRole('tab', { name, exact: true }).click();
    await expect(page.getByRole('tab', { name, exact: true })).toHaveAttribute('aria-selected', 'true');
  }
  await page.getByRole('tab', { name: 'Users', exact: true }).click();
  await expect(page.getByText('aigerim@example.kz')).toBeVisible();
});
