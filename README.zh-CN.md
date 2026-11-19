<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="design/brand/lastab-horizontal-reversed.svg">
    <img src="design/brand/lastab-horizontal-color.svg" alt="Lastab" width="240">
  </picture>
</p>

<p align="center"><b>Back to the last tab.</b><br>像 Sublime Text 的 ⌘N 一样快的临时文本工作台，写过的都找得回来。</p>

<p align="center"><a href="README.md">English</a> · 简体中文</p>

---

Lastab 是一个常驻桌面的纯文本片段应用，适用于 macOS 和 Windows。按 ⌘N 打开空白片段，粘贴进来改一段，再复制出去用。不用起名，不用选位置，也不用保存。所有片段都在后台自动留存，之后可以按时间、标题或正文找回来，关掉的也一样。

## 功能

- **即时新建**：⌘N / Ctrl+N 直接进入空白编辑区，无需命名或分类。
- **自动留存**：边写边保存，异常退出后自动恢复。关闭片段不等于删除。
- **本地自动标题**：标题按内容在本机生成，只作列表元数据，不会混进正文，也不会被一起复制出去。
- **「最近」切换器**：⌘\` / Ctrl+\` 在已打开的片段之间切换，「快捷键 + 回车」两步回到上一条。
- **全文搜索**：⌘K / Ctrl+K 搜索全部片段，包括已关闭的。基于 SQLite FTS5 trigram，中文、英文和代码符号都能按子串匹配。
- **版本历史**：查看片段改过什么，恢复修改前的内容，恢复后还可以撤销。
- **待办清单**：⌘T / Ctrl+T 新建待办。存为纯文本 `- [ ]` 清单，粘贴多行时每行变成一项。
- **复制全部**：⌘⇧C / Ctrl+Shift+C 一键复制整条正文，原样保留行尾、Tab 和全角字符。
- **Agent 访问**：自带只读命令行 `lastab` 和一份 Claude Code Skill，Agent 可以搜索、读取你的片段。默认关闭，密钥类内容默认遮蔽。
- **深色模式与多语言**：浅色、深色或跟随系统；界面支持简体中文和英文；强调色可自定义。

## 安装

在 [Releases](https://github.com/momaek/lastab/releases) 下载：

| 平台 | 文件 | 说明 |
| --- | --- | --- |
| macOS 12+ | `lastab-<版本>.dmg` | 通用二进制（Apple 芯片与 Intel） |
| Windows 10 / 11 | `lastab-setup-<版本>-amd64.exe` / `-arm64.exe` | 按用户安装到 `%LOCALAPPDATA%\Programs\Lastab`，不需要管理员权限 |

未签名的构建在 macOS 上会被 Gatekeeper 拦下，Windows 上 SmartScreen 会给出警告。

## 快捷键

| 操作 | macOS | Windows |
| --- | --- | --- |
| 新建文本片段 | ⌘N | Ctrl+N |
| 新建待办清单 | ⌘T | Ctrl+T |
| 呼出 Lastab 并新建（全局） | ⌃⌥N | Ctrl+Alt+N |
| 「最近」切换器 | ⌘\` | Ctrl+\` |
| 全部片段与搜索 | ⌘K | Ctrl+K |
| 复制全部 | ⌘⇧C | Ctrl+Shift+C |
| 版本历史 | ⌘Y | Ctrl+Shift+Y |
| 显示 / 隐藏侧栏 | ⌘\\ | Ctrl+\\ |
| 关闭当前片段 | ⌘W | Ctrl+W |
| 待办：完成 / 排序 / 删除 | ⌘↵ / ⌥↑↓ / ⌘⇧⌫ | Ctrl+Enter / Alt+↑↓ / Ctrl+Shift+Backspace |

按住 ⌘（Windows 为 Ctrl）约半秒会显示当前界面的快捷键提示；在列表里按住 ⌥ / Alt 会显示具体时间。快捷键都可以在设置中修改。

## 给 Agent 用

在 **设置 → Agent 访问** 中：

1. 打开「允许 Agent 访问」（默认关闭，关闭时命令行不输出任何内容）。
2. 在「命令行工具」一行点「安装」。macOS 上会在 `/usr/local/bin` 建立 `lastab` 的符号链接，没有写权限时退回 `~/.local/bin`；Windows 上会把应用的 `bin` 目录加入用户 PATH。
3. 点「安装 Skill」，写入 `~/.claude/skills/lastab/SKILL.md`。

之后在 Claude Code 里说「找一下我昨天改的那个 curl」即可。其他 Agent 可以直接调用命令行，输出都是 JSON：

```bash
lastab search "orders" --since 1d --client my-agent
lastab list --open --kind todo
lastab read 01JD2F9C          # ID 或唯一前缀
lastab status
```

「遮蔽疑似密钥」默认开启：`Authorization`、`Bearer`、`api_key` 等会输出为 `••••••••`，只能在设置里关，Agent 无法用参数绕过。每次读取都会记录下来，可以在设置中的「访问记录」里查看。退出码与错误格式见 [`skill/SKILL.md`](skill/SKILL.md)。

## 数据

所有数据只保存在本机，不做同步。

| 平台 | 位置 |
| --- | --- |
| macOS | `~/Library/Application Support/Lastab/` |
| Windows | `%LOCALAPPDATA%\Lastab\` |

`lastab.db` 存放片段、版本和设置，`agent_log.db` 存放 Agent 访问记录。开发版使用单独的 `Lastab Dev` 目录。

## 开发

整个项目只用 Go。桌面框架是 [MyGo](https://github.com/egoist/mygo)，界面使用它的原生 UI（macOS 上用 Metal 绘制，Windows 上用 Direct3D 11），不依赖 WebView、Node 或 C 编译器。

需要 Go 1.27.4。MyGo CLI 以 Go tool 的形式锁定在 `go.mod` 中。

```bash
go tool mygo doctor
```

```bash
go run ./scripts/build -cli-only
```

```bash
go tool mygo dev
```

```bash
go test ./...
```

依次是：检查环境；为本机构建 `lastab` 命令行（开发版的「Agent 访问」会用到它）；启动「Lastab Dev」，修改 `.go` 文件后自动重启；运行全部测试，包括无窗口的视图测试。

项目通过 purego 调用原生代码，构建时设置 `CGO_ENABLED=0` 即可。

### 生成文件

修改 `design/tokens.json` 或 `locales/*.json` 后，需要重新生成对应文件（CI 会检查它们是否最新）：

```bash
go run ./scripts/gentokens
```

```bash
go tool mygo generate
```

### 打包

```bash
go run ./scripts/build -platform darwin/universal
```

```bash
go run ./scripts/build -platform windows/amd64,windows/arm64
```

产物在 `build/<平台>/`。签名与公证通过环境变量开启（见 [`scripts/build/main.go`](scripts/build/main.go)），没有设置时，macOS 版为 ad hoc 签名，Windows 版不签名。在 Mac 上打 Windows 安装包需要先 `brew install makensis`。

### 发布

把 `mygo.json` 的 `version` 改成新版本并合入 `main`，然后推送同名标签：

```bash
git tag v0.1.2 && git push origin v0.1.2
```

[`release.yml`](.github/workflows/release.yml) 会构建 macOS DMG 和 Windows 安装包，上传到草稿 Release，人工检查后再发布。

### 目录

| 路径 | 内容 |
| --- | --- |
| `main.go` | 桌面应用入口 |
| `cmd/lastab/` | 只读命令行 `lastab` |
| `internal/store/` | SQLite 存储、迁移与 FTS5 搜索 |
| `internal/snippet/` | 片段、工作区与异常恢复 |
| `internal/ui/` | 全部界面与交互 |
| `internal/title/`、`history/`、`todo/`、`redact/` | 自动标题、版本历史、待办解析、敏感内容遮蔽 |
| `internal/install/` | 安装命令行与 Skill |
| `locales/` | 界面文案（`en`、`zh-Hans`） |
| `design/` | 设计画布、设计变量与品牌标志 |
| `skill/` | 随应用分发的 Claude Code Skill |
| `scripts/` | 构建、图标与设计变量生成 |

### 文档

- [需求说明](Lastab-需求说明-v0.5.md)
- [开发文档](Lastab-开发文档-v0.1.md)：架构、存储、各模块设计、平台差异
- [开发计划](Lastab-开发计划-v0.1.md)
- [验收对照](docs/acceptance.md)
- [UI 设计](design/README.md)与[品牌标志](design/brand/README.md)
