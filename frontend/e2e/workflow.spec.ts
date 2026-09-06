import { test, expect } from '@playwright/test';
test('login and authenticated SOC screens load real API data', async ({ page }) => {
  const failures: string[] = [];
  page.on('response', response => {
    if (response.url().includes('/api/v1/') && response.status() >= 400) failures.push(`${response.status()} ${response.url()}`);
  });
  await page.goto('/login');
  await page.locator('input[type=email]').fill(process.env.INTEGRATION_ADMIN_EMAIL ?? 'admin@example.com');
  await page.locator('input[type=password]').fill(process.env.INTEGRATION_ADMIN_PASSWORD ?? 'change-me-now');
  await page.locator('button[type=submit]').click();
  await expect(page).toHaveURL('http://127.0.0.1:3000/');
  for (const route of ['/events', '/alerts', '/cases']) {
    const response = page.waitForResponse(r => r.url().includes(`/api/v1${route}`) && r.status() === 200);
    await page.goto(route);
    await response;
    await expect(page.locator('main')).toBeVisible();
  }
  expect(failures).toEqual([]);
});
