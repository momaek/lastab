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
├── mygo.json                   # 应用名、identifier、版本、i18n、签名等配置
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
│   ├── shell/                  # 菜单、托盘、全局快捷键、单实例、登录时启动、CLI/Skill 安装
│   └── ui/                     # 原生 UI：视图、组件、快捷键表
│       ├── app.go              # 应用状态与根视图
│       ├── sidebar.go  view.go     # 左侧栏、编辑区标题栏、状态栏
│       ├── editor.go           # ui/editor 的配置与包装
│       ├── todo.go  switcher.go  library.go  history.go  settings.go  agent.go
│       ├── toast.go  confirm.go  # 按设计自绘的轻提示与确认气泡
│       └── keys/               # 快捷键默认表与平台显示（⌘ vs Ctrl）
├── locales/                    # en.json（源语言）、zh-Hans.json；catalog_gen.go（生成，需提交）
├── skill/
│   └── SKILL.md                # 安装到 ~/.claude/skills/lastab/ 的模板
├── resources/
│   ├── icon.png                # 1024×1024 应用图标；icon-dev.png 为开发版图标
│   ├── tray.png                # 托盘图标（macOS 模板图）
│   ├── fonts/                  # 由 //go:embed 使用，不进入资源目录（resourcesExclude）
│   ├── darwin-arm64/bin/lastab           # 由 scripts/build 生成，不提交
│   ├── darwin-amd64/bin/lastab
│   ├── windows-amd64/bin/lastab.exe
│   └── windows-arm64/bin/lastab.exe
└── scripts/
    ├── gentokens/              # design/tokens.json → internal/theme/tokens_gen.go
    └── build/                  # 先交叉编译 lastab 到 resources/<平台>/bin，再调用 mygo build
```

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
- **查询长度 < 3 个字符**（如「周会」「{"」）：trigram 无法命中，回退为 `body LIKE '%q%' OR title LIKE '%q%'` 全表扫描。本地片段量（万条级）下可接受，技术验证中实测。
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
- **编辑**：每项是一个单行 `ui.TextInput`，左侧是 `ui.Checkbox`。Enter 新增一项；⌘Enter / Ctrl+Enter 切换完成；⌥↑↓ / Alt+↑↓ 调整顺序；⌘⇧⌫ / Ctrl+Shift+Backspace 删除此项；空项按 ⌫ 直接删除。只有点勾选框才切换完成状态，点文字是编辑。
- **已完成项**：勾选后原位划线约 0.9 秒，再沉到「已完成」分组（按设计先这样做，**待确认**，需求待确认问题 6）；提供「清除已完成」，并可撤销。
- **粘贴多行**：每行成为一项，同时给出可撤销的轻提示。
- **拖动排序**：用 `ui.List` 的 `ListState.Reorder`（拖动时显示落点线）。设计中的「行缩至 90%、邻项按 8px×0.7ⁿ 让位」用 MyGo 的动效与变换补上，Esc 取消拖动。

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
- **其他**：快捷键自定义（录制新组合键、冲突检测、全部恢复默认）；编辑字体与字号（`ui.FontFamilies()` 中 `Monospaced` 为真的字体，字号 11–20）；登录时启动（`App.SetOpenAtLogin`，默认关闭，**待确认**）；按住 ⌘ 显示快捷键（开关，功能本身尚未实现，开关暂不显示）。
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
| `Font` / `LineHeight` | JetBrains Mono 13.5 / 22（来自 `tokens.json`） | 等宽字体；中文回退到系统字体 |
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
- **中英混排间距**：MyGo 的文本排版没有 `text-autospace`。在界面文字的渲染辅助函数中，为中文与拉丁字母、数字之间自动插入细空格（U+2009）。资源文件里不手打空格；片段正文不做处理。
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

`lastab` 作为 MyGo 的[辅助可执行文件](https://github.com/egoist/mygo/blob/main/docs/distribution.md)随应用分发：`scripts/build` 先把它交叉编译到 `resources/<平台>/bin/`，再调用 `mygo build`。`mygo build` 会和应用一起为它签名；打 macOS Universal 包时，arm64 与 amd64 两份会自动合并为通用二进制。

| 项 | macOS | Windows |
| --- | --- | --- |
| CLI 在应用中的位置 | `Lastab.app/Contents/Resources/bin/lastab` | `%LOCALAPPDATA%\Programs\Lastab\bin\lastab.exe` |
| 「安装命令行」 | 在 `/usr/local/bin/lastab` 建符号链接，需要管理员授权（系统授权对话框） | 把 `%LOCALAPPDATA%\Programs\Lastab\bin` 加入用户 PATH（`HKCU\Environment`），并广播环境变量变更 |
| Skill | 写入 `~/.claude/skills/lastab/SKILL.md` | 写入 `%USERPROFILE%\.claude\skills\lastab\SKILL.md` |

- 符号链接 / PATH 指向应用内的 CLI，应用更新后 CLI 自动跟着更新，不需要再复制。
- 设计稿中 Windows 的 CLI 路径写的是 `%LOCALAPPDATA%\Lastab\bin\lastab.exe`，需要改成实际安装位置 `%LOCALAPPDATA%\Programs\Lastab\bin\lastab.exe`（MyGo 安装包的默认位置）。
- `/usr/local/bin` 在部分 Apple Silicon 机器上不存在或不可写；授权失败时退回 `~/.local/bin`，并提示用户把它加入 PATH（**待确认**是否接受）。
- 安装失败留在原处显示原因和「重试」（对应设计「安装 Skill：失败」画板）。
- `SKILL.md` 说明何时调用（用户提到「我之前改的 / 记的…」）、命令用法、输出字段，并要求 Agent 在用户要求时才读取正文。
- 应用升级后，若 Skill 模板版本变化，在设置页提示「重新安装」，不静默覆盖。

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
go run ./scripts/build -platform darwin/universal          # 在 Mac 上：.app + DMG，签名并公证
go run ./scripts/build -platform windows/amd64,windows/arm64  # 任意机器：Lastab.exe + 安装包
```

