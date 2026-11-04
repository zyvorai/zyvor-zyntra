// Capture real console pages from a running lab for the README frames (docs/ux/shot-*.jpg).
// Nothing is mocked: start a lab, run this, then `node docs/social/readme/build.cjs`.
//
//   # AI infrastructure pack (needs fake sources):
//   ./bin/zyntra fake-sources -addr 127.0.0.1:19700 &
//   ZYNTRA_STATE_DIR=$(mktemp -d) ZYNTRA_ADMIN_PASSWORD=lab ZYNTRA_API_KEY=dev ZYNTRA_NETRA_URL=http://127.0.0.1:19700 \
//     ZYNTRA_GRAVIA_URL=http://127.0.0.1:19700 ZYNTRA_FABRIC_URL=http://127.0.0.1:19700 ZYNTRA_FABRIC_PASSWORD=fake \
//     ZYNTRA_KEEP_URL=http://127.0.0.1:19700 ./bin/zyntra serve -f packs/gpu &
//   PASSWORD=lab node docs/social/readme/capture.cjs gpu http://127.0.0.1:8080
//
//   # Manufacturing pack (Objects, Workflows, Scenarios):
//   ZYNTRA_STATE_DIR=$(mktemp -d) ZYNTRA_ADMIN_PASSWORD=lab ZYNTRA_API_KEY=dev ./bin/zyntra serve -f packs/manufacturing -addr 127.0.0.1:8081 &
//   PASSWORD=lab node docs/social/readme/capture.cjs mfg http://127.0.0.1:8081
//
// The script signs in through POST /api/v1/session and moves between pages by hash route, so it does
// not depend on how the menus open.
//
// Use a fresh ZYNTRA_STATE_DIR per pack so objects and proposals do not mix.
const path = require('node:path');
const fs = require('node:fs');
const root = path.join(__dirname, '..', '..', '..');
const pw = [process.env.PLAYWRIGHT, path.join(root, 'web', 'node_modules', 'playwright')].filter(Boolean).find((p) => fs.existsSync(p));
if (!pw) { console.error('playwright not found; set PLAYWRIGHT=/path/to/node_modules/playwright'); process.exit(1); }
const { chromium } = require(pw);

const [mode, base] = process.argv.slice(2);
const password = process.env.PASSWORD || 'Admin@321';
const out = path.join(__dirname, '_captures');
fs.mkdirSync(out, { recursive: true });

(async () => {
  const browser = await chromium.launch({ headless: true, channel: process.env.CHROME ? undefined : 'chrome', executablePath: process.env.CHROME });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 2, colorScheme: 'light' });
  const signin = await ctx.request.post(base + '/api/v1/session', { data: { username: 'admin', password, remember: false } });
  if (!signin.ok()) { console.error('sign-in failed: ' + signin.status()); process.exit(1); }
  const page = await ctx.newPage();
  const wait = (ms) => page.waitForTimeout(ms);
  const go = async (id) => {
    await page.goto(base + '/#/' + id); await page.reload();
    await page.waitForSelector('nav'); await page.mouse.move(700, 880); await wait(2200);
  };
  const shot = (name, opts = {}) => page.screenshot({ path: path.join(out, name + '.png'), ...opts });
  // The default-password banner is for first-run lab use, not for a product picture.
  const hideBanner = () => page.addStyleTag({ content: '.banner:has(code){display:none !important}' });

  if (mode === 'gpu') {
    await go('overview'); await hideBanner(); await shot('overview');
    await go('plan'); await hideBanner(); await shot('plan');
    await go('simulate'); await hideBanner(); await page.getByRole('button', { name: 'Simulate', exact: true }).click(); await wait(2200); await shot('simulate', { fullPage: true });
    await go('signals'); await hideBanner(); await shot('signals');
    await go('model'); await hideBanner(); await shot('model');
    // Approvals: propose the last (single, non-combined) action on Plan so the inbox has a real proposal.
    await go('plan');
    const propose = page.getByRole('button', { name: 'Propose for approval' });
    await propose.nth((await propose.count()) - 1).click(); await wait(2200);
    await go('approvals'); await hideBanner(); await shot('approvals', { fullPage: true });
  } else if (mode === 'mfg') {
    await go('scenarios');
    const save = async (name, boxes) => {
      await page.getByPlaceholder(/Name, e\.g\./).fill(name);
      for (const b of boxes) await page.getByLabel(b).check();
      await page.getByRole('button', { name: 'Save and run' }).click(); await wait(2500);
      for (const b of boxes) await page.getByLabel(b).uncheck().catch(() => {});
    };
    const gpu = 'Add GPU capacity to the inspection service', alt = 'Route a share of inspection to the alternate site';
    await save('More GPUs', [gpu]); await save('Alternate site', [alt]); await save('Both', [gpu, alt]);
    const boxes = page.locator('table input[type=checkbox]');
    for (let i = 0; i < (await boxes.count()); i++) await boxes.nth(i).check();
    await wait(2000); await hideBanner(); await shot('scenarios', { fullPage: true });
    await go('workflows'); await hideBanner(); await shot('workflows');
    await go('objects'); await hideBanner();
    await page.getByText('InspectionService', { exact: true }).first().click(); await wait(800);
    await page.getByText('Vision QA (plant A)', { exact: true }).first().click(); await wait(2000);
    await page.addStyleTag({ content: 'header,nav{position:static !important}' });
    await page.evaluate(() => window.scrollTo(0, 640)); await wait(500);
    await shot('objects');
  } else { console.error('usage: capture.cjs gpu|mfg <base-url>'); process.exit(2); }
  await browser.close();
})();
