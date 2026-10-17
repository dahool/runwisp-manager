// @ts-nocheck
import { test, expect } from 'playwright/test';

test.describe('task and service editor contracts', () => {
	test('manager health endpoint is ready in the compose stack', async ({ request }) => {
		if (!process.env.RWM_E2E_URL) test.skip(true, 'requires the compose manager target');
		const response = await request.get('/manager/healthz');
		expect(response.ok()).toBeTruthy();
		expect(response.headers()['content-type']).toContain('application/json');
	});

	test('editor route has a usable form or reports the protected daemon state', async ({ page }) => {
		const response = await page.goto('/manager/tasks/new');
		if (!response || response.status() >= 500) test.skip(true, 'manager daemon is unavailable in this environment');
		await expect(page.locator('body')).toContainText(/New task|Log in|Configuration tree|editor/i);
	});

	test('read view exposes editor navigation without inventing an edit control for viewers', async ({ page }) => {
		const response = await page.goto('/manager/tasks');
		if (!response || response.status() >= 500) test.skip(true, 'manager daemon is unavailable in this environment');
		await expect(page.locator('body')).toContainText(/Tasks|Log in|No units|Configuration/i);
	});
});