`scripts/build` 依次：交叉编译 `lastab` 到对应的 `resources/<平台>/bin/` → 运行 `go tool mygo build -platform …`。

| 平台 | 产物与签名 |
| --- | --- |
| macOS | `Lastab.app` 与 `lastab-<版本>-universal.dmg`。`mygo.json` 中配置 `macos.signingIdentity`（Developer ID）与 `macos.notarize.keychainProfile`，`mygo build` 自动签名（含资源中的 `lastab`）、启用强化运行时、公证并附加票据 |
| Windows | `Lastab.exe` 与 `lastab-setup-<版本>-<架构>.exe`（NSIS）。配置 `windows.certificate`（密码来自 `MYGO_WINDOWS_CERTIFICATE_PASSWORD`）或 `windows.signCommand`（如 Azure Trusted Signing），签名应用、`lastab.exe`、安装包与卸载程序 |

- 签名证书与公证凭据只放在 CI Secrets 与钥匙串中，不进仓库。
- 版本号：`mygo.json` 的 `version`；应用与 CLI 同一个版本号（构建时用 `-ldflags` 写入 CLI），`lastab version` 可查。
- 自动更新：MyGo 内置签名更新（`mygo.Updater`，支持 GitHub Releases 与增量更新）。需求未提，首期不开启，**待确认**。

### 10.3 CI（GitHub Actions）

- PR（已建，`.github/workflows/ci.yml`）：`CGO_ENABLED=0`；macOS 与 Windows 两个 runner 跑 `go vet`、`go test ./...`、`mygo i18n check` 和构建；macOS 上另查 `gofmt`、Windows / Linux 交叉 `vet`，以及 `tokens_gen.go`、`catalog_gen.go` 与源文件一致。`golangci-lint`、`staticcheck`、`govulncheck` 待加（需锁定版本）。
- `main`：额外构建未签名的产物，供内部试用。
- Tag：在 macOS runner 上构建、签名、公证 macOS 包；构建并签名 Windows 安装包；生成 Release 草稿。工作流参考 MyGo 文档中的 GitHub Actions 一节。

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
| 8 | 通过（无窗口）：浮层中过滤框的 `Element.Shortcut` 能接住 Tab / Shift+Tab / ↑↓ / ⌘\`，Enter 由 `Submitted` 处理 | `TestSwitcher` |

开发环境补充：本机 Command Line Tools 的 SDK 与链接器不匹配时，`go test` 默认启用 cgo 会链接失败。项目不用 cgo，统一用 `CGO_ENABLED=0` 运行 `go test` / `go build`（`scripts/build` 已设置）。

其余验证项（1、2、4、5、7、9、10、11）需要真实窗口、输入法或多种键盘布局，仍待手工验证。

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
