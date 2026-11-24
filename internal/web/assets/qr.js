// QR codes (ISO/IEC 18004) for the party's link, so others join with their
// phone's camera. Only what that needs: byte mode (UTF-8), error correction
// level M, versions 1 to 6 (up to 106 bytes: the link with a domain name of
// up to about 85 characters), the mask chosen by the standard's penalty
// rules. Larger versions would need the version information, more alignment
// patterns and blocks of two lengths. Pure: encode gives the modules, path an
// SVG path of the dark ones; the party page draws it. Tested by qr_test.js
// against matrices of another encoder (testdata/qr.json).
"use strict";

const QR = (() => {
  // Level M of versions 1 to 6 (index 0 unused): data codewords, error
  // correction codewords per block, number of blocks (ISO/IEC 18004,
  // table 9). The blocks of a version are all of one length.
  const DATA = [0, 16, 28, 44, 64, 86, 108];
  const ECC = [0, 10, 16, 26, 18, 24, 16];
  const BLOCKS = [0, 1, 1, 1, 2, 2, 4];
  const LEVEL_M = 0; // its two bits in the format information

  // ---------- Reed-Solomon over GF(256), polynomial 0x11D ----------
  function mul(x, y) {
    let z = 0;
    for (let i = 7; i >= 0; i--) {
      z = (z << 1) ^ ((z >>> 7) * 0x11d);
      z ^= ((y >>> i) & 1) * x;
    }
    return z;
  }
  // The generator polynomial of the given degree, without its leading 1.
  function divisor(degree) {
    const res = new Array(degree).fill(0);
    res[degree - 1] = 1;
    let root = 1;
    for (let i = 0; i < degree; i++) {
      for (let j = 0; j < degree; j++) {
        res[j] = mul(res[j], root);
        if (j + 1 < degree) res[j] ^= res[j + 1];
      }
      root = mul(root, 2);
    }
    return res;
  }
  function remainder(data, div) {
    const res = new Array(div.length).fill(0);
    for (const b of data) {
      const f = b ^ res.shift();
      res.push(0);
      div.forEach((c, i) => (res[i] ^= mul(c, f)));
    }
    return res;
  }

  // ---------- codewords ----------
  function codewords(bytes, ver) {
    const bits = [];
    const put = (v, n) => { for (let i = n - 1; i >= 0; i--) bits.push((v >>> i) & 1); };
    const cap = DATA[ver] * 8;
    put(4, 4); // byte mode
    put(bytes.length, 8);
    bytes.forEach((b) => put(b, 8));
    put(0, Math.min(4, cap - bits.length)); // terminator
    put(0, (8 - (bits.length % 8)) % 8);
    for (let pad = 0xec; bits.length < cap; pad ^= 0xec ^ 0x11) put(pad, 8);
    const data = [];
    for (let i = 0; i < bits.length; i += 8) data.push(bits.slice(i, i + 8).reduce((a, b) => (a << 1) | b, 0));

    // Split into blocks, add each one's error correction, then interleave.
    const len = DATA[ver] / BLOCKS[ver], div = divisor(ECC[ver]);
    const blocks = [];
    for (let k = 0; k < DATA[ver]; k += len) {
      const d = data.slice(k, k + len);
      blocks.push(d.concat(remainder(d, div)));
    }
    const out = [];
    for (let i = 0; i < blocks[0].length; i++) blocks.forEach((b) => out.push(b[i]));
    return out;
  }

  // ---------- the symbol ----------
  // The 15 bits of the format information for a mask.
  function formatBits(mask) {
    const d = (LEVEL_M << 3) | mask;
    let r = d;
    for (let i = 0; i < 10; i++) r = (r << 1) ^ ((r >>> 9) * 0x537);
    return ((d << 10) | r) ^ 0x5412;
  }

  const MASKS = [
    (x, y) => (x + y) % 2 === 0,
    (x, y) => y % 2 === 0,
    (x) => x % 3 === 0,
    (x, y) => (x + y) % 3 === 0,
    (x, y) => (Math.floor(x / 3) + Math.floor(y / 2)) % 2 === 0,
    (x, y) => ((x * y) % 2) + ((x * y) % 3) === 0,
    (x, y) => (((x * y) % 2) + ((x * y) % 3)) % 2 === 0,
    (x, y) => (((x + y) % 2) + ((x * y) % 3)) % 2 === 0,
  ];

  // The symbol with the given mask: rows of 0 (light) and 1 (dark).
  function build(ver, words, mask) {
    const size = ver * 4 + 17;
    const m = Array.from({ length: size }, () => new Array(size).fill(0));
    const fixed = Array.from({ length: size }, () => new Array(size).fill(false));
    const set = (x, y, dark) => { m[y][x] = dark ? 1 : 0; fixed[y][x] = true; };
    // A square pattern around (cx, cy): rings at the distances in light are
    // light, the others dark.
    const square = (cx, cy, r, light) => {
      for (let dy = -r; dy <= r; dy++) {
        for (let dx = -r; dx <= r; dx++) {
          const x = cx + dx, y = cy + dy;
          if (x >= 0 && x < size && y >= 0 && y < size) set(x, y, !light.includes(Math.max(Math.abs(dx), Math.abs(dy))));
        }
      }
    };

    for (let i = 0; i < size; i++) { set(6, i, i % 2 === 0); set(i, 6, i % 2 === 0); } // timing
    for (const [cx, cy] of [[3, 3], [size - 4, 3], [3, size - 4]]) square(cx, cy, 4, [2, 4]); // finders and separators
    if (ver >= 2) square(size - 7, size - 7, 2, [1]); // the one alignment pattern
    const f = formatBits(mask), bit = (v, i) => (v >>> i) & 1;
    for (let i = 0; i <= 5; i++) set(8, i, bit(f, i));
    set(8, 7, bit(f, 6));
    set(8, 8, bit(f, 7));
    set(7, 8, bit(f, 8));
    for (let i = 9; i < 15; i++) set(14 - i, 8, bit(f, i));
    for (let i = 0; i < 8; i++) set(size - 1 - i, 8, bit(f, i));
    for (let i = 8; i < 15; i++) set(8, size - 15 + i, bit(f, i));
    set(8, size - 8, 1); // the dark module

    // The codewords in two-module columns, zigzag from the bottom right,
    // skipping the vertical timing pattern; the mask flips data modules only.
    let i = 0;
    for (let right = size - 1; right >= 1; right -= 2) {
      if (right === 6) right = 5;
      const up = ((right + 1) & 2) === 0;
      for (let v = 0; v < size; v++) {
        const y = up ? size - 1 - v : v;
        for (let j = 0; j < 2; j++) {
          const x = right - j;
          if (fixed[y][x]) continue;
          const dark = i < words.length * 8 ? bit(words[i >>> 3], 7 - (i & 7)) : 0;
          i++;
          m[y][x] = dark ^ (MASKS[mask](x, y) ? 1 : 0);
        }
      }
    }
    return m;
  }

  // The standard's penalty of a symbol (features that make it hard to scan).
  function penalty(m) {
    const size = m.length;
    let p = 0;
    const lines = [];
    for (let y = 0; y < size; y++) lines.push(m[y], m.map((row) => row[y]));
    for (const line of lines) {
      for (let i = 0; i < size;) { // runs of five or more of a colour
        let j = i;
        while (j < size && line[j] === line[i]) j++;
        if (j - i >= 5) p += j - i - 2;
        i = j;
      }
      const s = line.join(""); // what looks like a finder
      for (let k = s.indexOf("1011101"); k >= 0; k = s.indexOf("1011101", k + 1)) {
        if (s.slice(k + 7, k + 11) === "0000") p += 40;
        if (k >= 4 && s.slice(k - 4, k) === "0000") p += 40;
      }
    }
    for (let y = 0; y + 1 < size; y++) { // 2×2 blocks of a colour
      for (let x = 0; x + 1 < size; x++) {
        const c = m[y][x];
        if (m[y][x + 1] === c && m[y + 1][x] === c && m[y + 1][x + 1] === c) p += 3;
      }
    }
    const dark = m.reduce((a, row) => a + row.reduce((b, c) => b + c, 0), 0);
    return p + Math.floor(Math.abs((dark * 100) / (size * size) - 50) / 5) * 10;
  }

  // The smallest symbol (level M) for text, as rows of 0 and 1. mask picks a
  // mask instead of the one with the least penalty (for tests).
  function encode(text, mask) {
    const bytes = [...new TextEncoder().encode(text)];
    let ver = 1;
    while (DATA[ver] < bytes.length + 2) { // with mode and length
      if (++ver >= DATA.length) throw new Error("QR: text too long");
    }
    const words = codewords(bytes, ver);
    if (mask !== undefined) return build(ver, words, mask);
    let best = null, bestP = Infinity;
    for (let k = 0; k < 8; k++) {
      const m = build(ver, words, k), p = penalty(m);
      if (p < bestP) { best = m; bestP = p; }
    }
    return best;
  }

  // An SVG path of the dark modules, one rectangle per horizontal run, in
  // units of a module.
  function path(m) {
    let d = "";
    m.forEach((row, y) => {
      for (let x = 0; x < row.length;) {
        if (!row[x]) { x++; continue; }
        let e = x;
        while (e < row.length && row[e]) e++;
        d += `M${x} ${y}h${e - x}v1h${x - e}z`;
        x = e;
      }
    });
    return d;
  }

  return { encode, path };
})();

if (typeof module !== "undefined") module.exports = QR;
