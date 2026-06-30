import { chromium } from 'playwright';
import { mkdir } from 'node:fs/promises';
import path from 'node:path';

const baseURL = 'http://localhost:1313';
const outDir = path.resolve('.screenshots/mobile');

const pages = [
  { name: 'home', path: '/' },
  { name: 'about', path: '/about/' },
  { name: 'blog', path: '/blog/' },
  { name: 'blog-post-kotlin', path: '/blog/setting-up-kotlin-graalvm-gradle/' },
  { name: 'blog-post-gradle', path: '/blog/gradle-repositories-settings-vs-build/' },
  { name: 'projects', path: '/projects/' },
  { name: 'resume', path: '/resume/' },
  { name: 'homelab', path: '/homelab/' },
  { name: 'uses', path: '/uses/' },
  { name: 'now', path: '/now/' },
];

await mkdir(outDir, { recursive: true });

const browser = await chromium.launch();
const context = await browser.newContext({
  viewport: { width: 390, height: 844 },
  deviceScaleFactor: 2,
});

for (const theme of ['light', 'dark']) {
  const page = await context.newPage();
  await page.emulateMedia({ colorScheme: theme });

  for (const { name, path: pagePath } of pages) {
    await page.goto(`${baseURL}${pagePath}`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(600);
    await page.screenshot({
      path: path.join(outDir, `${name}-${theme}.png`),
      fullPage: true,
    });
  }

  await page.goto(`${baseURL}/`, { waitUntil: 'networkidle' });
  await page.click('.nav-toggle');
  await page.waitForTimeout(400);
  await page.screenshot({
    path: path.join(outDir, `nav-open-${theme}.png`),
    fullPage: false,
  });

  await page.close();
}

await browser.close();
console.log(`Saved ${pages.length * 2 + 2} screenshots to ${outDir}`);
