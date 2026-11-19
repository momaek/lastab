# Lastab 标志

![Lastab 标志](board-final.png)

## 想法

左边是一个文本光标，右边一个箭头回到光标处：回到上一个 tab。

- **光标**代表产品本身：纯文本，打开就能写。
- **箭头**代表「⌘`」：把刚编辑过的那条找回来。

真实场景里的效果见 [board-in-use.png](board-in-use.png)。

## 文件

全部 SVG 都是轮廓化的路径，没有 `<text>`，不依赖字体。

| 文件 | 用途 |
| --- | --- |
| `lastab-horizontal-{color,reversed,black,white}.svg` | 主标志（横版），README、网页、发布说明 |
| `lastab-stacked-{color,reversed,black,white}.svg` | 竖版，空间接近正方形时用 |
| `lastab-symbol-{color,reversed,black,white}.svg` | 只用图形，头像、小尺寸 |
| `lastab-wordmark-{color,reversed}.svg` | 只用字标，图形已在附近出现时用 |
| `lastab-app-icon.svg` / `lastab-app-icon-dev.svg` | 应用图标母版（1024，macOS 图标网格） |

`color` 用在浅色背景上，`reversed` 用在深色背景上，`black` / `white` 用于只能单色的场合。

应用和托盘图标的 PNG 由 `go run ./scripts/genicon` 生成，几何与这里的 SVG 相同：

| 输出 | 尺寸 | 说明 |
| --- | --- | --- |
| `resources/icon.png` | 1024 | 正式版：墨色底、白箭头、蓝光标 |
| `resources/icon-dev.png` | 1024 | 开发版：橙色底、白箭头、墨色光标 |
| `internal/shell/assets/tray-template.png` | 36 | macOS 菜单栏模板图（18 pt @2x），黑色，由系统着色 |
| `internal/shell/assets/tray-win.png` | 32 | Windows 通知区域，墨色底 |

## 构造

- 256 网格，内容框 176 × 176（40 到 216），四条笔画都是宽 32 的胶囊形。
- **光标**：(56, 56) 到 (56, 200)。
- **箭头**：箭杆 (104, 128) 到 (192, 128)，箭头两翼从 (104, 128) 到 (152, 80) 和 (152, 176)，严格 45°。
- 光标比箭头高：176 对 128。光标和箭头之间留 24 的空隙。

改动图形时，`scripts/genicon/main.go` 里的常量和这里的 SVG 要一起改。

## 颜色

| 名称 | HEX | RGB | 用途 |
| --- | --- | --- | --- |
| 墨色 Ink | `#1C1C1A` | 28 28 26 | 箭头（浅底）、应用图标底色、字标 |
| Lastab 蓝 | `#2F5BD3` | 47 91 211 | 光标（浅底），即界面强调色 |
| 亮蓝 | `#5B85F2` | 91 133 242 | 光标（深底） |
| 纸色 | `#F4F4F1` | 244 244 241 | 展示用浅底 |
| 开发橙 | `#C2410C` | 194 65 12 | 仅开发版图标 |

对比度：亮蓝对墨色 4.9:1，Lastab 蓝对白色 5.9:1，白色对开发橙 5.2:1。

允许的组合：浅底用 `color`，墨色或深底用 `reversed`，Lastab 蓝底用 `white`，单色印刷用 `black`。

## 字标

- 字体是 Inter SemiBold（SIL OFL，可用于标志），字距 −2%，已转成轮廓。
- 横版中，字标的大写高度在图形内容框里垂直居中，图形与字标间距为 64，即两个笔画宽。
- 不要直接打字重排字标，用 SVG 文件。

## 留白与最小尺寸

- **留白**：四周至少留一个光标宽度（笔画宽 32，约为图形高度的 1/5.5）。
- **最小尺寸**：

  | 版本 | 屏幕最小 |
  | --- | --- |
  | 横版 | 宽 96 px |
  | 竖版 | 宽 64 px |
  | 图形 | 16 px |

## 不要

- 拉伸、旋转，或把箭头改成朝右。
- 改变光标与箭头的颜色关系，例如光标用墨色、箭头用蓝色（开发版图标除外）。
- 加阴影、描边、渐变。
- 把图形和字标拆开重排，或改变两者的比例。
- 放在杂乱背景上而不加底色。
