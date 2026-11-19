# Lastab：开发文档 v0.1

2026-11-18

根据 [需求说明 v0.5](Lastab-需求说明-v0.5.md) 与 [UI 设计 v0.6](design/README.md)，给出 Lastab 的技术选型、架构、数据模型、模块设计、构建发布与测试方案。本文是技术方案草案；需求里仍「待确认」的项，在这里先按建议默认设计，并在末尾列出会影响实现的待确认点。

## 文档说明

| 项 | 内容 |
| --- | --- |
| 版本 | v0.1（首版） |
| 对应需求 | `Lastab-需求说明-v0.5.md` |
| 对应设计 | `design/`（设计 v0.6，颜色等变量见 `design/tokens.json`） |
| 桌面框架 | [MyGo](https://github.com/egoist/mygo)，全部窗口使用原生 UI（已确认） |
| 状态 | 技术方案草案；第 12 节的技术验证完成后再定稿 |

本文用到的标记：

| 标记 | 含义 |
| --- | --- |
| **定** | 本文给出的技术决定，无特殊情况按此实现 |
| **验证** | 依赖框架或平台行为，开工前需先做技术验证（见第 12 节） |
| **待确认** | 依赖需求待确认项，先按建议默认实现，保留改动余地 |

## 1. 技术选型总览

需求倾向使用 Go。桌面框架用 MyGo，界面全部用它的**原生 UI**：用 Go 和 `ui` 包编写，MyGo 自己在 GPU 上绘制（macOS 用 Metal，Windows 用 Direct3D 11），不启动 WebView。这样窗口立即打开、内存占用小，对「新建不慢于 Sublime ⌘N」最有利。整个项目只有 Go，没有前端工程。

### 1.1 选型表

| 层 | 选型 | 版本 | 用途 / 理由 |
| --- | --- | --- | --- |
| 语言 | Go | 1.27.4（MyGo 要求 ≥ 1.27.3） | 应用、命令行、构建脚本都用 Go |
| 桌面框架 | MyGo | v0.10.0（`go.mod` 锁定，升级单独评审） | 窗口、菜单、托盘、全局快捷键、单实例、登录时启动、剪贴板、深色模式、打包签名 |
| 界面 | MyGo 原生 UI（`github.com/egoist/mygo/ui`） | 随 MyGo | 声明式视图（每帧由状态生成）；flex / grid 布局、虚拟化列表、输入法、无障碍、动效 |
| 文本编辑器 | `github.com/egoist/mygo/ui/editor` | 随 MyGo | Zed 风格编辑器：保留每行原有行尾、大文本只排版可见行、支持输入法；用作纯文本编辑器 |
| 版本差异 | `ui/editor` 的差异查看器 | 随 MyGo | 版本历史中的分栏 / 合并差异视图 |
| 多语言 | MyGo i18n：ARB 消息目录 + 生成的带类型 Go 函数 | 随 MyGo | ICU MessageFormat、复数、CLDR 日期与相对时间 |
| 数据库 | SQLite（WAL 模式） | — | 单文件本地存储；应用与命令行可同时访问 |
| SQLite 驱动 | `modernc.org/sqlite` | 锁定具体版本 | 纯 Go、无 cgo；应用和命令行用同一个驱动和同一版 SQLite，FTS5 行为一致（不用 MyGo 的 SQLite 插件，原因见 4.5） |
| 全文搜索 | SQLite FTS5 + `trigram` 分词器 | — | 中文、英文、代码符号的子串匹配（需求明确指出按词切分不适合） |
| 差异统计 | `github.com/sergi/go-diff` | v1.x | 保存版本时计算「改动 N 行」 |
| ID | ULID（`github.com/oklog/ulid/v2`） | v2.x | 按时间可排序；命令行可用前缀读取（如 `read 2f9c`） |
| 命令行框架 | `github.com/spf13/cobra` | v1.x | `lastab` 子命令、参数、帮助 |
| 日志 | `log/slog` + `gopkg.in/natefinch/lumberjack.v2` | — | 结构化日志，按大小滚动，写入 MyGo 的日志目录 |
| 数据库迁移 | 自写顺序迁移（`PRAGMA user_version`） | — | 表少，不引入迁移框架 |
| 字体 | Inter、JetBrains Mono（OFL 许可，`//go:embed` 后用 `ui.RegisterFont` 注册） | — | 中文回退到系统字体（PingFang SC / Microsoft YaHei） |
| 设计变量 | `design/tokens.json` → 生成 `internal/theme/tokens_gen.go` | — | 浅色 / 深色两套同名颜色 |
| 测试 | `go test` + `ui.NewTester`（无窗口运行视图）+ `testify` | — | 单元、视图、交互测试都在 `go test` 中运行 |
| 端到端测试 | MyGo 自动化插件（`-tags e2e`）+ `mygo-automation` 客户端 | 随 MyGo | 从外部驱动构建好的应用：查找元素、发送按键、截图 |
| 静态检查 | `golangci-lint`、`staticcheck`、`govulncheck`、`mygo i18n check` | — | CI 必过 |
| 构建工具 | `go tool mygo`（`dev` / `build`）+ `scripts/build`（Go 程序） | — | 不引入 Make / Taskfile / Node / Bun |
| CI | GitHub Actions（`macos-latest`、`windows-latest`） | — | 参考 MyGo 的 GitHub Actions 文档：构建、签名、公证、发布 |
| macOS 打包 | `mygo build -platform darwin/universal` → `.app` + DMG；Developer ID 签名 + 公证 | — | MyGo 内置签名与公证流程 |
| Windows 打包 | `mygo build -platform windows/amd64,windows/arm64` → NSIS 安装包 | — | 按用户安装到 `%LOCALAPPDATA%\Programs\Lastab`，无需管理员；Authenticode 签名 |

### 1.2 本地开发环境

| 工具 | 版本 | 说明 |
| --- | --- | --- |
| Go | 1.27.4 | macOS 上用 Homebrew 安装（`/opt/homebrew/bin/go`） |
| MyGo CLI | 随 `go.mod` | 作为 Go 工具：`go get -tool github.com/egoist/mygo/cmd/mygo@v0.10.0`，之后用 `go tool mygo …` |
| gopls / staticcheck / govulncheck | 用 Go 1.27.4 构建 | 编辑器与静态检查 |
| NSIS | 3.x | 只在 macOS 上交叉构建 Windows 安装包时需要（`brew install makensis`）；Windows 上 MyGo 会自动下载 |
| Xcode 命令行工具 | — | macOS 签名、公证、制作 DMG |

不需要 C 编译器、Node、Bun 或 WebView2 Runtime。`go tool mygo doctor` 用于检查环境。

### 1.3 不选的方案（简述）

| 方案 | 不选的原因 |
| --- | --- |
| MyGo 的 Web 前端（WebView + Svelte / CodeMirror） | WebView 冷启动更慢；macOS 系统级智能引号可能作用于 WebView 编辑区；需要 Bun 和前端工程 |
| MyGo 原生 UI + Web 混合 | 两套界面代码；设置页面也没有必须用 Web 的理由 |
| Wails、Fyne、Gio | 已选定 MyGo |
| MyGo 的 SQLite 插件 | 见 4.5 |

### 1.4 框架风险（**定**）

MyGo 很新：仓库创建于 2026-09，当前是 v0.10.0，1.0 之前 API 可能变化。应对方式：

- `go.mod` 锁定具体版本；升级时单独提交，读它的更新说明，并跑完整测试。
- 界面代码集中在 `internal/ui`，业务逻辑（存储、搜索、标题、遮蔽、版本）放在不依赖 MyGo 的包里，命令行也只依赖这些包。
- 第 12 节的技术验证先覆盖对本项目最关键的能力：编辑器保真、输入法、⌘\`、全局热键、启动速度。

## 2. 总体架构

```
┌────────────────────── Lastab.app / Lastab.exe（单个 Go 进程，常驻）──────────────────────┐
│                                                                                          │
│  internal/ui（MyGo 原生 UI，主线程）              业务层（不依赖 MyGo）                     │
│  ┌──────────────────────────────┐  直接调用   ┌─────────────────────────────────┐         │
│  │ 左侧栏（已打开的片段）         │ ─────────▶ │ snippet   片段与工作区            │         │
│  │ 编辑器（ui/editor）             │            │ store     SQLite 读写与迁移       │         │
│  │ 待办清单                       │ ◀───────── │ search    全文搜索               │         │
│  │ 「最近」切换器（窗口内浮层）     │ Window.    │ history   版本快照与差异统计      │         │
│  │ 全部片段与搜索（⌘K）            │ Update     │ title     后台自动标题（goroutine）│         │
│  │ 版本历史（右侧抽屉 + 差异视图）  │            │ redact    密钥遮蔽               │         │
│  │ 设置 / Agent 访问              │            │ todo      待办纯文本解析          │         │
│  └──────────────────────────────┘            └────────────────┬────────────────┘         │
│  internal/shell：菜单、托盘、全局快捷键、单实例、登录时启动、剪贴板、CLI 与 Skill 安装        │
└───────────────────────────────────────────────────────────────┼─────────────────────────┘
                                                                │ 读写
                          ┌─────────────────────────────────────▼────────────────┐
                          │ lastab.db（SQLite，WAL）  片段 / 版本 / 设置 / FTS     │
                          │ agent_log.db（SQLite）    Agent 访问记录              │
                          └─────────────────────────────────────▲────────────────┘
                                                                │ 只读 lastab.db，写 agent_log.db
                          ┌─────────────────────────────────────┴────────────────┐
                          │ lastab（命令行，独立进程，应用未运行时也可用）           │
                          │ Claude Code 通过 SKILL.md 调用；其他 Agent 直接调用     │
                          └──────────────────────────────────────────────────────┘
```

要点（**定**）：

- **没有前后端之分**：视图是 Go 函数，每帧由应用状态生成界面，直接调用业务层，不需要绑定或 IPC。
- **线程模型**：视图和 MyGo API 在主线程运行。数据库写入、标题生成、搜索在 goroutine 中执行，结果通过 `Window.Update(func(){…})` 交回主线程修改状态。
- **常驻后台**：`App.OnWindowAllClosed(func(){})` 让应用在窗口关闭后继续运行；主窗口的 `OnClose` 改为隐藏，不销毁。按全局热键时直接显示已有窗口并新建 tab。
- **单实例**：`App.RequestSingleInstanceLock()`；再次启动时，在 `OnSecondInstance` 中显示并聚焦主窗口。
- **一个主窗口**：tab、切换器、⌘K 列表、版本历史抽屉、设置都在主窗口内，用 MyGo 的浮层与页面路由实现，不开第二个窗口。
- **业务层与界面分离**：`internal/` 下除 `ui`、`shell` 外的包都不导入 MyGo，命令行 `cmd/lastab` 只依赖这些包，体积小、启动快。

## 3. 仓库结构

```
lastab/
├── Lastab-需求说明-v0.5.md
├── Lastab-开发文档-v0.1.md
├── design/                     # 设计源文件与 tokens.json（已存在）
├── go.mod                      # go 1.27.3+；tool github.com/egoist/mygo/cmd/mygo
├── mygo.json                   # 应用名、identifier、版本、i18n、最低系统版本；签名设置由 scripts/build 从环境变量注入，不进仓库
├── main.go                     # 桌面应用入口（App.Run）
├── e2e.go                      # //go:build e2e，导入自动化插件
├── cmd/
│   └── lastab/                 # 命令行入口（cobra）
├── internal/
│   ├── paths/                  # 数据目录（应用与命令行共用，见 4.1）
│   ├── store/                  # SQLite 打开、迁移、事务；片段/版本/设置的读写
│   ├── snippet/                # 片段与工作区领域逻辑（空白判定、关闭、删除）
│   ├── title/                  # 本地自动标题规则
│   ├── search/                 # FTS5 查询、短查询回退、命中片段
│   ├── history/                # 快照策略、去重、改动统计
│   ├── redact/                 # 密钥识别与遮蔽（列表与 Agent 输出共用）
│   ├── todo/                   # 待办纯文本解析与序列化
│   ├── agentlog/               # agent_log.db 读写
│   ├── theme/                  # tokens_gen.go（生成）+ 主题构建、强调色派生
│   ├── shell/                  # 菜单、托盘、全局快捷键、单实例、登录时启动；实现 appui.Platform（含 CLI/Skill 安装）
│   ├── install/                # 「安装命令行」与「安装 Skill」：符号链接 / 用户 PATH / 写 SKILL.md（纯 Go，可在临时目录测试）
│   └── ui/                     # 原生 UI：视图、组件、快捷键表
│       ├── app.go              # 应用状态与根视图
│       ├── sidebar.go  view.go     # 左侧栏、编辑区标题栏、状态栏
│       ├── editor.go           # ui/editor 的配置与包装
│       ├── todo.go  switcher.go  library.go  history.go  settings.go  agent.go
│       ├── toast.go  confirm.go  # 按设计自绘的轻提示与确认气泡
│       └── keys/               # 快捷键默认表与平台显示（⌘ vs Ctrl）
├── locales/                    # en.json（源语言）、zh-Hans.json；catalog_gen.go（生成，需提交）
├── skill/
│   ├── SKILL.md                # 安装到 ~/.claude/skills/lastab/ 的模板（含 lastab-skill-version 版本注释）
│   └── skill.go                # //go:embed SKILL.md，供设置页写出
├── resources/
│   ├── icon.png                # 1024×1024 占位应用图标（scripts/genicon 生成）；icon-dev.png 为开发版图标（橙色）
│   ├── tray.png                # 托盘图标（macOS 模板图）
│   ├── fonts/                  # 由 //go:embed 使用，不进入资源目录（resourcesExclude）
│   ├── darwin-arm64/bin/lastab           # 由 scripts/build 生成，不提交
│   ├── darwin-amd64/bin/lastab
│   ├── windows-amd64/bin/lastab.exe
│   └── windows-arm64/bin/lastab.exe
└── scripts/
    ├── gentokens/              # design/tokens.json → internal/theme/tokens_gen.go
    ├── genicon/                # 画占位图标 resources/icon.png、icon-dev.png
    └── build/                  # 先交叉编译 lastab 到 resources/<平台>/bin，再调用 mygo build；从环境变量注入签名设置
```

`.github/workflows/`：`ci.yml`（PR 与 main）、`release.yml`（`v*` 标签，见 10.3）。注意 `.gitignore` 只忽略根目录的 `/build/`，不能写成 `build/`，否则会连 `scripts/build/` 一起忽略（早期版本就因此漏提交了 `scripts/build`）。

## 4. 数据与存储

### 4.1 数据目录（**定**）

| 平台 | 数据目录 | 文件 |
| --- | --- | --- |
| macOS | `~/Library/Application Support/Lastab/` | `lastab.db`、`agent_log.db`、`session.lock` |
| Windows | `%LOCALAPPDATA%\Lastab\` | 同上 |

- 路径由 `internal/paths` 计算，应用与命令行共用同一份代码；不使用 MyGo 的 `PathUserData`，因为它在 Windows 上指向 Roaming（`%AppData%`）。需求不做同步，本地数据不应随域账户漫游，而且命令行不依赖 MyGo。
- 开发版（`go tool mygo dev` 启动的 `Lastab Dev`）使用 `Lastab Dev` 目录，与已安装的应用分开。命令行用 `--data-dir` 或环境变量 `LASTAB_DATA_DIR` 指向开发数据。
- 日志写入 MyGo 的 `PathLogs`（macOS 为 `~/Library/Logs/Lastab`）。

### 4.2 表结构（`lastab.db`）

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;      -- WAL 下兼顾安全与速度；崩溃最多丢最后一次提交
PRAGMA foreign_keys = ON;

CREATE TABLE snippet (
  id            TEXT PRIMARY KEY,          -- ULID
  kind          TEXT NOT NULL CHECK (kind IN ('text','todo')),
  body          TEXT NOT NULL,             -- 正文原样（含原有行尾）；待办为 "- [ ] 事项" 纯文本
  title         TEXT,                      -- 列表元数据，绝不写入 body
  title_source  TEXT NOT NULL DEFAULT 'pending' CHECK (title_source IN ('pending','auto','manual')),
  created_at    INTEGER NOT NULL,          -- Unix 毫秒，创建后不变
  modified_at   INTEGER NOT NULL,          -- 正文最后修改时间
  rev           INTEGER NOT NULL DEFAULT 0 -- 每次写入 +1，防止旧写覆盖新写
);
CREATE INDEX snippet_modified ON snippet(modified_at DESC);

-- 工作区：已打开的 tab（「最近」切换器只看这里）
CREATE TABLE workspace_tab (
  snippet_id  TEXT PRIMARY KEY REFERENCES snippet(id) ON DELETE CASCADE,
  position    INTEGER NOT NULL,            -- 侧栏顺序（按最近活动排好后写入，重启时恢复）
  opened_at   INTEGER NOT NULL,
  is_active   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE version (
  id            TEXT PRIMARY KEY,          -- ULID
  snippet_id    TEXT NOT NULL REFERENCES snippet(id) ON DELETE CASCADE,
  body          TEXT NOT NULL,
  created_at    INTEGER NOT NULL,
  reason        TEXT NOT NULL CHECK (reason IN ('paused','switch','close','copy','restore')),
  lines_changed INTEGER NOT NULL           -- 相对上一个版本，用于「改动 1 行」
);
CREATE INDEX version_snippet ON version(snippet_id, created_at DESC);

-- 全文搜索：外部内容表，触发器同步
CREATE VIRTUAL TABLE snippet_fts USING fts5(
  title, body,
  content = 'snippet', content_rowid = 'rowid',
  tokenize = 'trigram case_sensitive 0'
);
-- （插入/更新/删除触发器略）

CREATE TABLE setting (
  key    TEXT PRIMARY KEY,
  value  TEXT NOT NULL                    -- JSON
);
```

说明：

- **空白不留空壳**（**定**）：新建 tab 时只在内存中，不写数据库；第一次出现非空白字符（不只是空格与换行）时才插入 `snippet` 行。已有内容的片段被清空时不删除。
- **关闭不等于删除**：关闭 tab 只删除 `workspace_tab` 行；`snippet` 保留。
- **删除是明确操作**：从 ⌘K 列表删除时，同一事务里删除片段、版本与 FTS 行（硬删除，配合设计中的锚定确认气泡）。
- **时间**：库里一律存 UTC 毫秒；显示时按界面语言本地化。
- **设置项**（`setting` 表）：`appearance`、`language`、`accent`、`shortcuts`（自定义快捷键表）、`editor.font`、`editor.fontSize`、`holdHint`、`openAtLogin`、`agent.access`、`agent.mask`、`agent.allowRaw`。

### 4.3 应用与命令行并发访问（**定**，细节 **验证**）

- 应用是唯一写入 `lastab.db` 的进程。写入用单个连接串行化，读取用连接池。
- 命令行用 `file:lastab.db?mode=ro` 只读打开，并设 `busy_timeout = 2000`。WAL 模式下读者不阻塞写者，不会锁住应用或损坏数据。
- 命令行要记录 Agent 访问，就写入独立的 `agent_log.db`。这样主库对命令行始终只读。
- 应用未运行时，`-wal` / `-shm` 可能已清理，只读打开仍可读（技术验证中确认）。

### 4.4 迁移

- `PRAGMA user_version` 记录版本；`internal/store/migrations/NNN_*.sql` 用 `embed` 打包，顺序执行，每个迁移一个事务。
- 命令行遇到比自己新的 `user_version` 时，拒绝读取，并提示升级（避免旧 CLI 读新库出错）。

### 4.5 为什么不用 MyGo 的 SQLite 插件

MyGo 的 SQLite 插件是用 Zig 编写的原生模块，由应用从自己的资源目录加载，API 也是插件自己的（`Execute` / `Query` / `Transaction`）。`lastab` 命令行是独立的二进制，需要在应用未运行时自己打开数据库；让它也加载这个原生模块会把命令行绑在应用的资源布局上。改用 `modernc.org/sqlite`（纯 Go，`database/sql` 接口），应用与命令行用同一个驱动、同一版 SQLite，FTS5 trigram 行为也一致。

## 5. 核心模块设计

### 5.1 片段与工作区（`internal/snippet`）

业务层 API（界面直接调用，签名示意）：

```go
type Service struct{ /* store, title worker, history … */ }

func (s *Service) Workspace(ctx context.Context) (Workspace, error)                // 启动时恢复已打开的 tab
func (s *Service) Create(ctx context.Context, kind Kind, body string) (Snippet, error) // 首次有实质内容时调用
func (s *Service) SaveBody(ctx context.Context, id, body string, baseRev int) (SaveResult, error)
func (s *Service) Open(ctx context.Context, id string) (Snippet, error)             // 从列表打开，加入工作区
func (s *Service) Close(ctx context.Context, id string) error                      // 移出工作区，触发 'close' 快照
func (s *Service) Delete(ctx context.Context, id string) error                     // 永久删除
func (s *Service) Reorder(ctx context.Context, ids []string) error
func (s *Service) Rename(ctx context.Context, id, title string) error              // 手动标题，title_source = manual
```

后台结果（标题生成完成、保存失败等）通过回调交给界面，界面在回调里用 `Window.Update` 修改状态，触发重绘。

### 5.2 自动留存与异常恢复（P0）

- **写入时机**：输入停止 300ms 后保存；切换 tab、关闭 tab、窗口失焦、应用退出时立即保存。保存在 goroutine 中执行，不阻塞输入。
- **读取正文**：用 `Buffer.WriteTo` 取得正文（保留每行原有行尾），不用会统一换行符的读取方式（**验证** `String()` 是否保留行尾）。
- **防覆盖**：`SaveBody` 带 `baseRev`，库中 `rev` 不一致时返回冲突（正常情况下只有一个写者，冲突只用于防止乱序）。
- **状态栏**：「正在留存… / 已自动留存」；写入失败（如磁盘已满）时，状态栏常驻「未能留存：原因」和「重试」，正文保留在内存的 `Buffer` 中，后续每次编辑继续尝试写入（对应设计「留存失败」画板）。
- **异常退出检测**：启动时写 `session.lock`（含 PID），正常退出时删除。启动时发现残留的锁且该 PID 不在运行，就判定为异常退出。此时恢复工作区，并在状态栏安静提示「已恢复」（对应设计「异常退出后恢复」画板）。
- **退出与关机**：在 `App` 的退出监听中同步保存所有未写入的正文，然后才退出。

### 5.3 本地自动标题（`internal/title`，P0）

- 在后台 goroutine 中生成，绝不阻塞编辑与复制。片段第一次有实质内容后，等输入停止约 1 秒再生成。
- 生成后 `title_source = auto`，之后保持稳定；用户手动改名后为 `manual`，不再自动覆盖。生成前列表与 tab 显示「未命名 · 14:52」。
- 规则（按顺序命中即用）：

| 内容 | 标题示例 |
| --- | --- |
| `curl` 命令 | `curl · POST /v2/orders/search`（方法 + 路径） |
| JSON | `JSON · order_id A-20391`（第一个有识别度的键值） |
| 待办 | 第一项内容 + 进度，如 `发版前检查 · 3/7`（进度实时更新，不算「改标题」） |
| 其他 | 第一行有意义的内容（跳过只含 `{`、`[`、`\`、空白等符号的行），截断到 40 个字符 |

- 标题在入库前同样经过 `redact`：不会把 `Authorization: Bearer xxx` 当作标题明文显示。
- 列表上显示的时间戳在渲染时按语言格式化，不写死在标题字符串里，所以切换语言后时间格式会跟着变。

### 5.4 全文搜索（`internal/search`，P0）

- **索引**：FTS5 `trigram` 分词器，`title` 与 `body` 两列，覆盖所有片段（含已关闭的）。默认只搜当前内容，不搜旧版本（**待确认**，需求待确认问题 10）。
- **查询长度 ≥ 3 个字符**：用 `snippet_fts MATCH`，按 `bm25()` 加最近修改时间综合排序。
- **查询长度 < 3 个字符**（如「周会」「{"」）：trigram 无法命中，回退为 `body LIKE '%q%' OR title LIKE '%q%'` 全表扫描；不含 ASCII 字母时用等价而更快的 `instr()`。1 万条下最坏约 32 ms（实测见 12.1.1）。
- **过滤**：类型（文本 / 待办）、时间范围（今天、7 天、30 天、自定义），以及「只看已打开」。
- **结果**：返回命中位置附近的一段正文与高亮区间（界面用 `RichText` 的 `Span` 高亮）；预览先经过 `redact`。
- 搜索在 goroutine 中执行，带取消：输入变化时取消上一次查询。
- 「最近」切换器的输入过滤只在已打开的 tab 中进行，在内存里完成，不查数据库。

### 5.5 版本历史（`internal/history`，P1）

- **快照时机**：停止输入 3 秒（`paused`）、切换到别的片段（`switch`）、关闭（`close`）、复制全部（`copy`）、恢复旧版本前（`restore`，保证恢复可以撤销）。
- **去重**：与上一个版本完全相同就不记录。
- **改动统计**：用 `go-diff` 的按行模式，入库时算好 `lines_changed`。
- **展示**：⌘Y / Ctrl+Shift+Y 打开（⌘K 预览里也有「版本历史」按钮）。右侧 340px 抽屉列出版本：时间、原因、改动行数、第一处改动的行；最新一条标「当前」，最早一条标「首次内容」。编辑区换成对比区：`ui/editor` 的差异查看器（合并视图，带 +/−），可切换「原文」只读查看；默认选中「当前」的前一个版本。打开时先记录当前内容，保证「当前」就是编辑区的文字；历史跟随当前片段，切换片段时重新载入。
- **对比的写法**：差异与原文都经过 `redact` 遮蔽；比较前两边都补上结尾换行，不显示 git 式的「文件末尾没有换行」。复制此版本、恢复都用原文。
- **恢复与撤销**：恢复时先把当前内容记为版本（`restore`，`restored_from` 为空，显示「恢复前」），再把恢复后的内容记为版本（`restore`，`restored_from` 为来源版本 ID，显示「恢复自 14:34」）。`restored_from` 由迁移 002 加入。恢复后顶部轻提示「已恢复到 14:34 的版本 · 撤销」，5 秒倒计时，悬停时暂停；撤销后提示「已撤销恢复，回到最新内容」3 秒。只有一个版本时不能恢复。
- **保留**：首期全部保留（**待确认**，需求待确认问题 10）。

### 5.6 「最近」切换器（P0）

纯界面实现，数据来自工作区（只含已打开的 tab），顺序与左侧栏相同：按最近活动倒序（见 6.1.1）。

| 行为 | 实现 |
| --- | --- |
| 弹出 | ⌘\` / Ctrl+\` 打开窗口内浮层（`ui.Overlay`），过滤输入框 `AutoFocus`，默认选中第 2 项（当前之外最近修改的那条） |
| 移动 | Tab / ↓ 下一项，Shift+Tab / ↑ 上一项；再按 ⌘\` 同样是下一项。浮层用 `HandleInput` 接收 Tab，不让它移动键盘焦点 |
| 打开 / 取消 | Enter 切换到选中 tab；Esc 关闭 |
| 保持打开 | 松开修饰键不关闭；不采用「按住 ⌘ 连按 Tab」 |
| 过滤 | 输入即按标题与正文过滤（不区分大小写的子串匹配） |
| 预览 | 选中项展开几行正文预览（经过遮蔽） |
| 无障碍 | 组合框 + 列表的角色，选中项随方向键播报 |
| 动效 | 无进出场动画，浅遮罩 10% |

macOS 上 ⌘\` 是系统「在同一应用的窗口之间切换」的快捷键，见 7.2 与第 12 节。

### 5.7 待办清单（`internal/todo` + `internal/ui/todo.go`，P1）

- **存储**：纯文本 `- [ ] 事项` / `- [x] 事项`，一行一项。「复制全部」得到的就是这段纯文本；Agent 读到的也是它，另附解析好的 `items` 数组。
- **解析**：`internal/todo` 负责解析与序列化，用 `testdata/*.txt` 做往返测试。不合格式的行按普通事项处理，不丢内容。
- **与保存的衔接**（2026-11-19 实现）：待办 tab 仍以编辑器的 buffer 作为正文，`internal/ui/todo.go` 从正文解析出事项列表，每次改动后用 `todo.Format` 写回 buffer。自动留存、版本、「复制全部」、「最近」切换器、⌘K 搜索与带进度的标题因此都走原有的 `Tab` 保存路径，无需另开通道。buffer 被别处改动（如恢复版本）时重新解析。打开清单不改写正文：只有用户真的改了内容才写回，不合格式的正文在此之前逐字节保持原样。
- **编辑**：每项是一个单行 `ui.TextInput`，左侧是 `ui.Checkbox`。新建的清单没有事项，焦点在末尾的「新增一项」行；这一行的文字不为空时已算入正文（按回车前退出也不丢），回车后成为一项。在事项里：Enter 在下方新增一项；⌘Enter / Ctrl+Enter 切换完成；⌥↑↓ / Alt+↑↓ 在所在分组内调整顺序；⌘⇧⌫ / Ctrl+Shift+Backspace 删除此项（轻提示可撤销）；空项按 ⌫ 直接删除（不提示），焦点回到上一项末尾；↑↓ 在各项之间移动焦点。只有点勾选框才切换完成状态，点文字是编辑。悬停一项时显示删除按钮，效果同 ⌘⇧⌫。
- **已完成项**：勾选后原位划线 0.9 秒（`SettleFor`），再沉到「已完成」分组（按设计先这样做，**待确认**，需求待确认问题 6）。正文的顺序与界面一致：未完成项在前，已完成项在后；新完成的一项排在已完成分组最前，取消勾选的一项回到未完成项末尾。「已完成」分组可折叠；提供「清除已完成」，轻提示 5 秒内可撤销。
- **粘贴多行**：两行及以上时每个非空行成为一项（`- [x]` 行保留完成状态），插在当前项之后；粘进空项时它取第一行；粘进「新增一项」行时接在未完成项末尾。新增的几项短暂高亮，同时给出可撤销的轻提示。单行粘贴照常插入文字。
- **拖动排序**：未完成分组按住行左侧的抓手拖动（按在文字上是编辑与选择）。没有用 `ListState.Reorder`：它的落点线固定为强调色，也不提供拖动中的落点位置，做不出设计的效果。改为由抓手自己跟踪按压（`PressedButton` 在指针离开抓手后仍有效，位置取自所在列的 `PointerPosition`）。指针移动 4 DIP 后这一项浮起：缩至 90%、倾斜 −0.6°、带面板底色与阴影，跟随指针；其余各项按 8px×0.7ⁿ 向落点线两侧让位（`AnimateTo` 平滑过渡），落点线为 2px 的 `DropLine` 色，左缩进 48px。松手后移到落点，Esc 放回原处。已完成分组不支持拖动，用 ⌥↑↓ 调整。

### 5.8 复制（P0）

- 「复制全部」用 `mygo.Clipboard.WriteText` 写入纯文本，内容只有正文（由 `Buffer.WriteTo` 得到，保留原有行尾），不含标题。
- 快捷键 ⌘⇧C / Ctrl+Shift+C；按钮原地变成「已复制」约 2 秒，同时向辅助技术播报，不弹轻提示。
- 选中部分复制走编辑器自带的复制命令（**验证**：复制出去的行尾是否保持原样）。

### 5.9 敏感内容遮蔽（`internal/redact`，P1）

- **规则**：一组自维护的正则（可参考 gitleaks 的公开规则，注意许可），覆盖：
  - `Authorization:` 头、`Bearer` token；
  - `api_key` / `apikey` / `secret` / `token` / `password` 的键值；
  - 常见密钥格式：`sk-…`、`sk-ant-…`、`ghp_…`、`xox[bp]-…`、`AKIA…`、JWT（`eyJ…`），以及 `-----BEGIN … PRIVATE KEY-----`。
- **输出**：只遮蔽值，保留键名与结构，例如 `Authorization: Bearer ••••••••••••`，便于用户认出是哪一条。
- **用于**：列表标题与预览、⌘K 预览面板（设计里有「显示」按钮，可临时显示原文）、切换器预览，以及命令行输出。
- 编辑区内不遮蔽：用户要编辑原文。
- 片段正文在库中保存原文；遮蔽只在输出时进行（需求：本地存储暂不加密）。

### 5.10 设置

- **外观**：跟随系统 / 浅色 / 深色，用 `mygo.Theme.SetSource(ThemeSystem / ThemeLight / ThemeDark)`。启动时在 `App.Run` 之前按已保存的设置调用，应用直接以正确外观启动。
- **语言**：跟随系统 / 简体中文 / English；用 `App.SetLocale`（空字符串为跟随系统），即时生效，MyGo 会记住选择供下次启动使用。
- **强调色**：设计中的 4 个预设，用 `Theme.WithAccent` 派生悬停、按下、选中与焦点色。
- **其他**：快捷键自定义（录制新组合键、冲突检测、全部恢复默认）；编辑字体与字号（`ui.FontFamilies()` 中 `Monospaced` 为真的字体，字号 11–20）；登录时启动（`App.SetOpenAtLogin`，默认关闭，**待确认**）；按住 ⌘ 显示快捷键（默认开；单独按住主修饰键约 0.5 秒后，带快捷键的按钮下方出现反色小标签，松开或按下其他键即隐藏；切换器打开或录制快捷键时不显示）；菜单栏 / 通知区域图标（默认开；菜单：显示 Lastab、新建片段、全部片段、设置、退出；Windows 上单击图标显示窗口）。
- **页面**（2026-11-19 实现）：⌘, / Ctrl+, 打开整页设置（macOS 在应用菜单「设置…」，Windows 在「文件」菜单），左侧分类：通用、快捷键、编辑与文本保真、Agent 访问、存储、关于；esc 返回编辑。每项修改立即生效并写入 `setting` 表。外观、语言、登录启动、菜单与全局热键通过 `appui.Platform` 接口交给 `internal/shell`，视图测试用假实现。
- **Agent 访问**：「允许 Agent 访问」默认**关闭**（**待确认**，需求待确认问题 4）；「遮蔽疑似密钥」默认开启，写入 `agent.allowRaw` 的反值。访问记录读取 `agent_log.db` 最近 20 条。「安装命令行」「安装 Skill」随 M4 的打包一起做。
- **存储**：数据目录、片段数与版本数、占用空间，「显示所在位置」。清理策略待需求待确认问题 11。

## 6. 界面（MyGo 原生 UI）

### 6.1 视图与设计画板对应

| 视图（`internal/ui`） | 设计画板 | 主要用到的 MyGo 组件 | 主要状态 |
| --- | --- | --- | --- |
| `sidebar.go` | 所有画板（待改，见 6.1.1） | `TitleBarHidden` + `c.TitleBar()` + `DragWindow`；侧栏用自定义列（纵向 tablist / tab 角色） | macOS 红黄绿压在侧栏顶部 / Windows 系统窗口按钮在右上角；按最近活动排序；可收起 |
| `editor.go` | 0 新建空白片段、1 编辑器 | `editor.View` | `blank` / `rest` / `pending`（标题未生成）/ `saveFailed` / `copied` / `recovered` / 按住 ⌘ 提示 / 图标提示（`Tooltip`） |
| `statusbar.go` | 编辑器 | 自定义行 | 留存状态、创建 / 修改时间、`纯文本 · UTF-8 · LF`（按实际行尾显示）、字数、复制全部 |
| `switcher.go` | 2「最近」切换器 | `Overlay` + `TextInput` + 自定义列表 | 默认、输入过滤、无结果 |
| `library.go` | 3 全部片段与搜索 | `SearchField`、`List`（虚拟化）、`Segmented`、`Split` | 分组列表、筛选、预览面板、无结果、删除确认 |
| `history.go` | 4 版本历史（右侧抽屉） | 抽屉浮层 + `editor` 差异查看器 | 版本列表、差异、恢复后撤销、只有一个版本 |
| `settings.go` | 5 设置 | `Sidebar`、`Segmented`、`Switch`、`Select` | 通用、快捷键（录制中）、文本保真 |
| `agent.go` | 7 Agent 访问 | `Switch`、`Button`、`Spinner`、`Table` | 开关、安装中、安装失败、访问记录 |
| `todo.go` | 6 待办清单 | `List` + `Checkbox` + `TextInput` | 默认、拖动中、粘贴拆分 |
| `toast.go` | 状态页 | 自绘（MyGo 的 toast 在窗口底部，设计要求顶部居中） | 一次一条、标题栏下 12px、悬停暂停、可撤销倒计时 |
| `confirm.go` | 删除确认 | `Popover`（锚定在按钮上） | 写明删除对象与后果 |

### 6.1.1 左侧栏（2026-11-18 调整，替代顶部 tab 栏）

已打开的片段从顶部横排 tab 改为左侧竖排列表。原因：开到 20 条左右时横排 tab 只剩几个字，认不出；竖排能显示完整标题与时间，直接服务「找不回刚编辑的那条」。

| 项 | 做法（**定**） |
| --- | --- |
| 宽度与收起 | 232px；⌘\\ / Ctrl+\\ 或侧栏顶部按钮收起，收起后编辑区标题栏左侧出现「显示侧栏」与「新建」按钮。窗口最小宽度 640 |
| 每一项 | 标题（当前项加粗）+ 最近活动的相对时间（按住 ⌥ 显示具体时间）；待办带图标；悬停或当前项显示关闭按钮 |
| 排序 | 按最近活动倒序。最近活动 = 最后一次编辑或打开（从 ⌘K 打开已关闭的片段也算），单纯点击切换不算 |
| 何时重排 | 只在「落定」时重排：切换到别的片段、新建、关闭、从 ⌘K 打开或返回、打开切换器、启动恢复时。**输入过程中不重排**，正在编辑的那条原地不动，离开它后才移到顶部 |
| 关闭当前项后 | 切到其余片段中最近活动的那条（列表顶部） |
| 新建 | 新片段出现在顶部 |
| 与切换器 | ⌘\` 切换器的顺序与侧栏相同（当前项置顶，默认选第 2 项） |
| 不再需要 | tab 拖动排序 |

设计画布中 38 张画板仍是顶部 tab 栏，需要按此更新（至少主屏幕）。

（P0，部分 **验证**）

`editor.State` 配置（**定**）：

| 选项 | 取值 | 原因 |
| --- | --- | --- |
| 语言 | 不设语言（纯文本，无高亮） | 片段不是代码文件 |
| `NoAutoClose` | `true` | 不自动补全括号和引号，保证输入即所得 |
| `NoFolding` | `true` | 纯文本不需要折叠 |
| `LineNumbers` | 按设计显示行号 | — |
| `SoftWrap` | `true` | 长命令与 JSON 不出现横向滚动 |
| `Font` / `LineHeight` | JetBrains Mono 13.5 / 22（来自 `tokens.json`），`Features: calt=0,liga=0` | 等宽字体；中文回退到系统字体。关闭连字：JetBrains Mono 默认把 `->`、`!=`、`==` 画成一个字形，与「输入即所得」不符；列表、预览、差异视图里的等宽文字同样关闭 |
| `Placeholder` | 「直接粘贴或输入，无需命名，会自动留存」 | 设计「新建空白片段」画板 |
| `Bindings` | ⌘K / Ctrl+K 绑定到 `editor.NoCommand` | 编辑器默认把 ⌘K 作为组合键前缀（如 ⌘K ⌘0 折叠），必须让给应用的「全部片段与搜索」 |

文本保真：

- **行尾**：缓冲区记住每一行原本的行尾（LF、CRLF、CR，混合也保留），`WriteTo` 按原样写出。片段从库中打开时原样恢复，状态栏显示主要行尾（`Buffer.LineEnding`）。
- **粘贴的行尾**：编辑器规定粘贴进来的换行沿用所在行的行尾，与剪贴板中的写法无关。因此往空白片段粘贴一段 CRLF 文本时，可能被转成 LF。处理办法：在 `OnInput` 中拦截「向空缓冲区粘贴」，改用 `Buffer.Reset` / `Set` 原样载入剪贴板文本（**验证**）。
- **智能替换**：原生 UI 不经过系统的文本替换（智能引号、破折号、自动纠正），这一点优于 WebView（**验证**确认）。
- **自动缩进**：按 Enter 时编辑器保持上一行的缩进，这只影响键入，不影响粘贴；与 Sublime 行为一致。
- **Tab 键**：插入制表符，不转空格（`Indent` 设为制表符）。
- **输入法**：编辑器内置输入法组合输入支持；技术验证覆盖 macOS 拼音、Windows 微软拼音与搜狗。

### 6.3 主题与深色模式（P0）

- `scripts/gentokens` 把 `design/tokens.json` 生成为 `internal/theme/tokens_gen.go`：

  ```go
  var Light = Palette{Bg: ui.Hex("#FFFFFF"), Text: ui.Hex("#1C1C1A"), /* … */}
  var Dark  = Palette{Bg: ui.Hex("#1B1B1A"), Text: ui.Hex("#ECECE8"), /* … */}
  ```

  视图只用 `Palette` 的字段，不写死颜色（设计约定）。
- 每帧按 `mygo.Theme.IsDark()` 选择 `Light` 或 `Dark`，再以 `ui.LightTheme()` / `ui.DarkTheme()` 为基础，用 `WithAccent` 设置强调色，并覆盖 `Radius`、`Spacing`、`FontSize`、`Inverse` / `InverseText`（轻提示与工具提示），然后 `c.SetTheme`。
- 强调色派生色（`onAccent`、`sel`、`selLine`）按 `tokens.json` 的规则计算，深色下使用 `darkLift` 映射。
- 系统外观变化时，帧会自动跟随；「跟随系统」之外的选择用 `Theme.SetSource` 固定。
- **不白屏**：原生 UI 窗口在第一帧绘制后才显示，并在 `App.Run` 之前设置好外观，所以启动与新建不会闪白（**验证**）。
- 圆角、间距按设计：窗口与面板 12，控件 8，小元素 4；间距在 4px 网格上（主题 `Spacing = 4`）。

### 6.4 多语言（P0）

- `mygo.json` 中配置 `"i18n": { "dir": "locales", "source": "en" }`。消息目录为 `locales/en.json`（源语言，含 `@` 描述）与 `locales/zh-Hans.json`，采用 ARB 格式与 ICU MessageFormat。
- `go tool mygo dev` / `build` 生成 `locales/catalog_gen.go`，每条消息一个带类型的 Go 函数（如 `locales.SnippetsCount(n)`），需要提交到仓库。MyGo 自己的控件与菜单文字也跟随应用语言。
- 跟随系统语言；不在支持范围内时用英文。设置中可手动切换，即时生效。
- 片段正文、待办内容、自动标题都不翻译。
- **相对时间与日期**：`i18n.FormatRelativeTime`（如「2 分钟前 / 2 min ago」）与 `i18n.FormatDate`，并用 `App.Region().FormatLocale(...)` 尊重用户的 12/24 小时制等区域设置。
- **中英混排间距**：MyGo 的文本排版没有 `text-autospace`，界面文字又散在约两百处调用里，渲染时统一处理不现实。改为由 `scripts/autospace` 规范 `locales/zh-Hans.json`：汉字与拉丁字母、数字、按键符号或简单占位符（如 `{mod}`）相邻时，用细空格（U+2009）替换原有的一个普通空格，或补上一个。新增文案照常书写（写不写空格都行），提交前运行 `go run ./scripts/autospace`；CI 用 `-check` 检查。片段正文、标题等用户内容不做处理。复数块内部的边界（如「及其 {count, plural, …}」）工具不改，保持原样。
- 计数与时间用等宽数字（**验证** MyGo 文本是否支持 tabular-nums 字体特性，不支持时对数字列使用等宽字体）。
- 快捷键显示：macOS 用 ⌘ ⌥ ⇧ ⌃ 符号，Windows 用 Ctrl、Alt、Shift 文字，由 `internal/ui/keys` 统一格式化。
- CI 运行 `go tool mygo i18n check`，检查各语言消息齐全。

### 6.5 无障碍与动效

- MyGo 的原生 UI 会向屏幕阅读器公开元素：左侧栏用纵向 tablist / tab 角色；切换器与搜索用组合框 + 列表；图标按钮设置 `Label`；复制成功、留存失败等用播报（对应设计中的 polite 播报）。
- 只在悬停时出现的控件，用 `IgnoreInput(!hovered)` + 透明保持构建，键盘聚焦与辅助技术仍能到达（设计约定）。
- 文字对比度满足 WCAG AA（设计变量已按此取值）；系统开启「增强对比度」时，主题自动加深边框与次要文字。
- 动效按 `tokens.json` 的 `motion` 时长实现，用 MyGo 的弹簧、过渡与变换；系统开启「减弱动态效果」时，`Animate` 直接到达目标，只保留约 150ms 的淡入淡出。
- 按住 ⌘ 显示快捷键：读取 `c.Modifiers()`，按住 0.5 秒后显示；切换器打开时不显示。

## 7. 快捷键

### 7.1 快捷键表与实现方式

| 操作 | macOS | Windows | 范围 | 实现 |
| --- | --- | --- | --- | --- |
| 新建文本片段 | ⌘N | Ctrl+N | 应用内 | 菜单项 `Accelerator: "CmdOrCtrl+N"` |
| 新建待办清单 | ⌘T | Ctrl+T | 应用内 | 菜单项 `CmdOrCtrl+T` |
| 呼出 Lastab 并新建 | ⌃⌥N | Ctrl+Alt+N | 全局（**待确认**） | `mygo.GlobalShortcut.Register("Ctrl+Alt+N", …)`（macOS 上 Ctrl+Alt 即 ⌃⌥） |
| 「最近」切换器 | ⌘\` | Ctrl+\` | 应用内 | 菜单项（macOS，见 7.2）/ 视图中的 `c.Shortcut` |
| 显示 / 隐藏侧栏 | ⌘\\ | Ctrl+\\ | 应用内 | 菜单项「显示」/ 视图中的 `c.Shortcut` |
| 全部片段与搜索 | ⌘K | Ctrl+K | 应用内 | 菜单项；编辑器中解除 ⌘K 绑定 |
| 复制全部 | ⌘⇧C | Ctrl+Shift+C | 应用内 | 菜单项 |
| 版本历史 | ⌘Y | Ctrl+Shift+Y | 应用内 | 菜单项（Windows 上 Ctrl+Y 留给编辑器的「重做」） |
| 关闭当前片段 | ⌘W | Ctrl+W | 应用内 | 菜单项（替代 `RoleClose`，关闭 tab 而不是窗口） |
| 待办：完成 / 排序 / 删除 | ⌘↵ / ⌥↑↓ / ⌘⇧⌫ | Ctrl+Enter / Alt+↑↓ / Ctrl+Shift+Backspace | 待办内 | 待办视图中的 `Shortcut` |
| 显示快捷键提示 | 按住 ⌘ 约 0.5 秒 | 按住 Ctrl | 应用内 | `c.Modifiers()` + 计时 |
| 显示具体时间 | 按住 ⌥ | 按住 Alt | 列表 | `c.Modifiers()` |

- 快捷键默认表集中定义在 `internal/ui/keys`，使用 MyGo 的加速键语法（`CmdOrCtrl+N`）。菜单、设置页、提示浮层都读这一张表。
- 菜单项的 `Accelerator` 同时让 macOS 菜单显示快捷键，并在编辑器有焦点时优先生效。
- 自定义快捷键存入 `setting.shortcuts`（只存与默认不同的项），修改后立即生效：菜单按新表重建；窗口内快捷键每帧按当前表注册；编辑器把这些组合键绑定为 `NoCommand`；全局热键 `Unregister` / `Register`，被占用而失败时保留原来的并在设置页提示。
- **录制**：录制期间暂停菜单加速键与全局热键（否则 macOS 菜单会先拿走 ⌘N 之类的按键），窗口内快捷键也不注册，按键全部交给录制框；esc 取消。新组合键必须含 ⌘ / ⌃ / ⌥（Windows 为 Ctrl / Alt），与其他操作（含固定的 ⌘,）冲突时不生效并写明冲突对象。主修饰键记为 `CmdOrCtrl`，表在两个平台之间可通用。

### 7.2 macOS 的 ⌘\`（**验证**）

⌘\` 在 macOS 上通常由应用的「窗口」菜单处理（在同一应用的窗口之间切换）。方案：

1. 自定义应用菜单：「窗口」菜单只放最小化、缩放等角色，不放会占用 ⌘\` 的项；加入「最近片段」菜单项，`Accelerator: "Cmd+\`"`。
2. 技术验证：在中文、英文、日文等键盘布局下，确认 ⌘\` 能被应用接管。
3. 若无法可靠接管，默认值改为 ⌃\`（需求已预留这个备选），设置页说明原因。

## 8. Agent 访问（P1，范围 **待确认**）

### 8.1 命令行 `lastab`

只读（**待确认**，需求待确认问题 3）。所有子命令都输出 JSON，字段名固定为英文。

```
lastab list   [--limit 20] [--kind text|todo] [--since 1d] [--open]
lastab search <query> [--limit 20] [--since 1d] [--until 2026-11-18] [--kind …]
lastab read   <id 或唯一前缀>
lastab status                     # 访问开关、库版本、条数
lastab version
```

`--json` 为默认输出；`--client <name>` 可选，用于在访问记录中标明来源（Skill 中写明 `--client claude-code`）。没有指定时，尝试从环境变量推断，比如 Claude Code 注入的变量（**验证**），否则记为 `unknown`。

输出示例（`lastab read 2f9c`）：

```json
{
  "id": "01JD2F9C8Q4S1X7M3N5B6V8K0P",
  "kind": "text",
  "title": "curl · POST /v2/orders/search",
  "created_at": "2026-11-18T06:32:10Z",
  "modified_at": "2026-11-18T10:05:42Z",
  "open": true,
  "redacted": true,
  "body": "curl -X POST 'https://api.staging.example.com/v2/orders/search' \\\n  -H 'Authorization: Bearer ••••••••' \\\n  ..."
}
```

待办另带 `"items": [{"text": "…", "done": false}]` 与 `"progress": {"done": 3, "total": 7}`。

错误也是 JSON，并有固定的退出码：

| 退出码 | 含义 |
| --- | --- |
| 0 | 成功 |
| 2 | 参数错误 |
| 3 | 用户关闭了「允许 Agent 访问」 |
| 4 | 未找到 / ID 前缀不唯一 |
| 5 | 数据库不存在或版本过新 |

### 8.2 访问控制与遮蔽

- 命令行每次运行都先读 `setting.agent.access`；关闭时返回退出码 3，不输出任何内容。默认值**待确认**（需求待确认问题 4）。
- 正文默认经过 `redact`。是否允许输出原文，由设置项 `agent.allowRaw` 决定，命令行不提供绕过参数。
- 每次 `list` / `search` / `read` 都写入 `agent_log.db`：时间、来源、操作与参数、涉及的片段 ID。设置页「访问记录」读取这里的数据。

### 8.3 分发与安装（只在用户点击后写入 PATH 与 Skill）

`lastab` 作为 MyGo 的[辅助可执行文件](https://github.com/egoist/mygo/blob/main/docs/distribution.md)随应用分发：`scripts/build` 先把它交叉编译到 `resources/<平台>/bin/`（`-ldflags -X main.version=<mygo.json 的 version>`），再调用 `mygo build`。`mygo build` 会和应用一起为它签名；打 macOS Universal 包时，arm64 与 amd64 两份自动合并为通用二进制（已验证：`lipo -info` 为 `arm64 x86_64`，`Lastab.app/Contents/Resources/bin/lastab version` 可运行）。

| 项 | macOS | Windows |
| --- | --- | --- |
| CLI 在应用中的位置 | `Lastab.app/Contents/Resources/bin/lastab` | `%LOCALAPPDATA%\Programs\Lastab\bin\lastab.exe` |
| 「安装命令行」 | 在 `/usr/local/bin/lastab` 建符号链接；不可写时退回 `~/.local/bin/lastab`，并提示加入 PATH，另给「安装到 /usr/local/bin…」按钮，点了才弹系统管理员授权框 | 把 `%LOCALAPPDATA%\Programs\Lastab\bin` 加入用户 PATH（`HKCU\Environment` 的 `Path`，保留原有的 `REG_SZ` / `REG_EXPAND_SZ` 类型），并广播 `WM_SETTINGCHANGE`；提示新开的终端才生效 |
| Skill | 写入 `~/.claude/skills/lastab/SKILL.md` | 写入 `%USERPROFILE%\.claude\skills\lastab\SKILL.md` |

实现（**已做**）：

- 代码：`internal/install`（纯 Go，`Env{Home, CLI, GlobalBin}` 可指向临时目录测试）；`internal/shell` 用它实现 `appui.Platform` 新增的 `CLIStatus` / `SkillStatus` / `InstallCLI(admin)` / `InstallSkill`；界面在 `internal/ui/agent.go`，视图测试用假 Platform，覆盖「点击前不写入」、安装中、失败与重试、Skill 过期重装、`~/.local/bin` 退回与管理员安装。
- 应用内 CLI 的位置：`mygo.App.Path(PathResources)/bin/lastab`；`go run` / `mygo dev` 下退回 `resources/<平台>/bin/lastab`（先跑 `go run ./scripts/build -cli-only`）。
- 符号链接 / PATH 指向应用内的 CLI，应用更新后 CLI 自动跟着更新，不需要再复制。链接先建在同目录的临时名再改名覆盖，旧链接不会中途消失。
- 只替换指向某个 Lastab 的链接（指向本应用，或路径以 `.app/Contents/Resources/bin/lastab` 结尾）；同名的普通文件或指向别处的链接一律不动，报「已存在且不是 Lastab 的」。指向另一份 Lastab（如移动过的应用）的链接显示为「需重新安装」。
- 应用在磁盘映像里或被 App Translocation 临时搬运时（路径在 `/Volumes/` 或含 `/AppTranslocation/`），拒绝安装并提示先移到「应用程序」，否则链接会失效。
- 管理员安装用 `osascript` 的 `do shell script … with administrator privileges`，只在用户点「安装到 /usr/local/bin…」时调用；用户取消授权（-128）不算失败，不显示错误。
- 失败留在原处显示原因和「重试」（对应设计 AgentFailed 画板）：没有权限、文件已存在、应用内没有 CLI、应用未安装到「应用程序」，其余显示系统错误原文。安装中按钮显示转圈与「正在安装…」（AgentInstalling 画板）。
- 设计稿中 Windows 的 CLI 路径 `%LOCALAPPDATA%\Lastab\bin\lastab.exe` 已按实际安装位置显示为 `%LOCALAPPDATA%\Programs\Lastab\bin\lastab.exe`。
- `SKILL.md` 说明何时调用（用户提到「我之前改的 / 记的…」）、命令用法、输出字段，并要求 Agent 在用户要求时才读取正文。
- Skill 版本：`SKILL.md` 中的 `<!-- lastab-skill-version: N -->`。已安装文件的版本低于应用自带的（或没有版本注释，比如用户自己改写过的），设置页提示「这个版本的 Lastab 附带了更新的 Skill」并给「重新安装」，不静默覆盖。**改动 `SKILL.md` 时必须把 N 加一。**
- 卸载 Windows 版时从用户 PATH 移除 `$INSTDIR\bin`：`mygo.json` 的 `windows.installerInclude` 指向 `scripts/build/installer.nsh`，其 `customUnInstall` 读 `HKCU\Environment` 的 `Path`，去掉与该目录相同的条目（原样或展开变量后比较，不分大小写，忽略末尾反斜杠），写回为 `REG_EXPAND_SZ`（空了就删除该值），并广播 `WM_SETTINGCHANGE`。PATH 长到接近 NSIS 字符串上限（`NSIS_MAX_STRLEN`，常见为 1024）时可能读到截断的值，此时不改动，只在卸载日志里说明。应用自动更新不经过卸载程序，PATH 保持不变。
- 待确认：`~/.local/bin` 退回方案是否接受。

## 9. 平台差异

| 项 | macOS | Windows |
| --- | --- | --- |
| 最低系统版本 | macOS 12（MyGo 的下限；**待确认**是否提高） | Windows 10 / 11，x64 与 arm64 |
| 运行依赖 | 无（Metal 绘制） | 无（Direct3D 11 绘制，不需要 WebView2） |
| 窗口外观 | `TitleBarHidden`，保留红黄绿按钮，压在左侧栏顶部（侧栏标题行用 `c.TitleBar()` 让位） | `TitleBarHidden`，系统窗口按钮在编辑区标题栏右上角 |
| 常驻 | 关闭窗口后应用保留；Dock 图标 + 可选托盘图标 | 关闭窗口后隐藏到托盘 |
| 快捷键显示 | ⌘ ⌥ ⇧ ⌃ 符号 | Ctrl、Alt、Shift 文字 |
| 版本历史 | ⌘Y | Ctrl+Shift+Y（Ctrl+Y 是「重做」） |
| 全局热键实现 | Carbon 热键（不需要辅助功能权限） | `RegisterHotKey` |
| 登录时启动 | 登录项（macOS 13+）/ 启动代理（macOS 12） | 用户 Run 条目 |
| 字体回退 | PingFang SC | Microsoft YaHei |
| 已知风险 | ⌘\` 与系统冲突 | 中文输入法候选窗位置（**验证**） |

## 10. 构建、打包与发布

### 10.1 本地开发

```bash
go tool mygo doctor                    # 检查环境
go run ./scripts/build -cli-only       # 为本机平台构建 lastab 到 resources/<平台>/bin
go tool mygo dev                       # 构建并启动「Lastab Dev」，改 .go 文件后自动重启
go test ./...                          # 全部测试，含无窗口的视图测试
go run ./scripts/gentokens             # tokens.json 变化后重新生成颜色
go tool mygo generate                  # 重新生成 locales/catalog_gen.go
```

### 10.2 打包与签名

```bash
go run ./scripts/build                                        # 本机平台：Lastab.app + DMG（ad hoc 签名）
go run ./scripts/build -platform darwin/universal             # 在 Mac 上：通用 .app + DMG，有证书时签名并公证
go run ./scripts/build -platform windows/amd64,windows/arm64  # 任意机器：Lastab.exe + NSIS 安装包（需 makensis，没有时跳过安装包）
go run ./scripts/genicon                                      # 重新生成占位图标
```

`scripts/build` 依次：交叉编译 `lastab` 到对应的 `resources/<平台>/bin/` → 按环境变量临时把签名设置写进 `mygo.json` → 运行 `go tool mygo build -platform …` → 无论成败（含 Ctrl+C）都把 `mygo.json` 恢复原样。产物在 `build/<平台>/`，不提交。

`mygo.json` 只放不含秘密的设置：`macos.minimumSystemVersion: "12.0"`、`windows.installDirName: "Lastab"`（NSIS 安装包默认按用户安装到 `%LOCALAPPDATA%\Programs\Lastab`，不需要管理员权限）、`windows.installerInclude: "scripts/build/installer.nsh"`（卸载时移除 PATH 条目，见 8.3）、`resourcesExclude: ["fonts"]`。在 Mac 上打 Windows 安装包需 `brew install makensis`；Homebrew 的 makensis 在 `LANG` 为空时会崩溃（`std::bad_alloc`），`scripts/build` 此时自动设 `LANG=C.UTF-8`。`mygo.json` 是 JSON，不能读环境变量（只有 `mygo.config.ts` 可以，但那需要 Node/Bun），所以签名设置由 `scripts/build` 注入：

| 环境变量 | 写入 `mygo.json` 的 | 没有时 |
| --- | --- | --- |
| `MACOS_SIGNING_IDENTITY` | `macos.signingIdentity`（如 `Developer ID Application: … (TEAMID)`） | ad hoc 签名，其他 Mac 上 Gatekeeper 会拦 |
| `NOTARY_KEYCHAIN_PROFILE`、`NOTARY_KEYCHAIN` | `macos.notarize.keychainProfile` / `.keychain`（`xcrun notarytool store-credentials` 存的凭据；只在有签名身份时生效） | 不公证 |
| `WINDOWS_CERTIFICATE_FILE` | `windows.certificate`（`.pfx` 路径；密码由 mygo 直接读 `MYGO_WINDOWS_CERTIFICATE_PASSWORD`） | 不签名，SmartScreen 会警告 |
| `WINDOWS_SIGN_COMMAND` | `windows.signCommand`（如 Azure Trusted Signing，`%1` 为文件） | 同上 |

| 平台 | 产物与签名 |
| --- | --- |
| macOS | `Lastab.app` 与 `lastab-<版本>.dmg`（单架构为 `lastab-<版本>-<架构>.dmg`）。有签名身份时 `mygo build` 签名（含资源中的 `lastab`）、启用强化运行时；再有公证凭据时公证 app 与 DMG 并附加票据 |
| Windows | `Lastab.exe`、`bin\lastab.exe` 与 `lastab-setup-<版本>-<架构>.exe`（NSIS，按用户安装）。有证书时签名应用、`lastab.exe`、安装包与卸载程序 |

- 签名证书与公证凭据只放在 CI Secrets 与钥匙串中，不进仓库。
- 图标：`resources/icon.png`（1024×1024，`scripts/genicon` 画的占位图，蓝色）与 `resources/icon-dev.png`（橙色，`mygo dev` 用）。正式图标出来后直接替换这两个文件。`icon.png` 会随应用进入资源目录（MyGo 的更新插件会读它），不排除。
- 版本号：`mygo.json` 的 `version`；应用与 CLI 同一个版本号（构建时用 `-ldflags` 写入 CLI），`lastab version` 可查。
- 自动更新：MyGo 内置签名更新（`mygo.Updater`，支持 GitHub Releases 与增量更新）。需求未提，首期不开启，**待确认**；开启后 `release.yml` 改用 `mygo build -upload`。

### 10.3 CI（GitHub Actions）

- PR（已建，`.github/workflows/ci.yml`）：`CGO_ENABLED=0`；macOS 与 Windows 两个 runner 跑 `go vet`、`go test ./...`、`mygo i18n check` 和构建；macOS 上另查 `gofmt`、Windows / Linux 交叉 `vet`，以及 `tokens_gen.go`、`catalog_gen.go` 与源文件一致。另有 `lint` 作业：`staticcheck`、`golangci-lint`（配置见 `.golangci.yml`）与 `govulncheck`，均锁定确切版本，用项目的 Go 构建；前两者另按 `GOOS=windows` 再查一遍。
- `main`：额外构建未签名的产物，供内部试用（未做）。
- Tag（已建，`.github/workflows/release.yml`，参考 MyGo 文档的 GitHub Actions 一节）：推送 `v*` 标签触发。
  1. `draft-release`：检查标签与 `mygo.json` 的 `version` 一致（`v0.1.0` 对 `0.1.0`），不一致就失败、什么都不构建；然后创建草稿 Release（已有则复用）。
  2. `macos`（macos-latest）：有证书时导入到临时钥匙串、从证书中取出 `Developer ID Application` 身份写入 `MACOS_SIGNING_IDENTITY`；再有 Apple ID 凭据时存为 notarytool 配置 `notary`，设置 `NOTARY_KEYCHAIN_PROFILE` / `NOTARY_KEYCHAIN`。然后 `go run ./scripts/build -platform darwin/universal`，检查通用 `lastab` 可运行、`codesign --verify`，把 DMG 传到草稿。
  3. `windows`（ubuntu-latest，交叉编译）：装 `nsis` 与 `osslsigncode`；有证书时写出 `.pfx` 并设置 `WINDOWS_CERTIFICATE_FILE`；`go run ./scripts/build -platform windows/amd64,windows/arm64`，把两个安装包传到草稿。
  4. 人工检查草稿后发布。
- 没有任何签名 Secret 时工作流照常成功，只在日志里给出 warning：macOS 为 ad hoc 签名、不公证；Windows 不签名。

需要在仓库 Secrets 中配置（都可选，缺哪组就跳过哪一步）：

| Secret | 内容 | 用于 |
| --- | --- | --- |
| `MACOS_CERTIFICATE` | `Developer ID Application` 证书连私钥导出的 `.p12`，base64（`base64 -i cert.p12 \| gh secret set MACOS_CERTIFICATE`） | macOS 签名 |
| `MACOS_CERTIFICATE_PASSWORD` | `.p12` 的密码 | macOS 签名 |
| `APPLE_ID` | 开发者账号的 Apple ID | 公证 |
| `APPLE_TEAM_ID` | Team ID（身份名括号里的那串） | 公证 |
| `APPLE_APP_PASSWORD` | Apple ID 的 App 专用密码（account.apple.com → 登录与安全） | 公证 |
| `WINDOWS_CERTIFICATE` | 代码签名 `.pfx`，base64 | Windows 签名 |
| `WINDOWS_CERTIFICATE_PASSWORD` | `.pfx` 的密码（作为 `MYGO_WINDOWS_CERTIFICATE_PASSWORD` 传给 mygo） | Windows 签名 |

硬件令牌或 Azure Trusted Signing 这类证书需要 `signtool`：把 `windows` 作业改到 `windows-latest`，设置 `WINDOWS_SIGN_COMMAND`。

## 11. 测试与验收

### 11.1 自动化测试

| 层 | 覆盖内容 |
| --- | --- |
| `internal/title` | 各类内容的标题规则、遮蔽、截断；表驱动测试 |
| `internal/redact` | 正例与反例样本库（避免误伤普通文本） |
| `internal/search` | 中文 2 字与 3 字以上、英文、代码符号（`{"`、`--since`、`/v2/`）、已关闭片段、时间过滤 |
| `internal/history` | 快照去重、改动行数、恢复与撤销 |
| `internal/todo` | 解析与序列化往返一致 |
| `internal/store` | 迁移；并发：应用持续写入时 CLI 只读不报错、不阻塞；异常中断后数据完整 |
| `cmd/lastab` | JSON 字段与退出码的黄金文件测试；访问关闭时无输出 |
| 视图（`ui.NewTester`） | 新建 → 输入 → 复制全部；切换器 Tab / Shift+Tab / Enter / Esc；⌘K 搜索打开已关闭片段；待办键盘操作；深色与英文下的文字；无障碍名称 |
| 文本保真 | 含智能引号、破折号、Tab、行尾空格、全角字符、emoji、CRLF 与混合行尾的样本：载入 → 编辑 → `WriteTo` / 复制全部，逐字节比对 |
| 端到端（`-tags e2e`） | 用自动化插件驱动构建好的应用：全局热键呼出、菜单快捷键、窗口隐藏与恢复、截图对比设计关键画板 |

### 11.2 验收对照

需求「验收行为」表中的每一行，都在 `docs/acceptance.md`（开工后建立）中对应到一个自动化测试或手工检查步骤。其中两项需要手工对比：

- **新建速度**：在同一台机器上，按下快捷键到光标可输入，与 Sublime Text 的 ⌘N / Ctrl+N 对比（录屏逐帧，或在应用内打点记录）。
- **找回对照**：开约 20 个片段，在 Sublime 与 Lastab 中分别找回 3 分钟前编辑的那条，记录用时。

## 12. 开工前的技术验证

以下事项依赖框架或平台行为，建议在 M0 用一个最小原型逐项验证，结论回写本文。

| # | 验证项 | 通过标准 | 不通过时的备选 |
| --- | --- | --- | --- |
| 1 | 常驻后台时，从全局热键 / ⌘N 到可输入的耗时 | 不慢于 Sublime | 预先构建空白 tab 的编辑器状态；精简首帧 |
| 2 | macOS ⌘\` 能否由应用菜单接管 | 多种键盘布局下都可以 | 默认改为 ⌃\` |
| 3 | 编辑器行尾保真：载入、编辑、`WriteTo`、选区复制、向空缓冲区粘贴 CRLF | 逐字节一致 | 在 `OnInput` 中拦截粘贴并原样载入 |
| 4 | 编辑器不做任何智能替换（引号、破折号、自动大写） | 输入与粘贴都原样保留 | 通过 `OnInput` 过滤；向 MyGo 反馈 |
| 5 | 中文输入法（macOS 拼音、搜狗、微软拼音）在编辑器、`TextInput`、切换器过滤框中组合输入 | 无丢字、无重复，候选窗位置正确 | 向 MyGo 反馈；锁定可用版本 |
| 6 | `modernc.org/sqlite` 的 FTS5 trigram 与只读 WAL 打开 | 可用；应用写入时 CLI 读取无错误 | 换 `ncruces/go-sqlite3`（Wasm，仍无 cgo） |
| 7 | 1 万条片段下的搜索延迟（含短查询全表回退） | 输入即搜体验流畅（目标 < 50ms） | 短查询加一张 bigram 辅助表 |
| 8 | 切换器浮层接管 Tab（不移动焦点）、输入过滤框与列表联动 | 行为符合 5.6 | 用 `HandleInput` 自建输入框 |
| 9 | 运行时切换界面语言、外观即时生效 | 无需重启 | 切换后重建主窗口视图 |
| 10 | 中英混排细空格、等宽数字的显示效果 | 与设计稿一致 | 调整插入规则；数字改用等宽字体 |
| 11 | Claude Code 调用 Skill → CLI 的完整链路 | 能搜索并读到片段，访问记录正确 | — |

### 12.1 M0 进展（2026-11-18）

骨架已按第 3 节建好（`go.mod` 锁定 MyGo v0.10.0），业务层、命令行与 M1 主体界面可运行。已有结论：

| # | 结论 | 依据 |
| --- | --- | --- |
| 3 | 部分通过：CRLF 文本粘贴进空白片段、复制全部，逐字节一致（在 `OnInput` 拦截 `paste` 命令，用 `Buffer.Set` 原样载入） | `internal/ui` 视图测试（`ui.NewTester`）；真实窗口与选区复制待手工确认 |
| 6 | 通过：`modernc.org/sqlite` 支持 FTS5 trigram；应用持续写入时，只读打开的 CLI 搜索无错误、不阻塞 | `internal/store` 测试；真实应用运行时 `lastab status` / `list` 正常 |
| 7 | 通过（数据层，Apple M4）：1 万条片段下 `Store.Search` 全部 < 35 ms，⌘K 每次按键的查询（含预览、版本数、总数）≤ 32 ms。短查询暂不需要 bigram 辅助表 | 下文 12.1.1；`internal/store`、`internal/search` 的基准测试 |
| 8 | 通过（无窗口）：浮层中过滤框的 `Element.Shortcut` 能接住 Tab / Shift+Tab / ↑↓ / ⌘\`，Enter 由 `Submitted` 处理 | `TestSwitcher` |

开发环境补充：本机 Command Line Tools 的 SDK 与链接器不匹配时，`go test` 默认启用 cgo 会链接失败。项目不用 cgo，统一用 `CGO_ENABLED=0` 运行 `go test` / `go build`（`scripts/build` 已设置）。

其余验证项（1、2、4、5、9、10、11）需要真实窗口、输入法或多种键盘布局，仍待手工验证，步骤见 `docs/acceptance.md`。

#### 12.1.1 验证项 7：1 万条片段的搜索延迟（2026-11-19）

**方法**：`internal/store/storetest` 用固定种子生成 1 万条片段：中文段落 30%、英文 20%、curl 命令 10%（带 `Authorization`、`--since`、`-d '{"…"}'`）、JSON 12%、代码 13%、日志 7%、待办 8%；长度 60% 在 1 KB 以内，3% 为 12–20 KB；修改时间集中在最近 90 天；20 条在工作区中打开；每条 0–5 个版本。合计正文 24.4 MB、版本 24,929 个，数据库 160 MB（trigram 索引占大头）。在 Apple M4、macOS、Go 1.27.4、`modernc.org/sqlite` v1.62.0 上运行：

```
CGO_ENABLED=0 go test -run '^$' -bench . ./internal/store ./internal/search
```

**`Store.Search`**（`Limit` 300，即 ⌘K 的上限，另注明的除外）：

| 查询 | 路径 | 命中 | 耗时 |
| --- | --- | ---: | ---: |
| `周会` | 短查询扫描 | 300 | 6.6 ms |
| `周会`，不限条数 | 短查询扫描 | 1,656 | 34.4 ms |
| `发版` | 短查询扫描 | 300 | 2.8 ms |
| `{"` | 短查询扫描 | 300 | 4.0 ms |
| `鲸鱼`（无命中，扫完全表，最坏情况） | 短查询扫描 | 0 | 32.0 ms |
| `鲸鱼` + 30 天 | 短查询扫描 | 0 | 12.7 ms |
| `周会` + 7 天 | 短查询扫描 | 181 | 3.0 ms |
| `发版` + 只看已打开 | 短查询扫描 | 11 | 0.09 ms（改前 31 ms） |
| `发版` + 只看已关闭 | 短查询扫描 | 300 | 3.2 ms |
| `数据库` | trigram | 300 | 17.8 ms |
| `--since` | trigram | 300 | 13.2 ms |
| `{"id` | trigram | 300 | 10.9 ms |
| `/v2/` | trigram | 300 | 14.9 ms |
| `Authorization` | trigram | 300 | 15.8 ms |
| `certificate`（只出现在部分待办里） | trigram | 300 | 4.2 ms |
| `zebra crossing`（无命中） | trigram | 0 | 0.13 ms |
| `the`（常见词） | trigram | 300 | 15.8 ms |
| `the`，不限条数 | trigram | 2,662 | 21.5 ms |
| `the` + 7 天 | trigram | 299 | 4.4 ms |
| `the` + 30 天 + 只看已关闭 | trigram | 300 | 8.2 ms |
| `the` + 只看已打开 | trigram | 5 | 2.5 ms |
| `the` + 待办 | trigram | 300 | 6.7 ms |
| `数据库` + 7 天 | trigram | 291 | 5.6 ms |

| 其他调用 | 耗时 |
| --- | ---: |
| `List`，300 条（空查询的 ⌘K） | 0.53 ms |
| `List`，300 条 + 7 天 / 只看已关闭 | 0.55 / 0.58 ms |
| `List`，全部 1 万条 | 22.0 ms |
| `VersionCounts`，300 个 ID | 0.29 ms |
| `Count` | 0.005 ms |

**⌘K 一次按键的完整查询**（`search.Searcher.Query` 含预览与遮蔽，加 `VersionCounts` 与 `Count`，即界面在后台 goroutine 中做的全部工作）：

| 查询 | 改前 | 改后 |
| --- | ---: | ---: |
| 空查询 | 107 ms | 5.0 ms |
| `周会` | 72 ms | 13.8 ms |
| `鲸鱼`（无命中） | 39 ms | 32.1 ms |
| `the` | 140 ms | 24.9 ms |
| `数据库` | 109 ms | 25.5 ms |
| `--since` | 210 ms | 22.4 ms |
| `the` + 7 天 | 121 ms | 13.4 ms |

**改动**：

- 慢的不是 SQLite，是预览：`search.Preview` 对每条结果的整段正文跑一遍 `redact`（13 条正则），再只取 160 字节。现在只遮蔽命中位置前后若干整行的窗口（遮蔽规则都不跨行，窗口外再多留两行以防键与值之间的空白跨行），含 PEM 私钥的正文仍整段遮蔽；`TestPreviewWindow` 用生成的片段与边界样本逐条比对，与整段遮蔽的结果一致。
- `redact` 先查每条规则必含的字面量（如 `authorization`、`sk-`、`AKIA`），文本里没有就跳过该正则；`(?i)` 规则在折叠大小写后的文本里查（含 K 的开尔文符号、长 s）。`TestPrefilter` 比对与逐条跑正则的结果一致。
- 短查询（< 3 个字符）不含 ASCII 字母时（`周会`、`{"`）用 `instr()` 代替 `LIKE`：SQLite 的 `LIKE` 只对 ASCII 忽略大小写，两者结果相同，全表扫描快约 20%。
- 「只看已打开」改为 `s.id IN (SELECT snippet_id FROM workspace_tab)`，让 SQLite 从约 20 个打开的 tab 出发，而不是逐条扫描再判断。

**结论**：

- 目标（< 50 ms）在这台机器上达到。最慢的是不能提前停下的短查询扫描（无命中或不限条数，约 32–34 ms）：它要读完 24 MB 正文，余量约三分之一。**bigram 辅助表暂不加**；在较慢的 Windows 笔记本上复测这两项，超过 50 ms 再加（短查询走 bigram 表，3 个字符以上仍走 trigram）。
- ⌘K 的 300 条上限对数据层没有问题：300 条的预览、版本数与总数合计几毫秒。
- 不做虚拟化的列表是现在每次按键最大的开销。用 `ui.NewTester` 在同一份数据上临时测量（未提交）：结果变化后的第一帧要重新排版 300 行，约 30–95 ms，比查询本身还慢；结果不变的帧 2–3 ms，↓ 键加一帧约 10 ms。建议界面先只渲染前 50–100 行，滚动时再补（或做虚拟化），属于界面的后续工作，不阻塞 M0。
- 测量中发现一个界面问题：结果按相关度排序时（3 个字符以上的查询），同一天的结果可能不相邻，`libraryList` 以 `day-<标签>` 作为日期分组标题的 key，同一个标签出现两次时 MyGo 报重复 key 并 panic（`internal/ui/library.go` 的日期分组）。需要界面修复，例如 key 加上序号，或相关度排序时不分组。
- **已处理（2026-11-19）**：
  - ⌘K 结果列表改用 MyGo 的虚拟列表 `ui.List`，只构建可见的行，日期标题用 `ListState.Header` 固定在顶部。同一份 1 万条数据上，一次按键（查询加结果变化后的第一帧，测试里查询同步执行）为 9–30 ms，排版部分从 30–95 ms 降到几毫秒；↓ 键一帧约 4 ms。
  - 结果改为按最近修改时间显示（页面上本来就写着「按最近修改排序」），相关度只决定取回哪 300 条，所以同一天的结果总是相邻，每个日期标题只出现一次。`TestLibraryGroupsByDayWhateverTheRanking` 用同一份语料复现了原来的 panic，并验证修复。

## 13. 里程碑（建议）

| 里程碑 | 内容 | 对应优先级 |
| --- | --- | --- |
| M0 技术验证 | 第 12 节各项；`go tool mygo init -template native` 生成骨架，接入 CI、tokens 生成、i18n | — |
| M1 即时编辑 | 多 tab 编辑器、即时新建、文本保真、自动留存、异常恢复、复制全部、深浅色、中英文 | P0 |
| M2 找回 | 自动标题、「最近」切换器、⌘K 列表与全文搜索、时间显示 | P0 |
| M3 增强 | 版本历史、快捷键自定义、标题手改、遮蔽、时间过滤 | P1 |
| M4 待办与 Agent | 待办清单；`lastab` 命令行、Skill 安装、访问开关与记录 | P1（首期与否**待确认**） |
| M5 发布 | 签名、公证、安装包、验收对照全部通过 | — |

M1、M2 之后即可内部试用，验证「两周后不再用 Sublime 做中转编辑」这一成功标准。

## 14. 影响实现的待确认点

除需求文档中已有的待确认问题外，开发方案新增以下几点：

1. **最低系统版本**：MyGo 支持 macOS 12+ 与 Windows 10/11。是否就按这个下限？
2. **CLI 安装位置**：macOS 默认 `/usr/local/bin` 需要管理员授权；授权失败时退回 `~/.local/bin` 是否可以接受？Windows 的路径按 8.3 改为实际安装位置，设计稿相应更新。
3. **开机自启**：常驻后台才能保证呼出速度。设置中提供「登录时启动」，默认是否开启？
4. **自动更新**：MyGo 内置签名更新与增量更新。首期是否开启？开启的话，需要生成并妥善保管更新签名密钥（`mygo keygen`）。
5. **MyGo 版本策略**：MyGo 仍是 0.x，锁定 v0.10.0 开工；之后多久评估一次升级？
