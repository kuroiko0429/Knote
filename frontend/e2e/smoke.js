// 使い方: wails dev 起動後に node frontend/e2e/smoke.js
// chromium は ~/.cache/ms-playwright/chromium-*/chrome-linux64/chrome を使う
const { chromium } = require('playwright-core')
const fs = require('fs')
const os = require('os')

function findChromium() {
  const dir = os.homedir() + '/.cache/ms-playwright'
  const hit = fs.readdirSync(dir).filter((d) => /^chromium-\d+$/.test(d)).sort().pop()
  if (!hit) throw new Error('playwright chromium not found; npx playwright install chromium')
  return `${dir}/${hit}/chrome-linux64/chrome`
}

const fail = (msg) => { console.error('FAIL:', msg); process.exit(1) }

;(async () => {
  const browser = await chromium.launch({ executablePath: findChromium(), headless: true })
  const page = await browser.newPage({ viewport: { width: 1400, height: 900 } })
  const errors = []
  page.on('pageerror', (e) => errors.push(String(e)))

  await page.goto('http://localhost:34115', { waitUntil: 'networkidle' })
  await page.waitForTimeout(1500)

  // 1. 通常ノート: プレビューが描画される
  await page.locator('text=markdown-cheat-cheet').first().click()
  await page.waitForTimeout(1500)
  const htmlLen = await page.evaluate(() => document.querySelector('.preview')?.innerHTML.length ?? 0)
  if (htmlLen < 100) fail('normal preview empty')

  // 2. Marpノート: スライドとサムネイルが出る
  await page.locator('text=無題2').first().click()
  await page.waitForTimeout(4000)
  const marp = await page.evaluate(() => ({
    slides: document.querySelectorAll('.marp-slide-box').length,
    thumbs: document.querySelectorAll('.marp-thumb').length,
  }))
  if (marp.slides < 1 || marp.thumbs !== marp.slides) fail('marp render broken: ' + JSON.stringify(marp))

  // 3. ページエラーゼロ
  if (errors.length) fail('pageerrors: ' + JSON.stringify(errors))
  console.log('SMOKE PASS')
  await browser.close()
})().catch((e) => fail(e))
