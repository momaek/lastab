// Writes the two brand boards as SVG from the marks gen.mjs wrote, for
// rsvg-convert to render to PNG; the SVGs are not kept:
//
//	node design/brand/gen.mjs && node design/brand/boards.mjs
//	rsvg-convert design/brand/board-final.svg -o design/brand/board-final.png
//	rsvg-convert design/brand/board-in-use.svg -o design/brand/board-in-use.png
//	rm design/brand/board-*.svg
//
// Text uses Inter when installed, else the system sans.
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const read = (f) => readFileSync(join(here, f), "utf8");

// Places a mark file at (x, y), h tall (or w wide), keeping its viewBox.
function place(file, x, y, { h, w }) {
  const s = read(file);
  const [vx, vy, vw, vh] = s.match(/viewBox="([^"]+)"/)[1].split(" ").map(Number);
  const height = h ?? (w * vh) / vw, width = w ?? (h * vw) / vh;
  const inner = s.replace(/^<svg[^>]*>/, "").replace(/<\/svg>\s*$/, "").replace(/<title>[^<]*<\/title>/, "")
    .replaceAll('id="tile"', `id="tile${uid}"`).replaceAll("url(#tile)", `url(#tile${uid++})`);
  return `<svg x="${x}" y="${y}" width="${width}" height="${height}" viewBox="${vx} ${vy} ${vw} ${vh}">${inner}</svg>`;
}
let uid = 0;

const font = `font-family="Inter, -apple-system, Helvetica, sans-serif"`;
const text = (x, y, s, { size = 14, fill = "#1C1C1A", weight = 400, anchor = "start", spacing = 0, mono = false } = {}) =>
  `<text x="${x}" y="${y}" font-size="${size}" font-weight="${weight}" fill="${fill}" text-anchor="${anchor}" letter-spacing="${spacing}" ${mono ? 'font-family="JetBrains Mono, Menlo, monospace"' : font}>${s}</text>`;
const board = (title, page, body) =>
  `<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="800" viewBox="0 0 1280 800">
<rect width="1280" height="800" fill="#FFFFFF"/>
${text(64, 69, title, { size: 12, fill: "#666", spacing: 1.5 })}
${text(64, 122, "Unsaved tab", { size: 30, weight: 700 })}
${body}
${text(64, 771, "Lastab — final identity", { size: 12, fill: "#555" })}
${text(1216, 771, page, { size: 12, fill: "#555", anchor: "end" })}
</svg>\n`;

// board-final: the idea, the lockup, sizes, reversed.
{
  let b = text(64, 165, "The dot an editor shows on an unsaved tab. In Lastab it never needs saving, and is never lost.", { size: 19 });
  b += `<rect x="64" y="190" width="629" height="420" rx="16" fill="#F4F4F1"/>`;
  b += place("lastab-horizontal-color.svg", 148, 340, { h: 120 });
  const bullets = [
    "The tab is the product: scratch tabs opened with ⌘N",
    "The dot is “unsaved, but kept”: Lastab remembers it",
    "Below 24 px the title drops and the dot grows",
  ];
  bullets.forEach((s, i) => (b += `<circle cx="740" cy="${261 + i * 28}" r="2.5" fill="#1C1C1A"/>` + text(751, 266 + i * 28, s, { size: 14.5 })));
  const sizes = [[16, "small"], [24, "small"], [32], [64]];
  let x = 733;
  for (const [s, small] of sizes) {
    b += place(small ? "lastab-symbol-small-color.svg" : "lastab-symbol-color.svg", x, 420 - s, { h: s });
    b += text(x + s / 2, 448, `${s}px`, { size: 11, fill: "#555", anchor: "middle" });
    x += s + 30;
  }
  b += place("lastab-app-icon.svg", 1010, 336, { h: 104 });
  b += `<rect x="733" y="472" width="200" height="80" rx="10" fill="#1C1C1A"/>`;
  b += place("lastab-horizontal-reversed.svg", 753, 497, { h: 30 });
  b += place("lastab-app-icon-dev.svg", 1010, 460, { h: 104 });
  b += text(1062, 578, "dev", { size: 11, fill: "#555", anchor: "middle" });
  writeFileSync(join(here, "board-final.svg"), board("FINAL DESIGN", "3", b));
}

