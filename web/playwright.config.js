import { defineConfig } from 'playwright/test';

export default defineConfig({
	testDir: './tests',
	testMatch: '**/*.spec.js',
	fullyParallel: true,
	use: { baseURL: process.env.RWM_E2E_URL ?? 'http://127.0.0.1:4173', screenshot: 'only-on-failure' },
	webServer: process.env.RWM_E2E_URL ? undefined : { command: 'npx vite preview --host 127.0.0.1 --port 4173', url: 'http://127.0.0.1:4173', reuseExistingServer: true }
});
