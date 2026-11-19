// Writes Lastab's SVG marks from one geometry: a tab on its tab strip,
// marked with the dot an editor shows on an unsaved tab. In Lastab the tab
// never needs saving and is never lost. See README.md for the construction.
//
// The wordmark outlines are read back from the existing horizontal and
// stacked files, so this is safe to run again. scripts/genicon draws the
// PNG icons from the same numbers; change both together.
//
//	node design/brand/gen.mjs
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));

// The symbol on its 256 grid. The content box is 32..224 wide and 64..192
// tall, centred on (128, 128).
const S = {
  L: 32, R: 224, // the baseline (tab strip), round ends
  yb: 168, bh: 24, // its top and thickness
  x0: 64, x1: 192, yt: 64, // the tab
  rt: 36, // the tab's top corners
  rf: 20, // the fillets where the tab meets the baseline
  title: { x: 82, y: 107, w: 56, h: 18 }, // the title, a pill
  dot: { cx: 166, cy: 116, r: 14 }, // the unsaved dot
  dotSmall: { cx: 128, cy: 116, r: 24 }, // the dot alone, below 24 px
};

const C = {
  ink: "#1C1C1A", white: "#FFFFFF", blue: "#2F5BD3", blueLift: "#5B85F2",
  greyOnInk: "#6A6A63", greyOnWhite: "#C9C9C2", line: "#E3E3DD",
  orange: "#C2410C", inactive: "#34342F", inactiveDev: "#D2581F",
};

const n = (v) => +v.toFixed(2);
const tabShape = ({ L, R, yb, bh, x0, x1, yt, rt, rf }) => {
  const c = bh / 2;
  return `M${L + c} ${yb} H${x0 - rf} A${rf} ${rf} 0 0 0 ${x0} ${yb - rf} V${yt + rt} ` +
    `A${rt} ${rt} 0 0 1 ${x0 + rt} ${yt} H${x1 - rt} A${rt} ${rt} 0 0 1 ${x1} ${yt + rt} ` +
    `V${yb - rf} A${rf} ${rf} 0 0 0 ${x1 + rf} ${yb} H${R - c} A${c} ${c} 0 0 1 ${R - c} ${yb + bh} ` +
    `H${L + c} A${c} ${c} 0 0 1 ${L + c} ${yb} Z`;
};
const pill = ({ x, y, w, h }) => {
  const r = h / 2;
  return `M${n(x + r)} ${y} H${n(x + w - r)} A${r} ${r} 0 0 1 ${n(x + w - r)} ${y + h} H${n(x + r)} A${r} ${r} 0 0 1 ${n(x + r)} ${y} Z`;
};
const circle = ({ cx, cy, r }) => `M${cx - r} ${cy} A${r} ${r} 0 1 0 ${cx + r} ${cy} A${r} ${r} 0 1 0 ${cx - r} ${cy} Z`;

// The symbol in colours, or in one colour with the title and dot cut out.
function symbol({ tab, title, dot, mono, small }) {
  const d = small ? S.dotSmall : S.dot;
  if (mono)
    return `<g id="symbol"><path fill="${mono}" fill-rule="evenodd" d="${tabShape(S)} ${small ? "" : pill(S.title) + " "}${circle(d)}"/></g>`;
  return `<g id="symbol"><path fill="${tab}" d="${tabShape(S)}"/>` +
    (small ? "" : `<path fill="${title}" d="${pill(S.title)}"/>`) +
    `<path fill="${dot}" d="${circle(d)}"/></g>`;
}

const schemes = {
  color: { tab: C.ink, title: C.greyOnInk, dot: C.blueLift, word: C.ink },
  reversed: { tab: C.white, title: C.greyOnWhite, dot: C.blue, word: C.white },
  black: { mono: C.ink, word: C.ink },
  white: { mono: C.white, word: C.white },
};

const svg = (viewBox, title, body) => {
  const [, , w, h] = viewBox.split(" ");
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${viewBox}" width="${w}" height="${h}" role="img"><title>${title}</title>${body}</svg>\n`;
};
const out = (name, s) => writeFileSync(join(here, name), s);

// Wordmark outlines and layout from the existing lockups.
const lockup = (name) => {
  const s = readFileSync(join(here, `lastab-${name}-color.svg`), "utf8");
  return {
    viewBox: s.match(/viewBox="([^"]+)"/)[1],
    word: s.match(/<path id="wordmark" fill="[^"]+" d="([^"]+)"/)[1],
  };
};
const horizontal = lockup("horizontal");
const stacked = lockup("stacked");

