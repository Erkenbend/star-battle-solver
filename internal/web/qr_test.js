// Tests for assets/qr.js. Run by TestJSQR (web_test.go): node qr_test.js
//
// testdata/qr.json holds symbols of another encoder (python-qrcode, byte
// mode, level M, the given mask), of versions 1 to 17: with one and several
// blocks, blocks of two lengths, the version information, the 16-bit length
// and UTF-8. When it was made, a scanner (zxing) also read the party's link
// back from screenshots of the lobby, phone and desktop, light and dark.
"use strict";
const assert = require("node:assert/strict");
const QR = require("./assets/qr.js");
const cases = require("./testdata/qr.json");

const rows = (m) => m.map((r) => r.join(""));

for (const c of cases) {
  const name = `${JSON.stringify(c.text.slice(0, 30))} (mask ${c.mask})`;
  assert.deepEqual(rows(QR.encode(c.text, c.mask)), c.rows, name);

  // Without a mask, the same symbol with one of the eight.
  const auto = rows(QR.encode(c.text));
  assert.ok([0, 1, 2, 3, 4, 5, 6, 7].some((k) => auto.join() === rows(QR.encode(c.text, k)).join()), name + ": automatic mask");

  // The path covers the dark modules and nothing else.
  const n = c.rows.length;
  const drawn = Array.from({ length: n }, () => new Array(n).fill("0"));
  for (const [, x, y, w] of QR.path(QR.encode(c.text, c.mask)).matchAll(/M(\d+) (\d+)h(\d+)v1h-\d+z/g)) {
    for (let i = 0; i < +w; i++) drawn[+y][+x + i] = "1";
  }
  assert.deepEqual(drawn.map((r) => r.join("")), c.rows, name + ": path");
}

assert.throws(() => QR.encode("x".repeat(2332)), /too long/); // 2331 bytes fit in version 40
assert.equal(QR.encode("x".repeat(2331)).length, 177);

console.log(`qr: ${cases.length} symbols`);
