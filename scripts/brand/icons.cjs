// Renders the Aero app icon (the Streamline A) at each size, with fewer and
// bolder strands when small, and packs them into a multi-resolution .ico plus
// a 1024px PNG and an SVG master.
//
// Usage: node scripts/brand/icons.cjs <outDir>     (needs puppeteer-core + local Chrome)
const fs = require('fs');
const path = require('path');
const puppeteer = require('puppeteer-core');
const { mark } = require('./mark.cjs');
const outDir = process.argv[2] || path.join(__dirname, '..', '..', 'assets');

const detail = (px) =>
  px >= 96 ? { n: 3, w: 2.6, o: [1, 0.62, 0.36] } : px >= 40 ? { n: 3, w: 3.2, o: [1, 0.6, 0.34] } : px >= 24 ? { n: 2, w: 4, o: [1, 0.5] } : { n: 1, w: 5.4, o: [1] };

const svg = (px) => {
  const d = detail(px);
  const m = mark(d.n);
  const strands = m.lines.map((l, k) => `<path d="${l}" fill="none" stroke="#E8ECF0" stroke-opacity="${d.o[k]}" stroke-width="${d.w}" stroke-linecap="round" stroke-linejoin="round"/>`).join('');
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="100%" height="100%"><defs><clipPath id="c"><rect x="2" y="2" width="60" height="60" rx="14"/></clipPath><linearGradient id="bg" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#151920"/><stop offset="1" stop-color="#0B0D10"/></linearGradient></defs><rect x="2" y="2" width="60" height="60" rx="14" fill="url(#bg)"/><g clip-path="url(#c)"><g transform="translate(32 33) scale(0.74) translate(-32 -32)">${strands}<path d="${m.crossbar}" fill="none" stroke="#FF5B1F" stroke-width="${d.w}" stroke-linecap="round"/></g></g><rect x="2.5" y="2.5" width="59" height="59" rx="13.5" fill="none" stroke="#ffffff" stroke-opacity=".07"/></svg>`;
};
const sizes = [16, 20, 24, 32, 40, 48, 64, 128, 256, 1024];
(async () => {
  const b = await puppeteer.launch({ executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: 'new' });
  const p = await b.newPage();
  const pngs = {};
  for (const s of sizes) {
    await p.setViewport({ width: s, height: s, deviceScaleFactor: 1 });
    await p.setContent(`<html><body style="margin:0;background:transparent">${svg(s)}</body></html>`);
    pngs[s] = await p.screenshot({ omitBackground: true, clip: { x: 0, y: 0, width: s, height: s } });
  }
  await b.close();
  fs.writeFileSync(`${outDir}/appicon.png`, pngs[1024]);
  fs.writeFileSync(`${outDir}/appicon.svg`, svg(1024).replace(' width="100%" height="100%"', ''));
  const ico = sizes.filter((s) => s <= 256);
  const header = Buffer.alloc(6 + 16 * ico.length);
  header.writeUInt16LE(0, 0); header.writeUInt16LE(1, 2); header.writeUInt16LE(ico.length, 4);
  let offset = header.length;
  ico.forEach((s, i) => {
    const e = 6 + i * 16, data = pngs[s];
    header.writeUInt8(s === 256 ? 0 : s, e); header.writeUInt8(s === 256 ? 0 : s, e + 1);
    header.writeUInt16LE(1, e + 4); header.writeUInt16LE(32, e + 6);
    header.writeUInt32LE(data.length, e + 8); header.writeUInt32LE(offset, e + 12);
    offset += data.length;
  });
  fs.writeFileSync(`${outDir}/icon.ico`, Buffer.concat([header, ...ico.map((s) => pngs[s])]));
  // Contact sheet for visual review
  fs.writeFileSync(path.join(require('os').tmpdir(), 'aero-icon-sheet.html'), `<html><body style="margin:0;padding:24px;background:#2a2d33;display:flex;gap:24px;align-items:end">${ico.map((s) => `<img src="data:image/png;base64,${pngs[s].toString('base64')}" width="${s}" height="${s}">`).join('')}<div style="background:#f3f3f3;padding:16px;display:flex;gap:16px;align-items:end">${[16, 24, 32, 48].map((s) => `<img src="data:image/png;base64,${pngs[s].toString('base64')}" width="${s}" height="${s}">`).join('')}</div></body></html>`);
  console.log('ok', offset, 'bytes ico');
})();