// board-in-use: the marks where they will appear.
{
  const cell = (i, label, bg = "#E9E9E6") => {
    const x = 64 + (i % 3) * 390, y = 143 + Math.floor(i / 3) * 288;
    return { x, y, svg: `<rect x="${x}" y="${y}" width="372" height="270" rx="16" fill="${bg}"/>` +
      `<rect x="${x + 12}" y="${y + 239}" width="${label.length * 6.2 + 22}" height="20" rx="10" fill="#FFFFFF" opacity=".9"/>` +
      text(x + 22, y + 253, label, { size: 11, fill: "#333" }) };
  };
  let b = "";
  // README on GitHub, dark.
  let c = cell(0, "GitHub README");
  b += c.svg + `<rect x="${c.x + 6}" y="${c.y + 53}" width="360" height="164" rx="6" fill="#0D1117"/>`;
  b += place("lastab-horizontal-reversed.svg", c.x + 26, c.y + 74, { h: 26 });
  b += `<rect x="${c.x + 26}" y="${c.y + 112}" width="320" height="1" fill="#30363D"/>`;
  b += text(c.x + 26, c.y + 134, "Back to the last tab", { size: 11, fill: "#E6EDF3" });
  b += [["#238636", 54], ["#1F6FEB", 54], ["#6E7681", 54]].map(([f, w], i) => `<rect x="${c.x + 26 + i * 60}" y="${c.y + 146}" width="${w}" height="12" rx="2" fill="${f}"/>`).join("");
  b += [320, 224, 272].map((w, i) => `<rect x="${c.x + 26}" y="${c.y + 166 + i * 12}" width="${w}" height="5" rx="2.5" fill="#30363D"/>`).join("");
  // Dock and Launchpad.
  c = cell(1, "App icon");
  b += c.svg + `<rect x="${c.x + 42}" y="${c.y + 16}" width="150" height="224" rx="22" fill="#11161C"/><rect x="${c.x + 51}" y="${c.y + 25}" width="132" height="206" rx="15" fill="#D5D9E0"/>`;
  for (let r = 0; r < 4; r++) for (let k = 0; k < 3; k++) {
    const gx = c.x + 61 + k * 41, gy = c.y + 41 + r * 41;
    b += r === 1 && k === 1 ? place("lastab-app-icon.svg", gx - 4, gy - 4, { h: 40 }) : `<rect x="${gx}" y="${gy}" width="32" height="32" rx="8" fill="#BFC4CC"/>`;
  }
  b += place("lastab-app-icon.svg", c.x + 214, c.y + 58, { h: 128 });
  b += text(c.x + 278, c.y + 198, "Lastab", { size: 12, weight: 600, anchor: "middle" });
  // Website.
  c = cell(2, "Website");
  b += c.svg + `<rect x="${c.x + 16}" y="${c.y + 30}" width="340" height="210" rx="8" fill="#FFFFFF"/><rect x="${c.x + 16}" y="${c.y + 30}" width="340" height="22" rx="8" fill="#ECEDEF"/><rect x="${c.x + 16}" y="${c.y + 44}" width="340" height="8" fill="#ECEDEF"/>`;
  b += [0, 1, 2].map((i) => `<circle cx="${c.x + 30 + i * 12}" cy="${c.y + 41}" r="3.5" fill="#C9CCD1"/>`).join("");
  b += place("lastab-horizontal-color.svg", c.x + 32, c.y + 66, { h: 16 });
  b += ["Docs", "Pricing", "GitHub"].map((s, i) => text(c.x + 228 + i * 44, c.y + 77, s, { size: 9.5, fill: "#444" })).join("");
  b += `<rect x="${c.x + 16}" y="${c.y + 94}" width="340" height="1" fill="#E5E7EB"/>`;
  b += text(c.x + 32, c.y + 130, "Back to the last tab", { size: 17, weight: 700 });
  b += `<rect x="${c.x + 32}" y="${c.y + 142}" width="246" height="6" rx="3" fill="#EDEDED"/><rect x="${c.x + 32}" y="${c.y + 154}" width="185" height="6" rx="3" fill="#EDEDED"/>`;
  // Menu bar: the tray icon, light and dark.
  c = cell(3, "Menu bar", "#D3D6DB");
  b += c.svg;
  for (const [i, bar, ink] of [[0, "#ECECEC", "#1C1C1C"], [1, "#2B2B2B", "#EDEDED"]]) {
    const y = c.y + 60 + i * 74;
    b += `<rect x="${c.x + 6}" y="${y}" width="360" height="40" rx="6" fill="${bar}"/>`;
    b += place(`lastab-symbol-${i ? "white" : "black"}.svg`, c.x + 196, y + 9, { h: 22 }).replace(/#FFFFFF|#1C1C1A/g, ink);
    b += `<circle cx="${c.x + 248}" cy="${y + 20}" r="7" fill="none" stroke="${ink}" stroke-width="1.6"/>`;
    b += `<rect x="${c.x + 270}" y="${y + 14}" width="28" height="12" rx="3" fill="none" stroke="${ink}" stroke-width="1.6"/>`;
    b += text(c.x + 316, y + 25, "14:32", { size: 12, fill: ink, weight: 500 });
  }
  // Terminal: the agent CLI.
  c = cell(4, "Terminal", "#CFD2D6");
  b += c.svg + `<rect x="${c.x + 6}" y="${c.y + 86}" width="360" height="98" rx="6" fill="#1E1E1E"/><rect x="${c.x + 6}" y="${c.y + 86}" width="360" height="20" rx="6" fill="#2D2D2D"/><rect x="${c.x + 6}" y="${c.y + 98}" width="360" height="8" fill="#2D2D2D"/>`;
  b += [0, 1, 2].map((i) => `<circle cx="${c.x + 20 + i * 12}" cy="${c.y + 96}" r="3.5" fill="#5A5A5A"/>`).join("");
  b += place("lastab-symbol-reversed.svg", c.x + 18, c.y + 118, { h: 50 });
  b += text(c.x + 84, c.y + 132, "$ lastab search \"curl\"", { size: 11, fill: "#E6E6E6", mono: true });
  b += text(c.x + 84, c.y + 150, "{ \"snippets\": [ … 3 ] }", { size: 11, fill: "#7EC07F", mono: true });
  b += text(c.x + 84, c.y + 168, "$ lastab read 01JD2F9C", { size: 11, fill: "#E6E6E6", mono: true });
  // Social profile.
  c = cell(5, "Social profile");
  b += c.svg + `<clipPath id="card"><rect x="${c.x + 36}" y="${c.y + 50}" width="300" height="168" rx="12"/></clipPath>`;
  b += `<g clip-path="url(#card)"><rect x="${c.x + 36}" y="${c.y + 50}" width="300" height="168" fill="#FFFFFF"/><rect x="${c.x + 36}" y="${c.y + 50}" width="300" height="70" fill="#1C1C1A"/></g>`;
  b += `<circle cx="${c.x + 92}" cy="${c.y + 120}" r="38" fill="#FFFFFF"/>`;
  b += place("lastab-symbol-color.svg", c.x + 62, c.y + 90, { h: 60 });
  b += text(c.x + 56, c.y + 182, "Lastab", { size: 14, weight: 700 });
  b += text(c.x + 56, c.y + 200, "@lastab · Back to the last tab", { size: 11, fill: "#444" });
  writeFileSync(join(here, "board-in-use.svg"), board("FINAL DESIGN · IN USE", "4", b));
}
console.log("wrote the boards");
