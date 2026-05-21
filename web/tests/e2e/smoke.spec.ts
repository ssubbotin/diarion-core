import { test, expect } from '@playwright/test';

test.describe('Public read smoke test', () => {
	test('global feed loads and shows the site header', async ({ page }) => {
		await page.goto('/');
		await expect(page.getByRole('link', { name: 'Diarion', exact: true })).toBeVisible();
		await expect(page).toHaveTitle(/Diarion/);
	});

	test('search page renders the form even without a query', async ({ page }) => {
		await page.goto('/search');
		await expect(page.getByRole('searchbox', { name: 'Search query' })).toBeVisible();
	});

	test('about page renders', async ({ page }) => {
		await page.goto('/about');
		await expect(page.getByRole('heading', { name: 'About Diarion' })).toBeVisible();
	});

	test('skip-to-content link is keyboard-reachable', async ({ page }) => {
		await page.goto('/');
		await page.keyboard.press('Tab');
		await expect(page.getByRole('link', { name: 'Skip to content' })).toBeFocused();
	});
});
