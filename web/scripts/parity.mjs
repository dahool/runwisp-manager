import { chromium } from 'playwright';
import { mkdir } from 'node:fs/promises';

const managerURL = process.env.MANAGER_URL ?? 'http://127.0.0.1:9478/manager/';
const runwispURL = process.env.RUNWISP_URL ?? 'http://127.0.0.1:9477/';
const browser = await chromium.launch();
const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, colorScheme: 'light' });
await mkdir('parity-report', { recursive: true });

for (const theme of ['light', 'dark']) {
  await context.addInitScript((value) => localStorage.setItem('runwisp:theme', value), theme);
  const manager = await context.newPage();
  const upstream = await context.newPage();
  await Promise.all([manager.goto(managerURL), upstream.goto(runwispURL)]);
  await manager.screenshot({ path: `parity-report/manager-${theme}.png`, fullPage: true });
  await upstream.screenshot({ path: `parity-report/runwisp-${theme}.png`, fullPage: true });
  const [mStyle, uStyle] = await Promise.all([
    manager.evaluate(() => { const body = getComputedStyle(document.body); const header = document.querySelector('header'); return { font: body.fontFamily, headerHeight: header?.getBoundingClientRect().height }; }),
    upstream.evaluate(() => { const body = getComputedStyle(document.body); const header = document.querySelector('header'); return { font: body.fontFamily, headerHeight: header?.getBoundingClientRect().height }; })
  ]);
  if (mStyle.font !== uStyle.font || mStyle.headerHeight !== uStyle.headerHeight) throw new Error(`${theme} chrome styles differ`);
  await manager.close(); await upstream.close();
}
await browser.close();
console.log('Parity captures and computed-style checks passed. Pixel diff regions are in parity-report/.');