for (const [name, sc] of Object.entries(schemes)) {
  out(`lastab-symbol-${name}.svg`, svg("24 24 208 208", "Lastab symbol", symbol(sc)));
  out(`lastab-symbol-small-${name}.svg`, svg("24 24 208 208", "Lastab symbol (small sizes)", symbol({ ...sc, small: true })));
  for (const [kind, l] of [["horizontal", horizontal], ["stacked", stacked]])
    out(`lastab-${kind}-${name}.svg`, svg(l.viewBox, "Lastab", symbol(sc) + `<path id="wordmark" fill="${sc.word}" d="${l.word}"/>`));
}

// The app icon on the macOS 1024 grid (tile 100..924, radius 185): the
// tile's top is the tab strip, one tab is open and runs into the page below,
// whose last line holds the cursor. An inactive tab sits behind on the right.
export const A = {
  tile: { x0: 100, y0: 100, x1: 924, y1: 924, r: 185 },
  yj: 476, // where the strip ends and the page begins
  tab: { x0: 212, x1: 632, yt: 252, rt: 96, rf: 56 },
  inactive: { x0: 716, yt: 316, r: 60 },
  title: { x: 292, y: 342, w: 170, h: 52 },
  dot: { cx: 548, cy: 368, r: 50 },
  lines: [{ x: 212, y: 624, w: 300, h: 52 }, { x: 212, y: 728, w: 190, h: 52 }],
  cursor: { x: 430, y: 700, w: 36, h: 108 },
};

function appIcon(strip, inactive, title) {
  const { tile: t, yj, tab: b } = A;
  const page = `M${t.x0} ${yj} H${b.x0 - b.rf} A${b.rf} ${b.rf} 0 0 0 ${b.x0} ${yj - b.rf} V${b.yt + b.rt} ` +
    `A${b.rt} ${b.rt} 0 0 1 ${b.x0 + b.rt} ${b.yt} H${b.x1 - b.rt} A${b.rt} ${b.rt} 0 0 1 ${b.x1} ${b.yt + b.rt} ` +
    `V${yj - b.rf} A${b.rf} ${b.rf} 0 0 0 ${b.x1 + b.rf} ${yj} H${t.x1} V${t.y1} H${t.x0} Z`;
  const i = A.inactive;
  const back = `M${i.x0} ${yj} V${i.yt + i.r} A${i.r} ${i.r} 0 0 1 ${i.x0 + i.r} ${i.yt} H${t.x1} V${yj} Z`;
  const cur = A.cursor;
  return svg("0 0 1024 1024", title,
    `<defs><clipPath id="tile"><rect x="${t.x0}" y="${t.y0}" width="${t.x1 - t.x0}" height="${t.y1 - t.y0}" rx="${t.r}"/></clipPath></defs>` +
    `<g clip-path="url(#tile)"><rect x="0" y="0" width="1024" height="1024" fill="${strip}"/>` +
    `<path fill="${inactive}" d="${back}"/><path fill="${C.white}" d="${page}"/>` +
    `<path fill="${C.greyOnWhite}" d="${pill(A.title)}"/><path fill="${C.blue}" d="${circle(A.dot)}"/>` +
    A.lines.map((l) => `<path fill="${C.line}" d="${pill(l)}"/>`).join("") +
    `<rect x="${cur.x}" y="${cur.y}" width="${cur.w}" height="${cur.h}" rx="${cur.w / 2}" fill="${C.blue}"/></g>`);
}
out("lastab-app-icon.svg", appIcon(C.ink, C.inactive, "Lastab app icon"));
out("lastab-app-icon-dev.svg", appIcon(C.orange, C.inactiveDev, "Lastab app icon (development)"));

// The marks the app draws (internal/ui/brand, embedded). The tab and the
// wordmark are currentColor, so they take the text color of each
// appearance; the title is cut out of the tab and drawn faint over what is
// behind. The dot is the brand blue, lifted on the dark tab of a light
// theme as the color scheme does, darker on the light tab of a dark one.
const ui = join(here, "../../internal/ui/brand");
const uiSymbol = `<style>.dot{fill:${C.blueLift}}@media (prefers-color-scheme: dark){.dot{fill:${C.blue}}}</style>` +
  `<g id="symbol"><path fill="currentColor" fill-rule="evenodd" d="${tabShape(S)} ${pill(S.title)}"/>` +
  `<path fill="currentColor" fill-opacity=".4" d="${pill(S.title)}"/>` +
  `<path class="dot" d="${circle(S.dot)}"/></g>`;
const uiSvg = (viewBox, body) => {
  const [, , w, h] = viewBox.split(" ");
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${viewBox}" width="${w}" height="${h}">${body}</svg>\n`;
};
writeFileSync(join(ui, "symbol.svg"), uiSvg("24 24 208 208", uiSymbol));
writeFileSync(join(ui, "horizontal.svg"), uiSvg(horizontal.viewBox, uiSymbol + `<path fill="currentColor" d="${horizontal.word}"/>`));
console.log("wrote the Lastab marks");
