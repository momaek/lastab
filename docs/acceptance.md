# Lastab 验收对照

需求说明 v0.5「验收行为」表中的每一行，在这里对应到一个自动化测试或手工检查（开发文档 11.2、开发计划「每个阶段结束」）。每个阶段结束时更新状态与结果表。

**状态**：

- **通过**：有自动化测试覆盖，且在 `CGO_ENABLED=0 go test ./...` 中通过（CI 在 macOS 与 Windows 上都跑）。
- **未做**：功能或测试还没有，写明缺什么、预计哪个阶段补上。
- **手工**：只能在真实窗口、输入法或键盘布局下确认，按下文「手工检查」的步骤做，结果填进对应的结果表。

一行可以同时有自动化测试与手工检查：自动化覆盖逻辑，手工确认真实窗口里的表现。两者都通过，这一行才算通过。

本文最后核对：2026-11-19，合入待办界面（#6）与打包（#7）之后的 `main`。

## 验收行为对照

| # | 行为（需求原文摘要） | 自动化测试 | 手工检查 | 状态 |
| --- | --- | --- | --- | --- |
| A1 | 从按下新建快捷键到可以输入，不慢于同机 Sublime Text 的 ⌘N / Ctrl+N | — | M1 | 手工 |
| A2 | 新建快捷键默认 ⌘N / Ctrl+N，可在设置中修改并立即生效 | `internal/ui/prefs_test.go` `TestRecordShortcut`；`internal/ui/keys/keys_test.go` `TestTableAndConflicts` | — | 通过 |
| A3 | 无需先填标题、位置或分类，即可开始输入或粘贴 | `internal/ui/app_test.go` `TestTypeKeepsAndTitles`、`TestCopyAllKeepsLineEndings`（新窗口里直接输入、直接粘贴） | — | 通过 |
| A4 | 从新建到复制正文不被保存对话框或分类步骤打断 | `internal/ui/app_test.go` `TestCopyAllKeepsLineEndings`（粘贴后直接 ⌘⇧C） | — | 通过 |
| A5 | 复制得到的是正文，不含列表标题；一键复制全部得到完整正文 | `internal/ui/app_test.go` `TestCopyAllKeepsLineEndings`；`internal/ui/library_test.go` `TestLibraryMasksAndCopies`（列表中遮蔽，复制仍是原文） | — | 通过 |
| A6 | 连续编辑多个片段后，用「最近」切换器两步内回到其中最近修改的任意一条 | `internal/ui/app_test.go` `TestSwitcher`；`internal/ui/sidebar_test.go` `TestSidebarOrdersByLastEditWithoutJumping`（切换器顺序） | — | 通过 |
| A7 | （新）按切换器快捷键弹出列表后，用 Tab 切换选中项并打开 | `internal/ui/app_test.go` `TestSwitcher` | M4（真实键盘下 ⌘\`） | 通过；M4 手工 |
| A8 | （新）切换器只列出已打开的 tab；已关闭的片段可通过列表或搜索找回 | 找回部分：`internal/ui/library_test.go` `TestLibrarySearchFindsClosed`。切换器不列已关闭片段：尚无断言（实现上切换器只遍历工作区的 tab） | — | 未做：给 `TestSwitcher` 加「关闭一个 tab 后不在切换器中」的断言（M2） |
| A9 | （新）弹出时默认选中上一条，快捷键 + 回车即回到它；Shift+Tab 反向，Esc 取消 | `internal/ui/app_test.go` `TestSwitcher` | — | 通过 |
| A10 | （新）macOS 上松开 ⌘ 后用 Tab 选择，不触发系统 ⌘Tab 应用切换 | — | M4 第 5 步 | 手工 |
| A11 | 列表按最近修改排序，每条显示标题与创建 / 修改时间 | `internal/ui/sidebar_test.go` `TestSidebarOrdersByLastEditWithoutJumping`；`internal/ui/library_test.go` `TestLibrarySearchFindsClosed`（日期分组）；`internal/store/store_test.go` `TestWorkspaceCloseIsNotDelete` | — | 通过 |
| A12 | 已关闭的片段仍能被全文搜索找到 | `internal/ui/library_test.go` `TestLibrarySearchFindsClosed`；`internal/store/store_test.go` `TestSearch`、`TestClosedOnlyAndVersionCounts` | — | 通过 |
| A13 | 列表与搜索需主动呼出，默认面对编辑视图 | 部分：`TestLibrarySearchFindsClosed`（Enter 打开后回到编辑器）。启动时默认是编辑视图：尚无断言 | M6 第 1 步 | 手工 |
| A14 | 标题仍在生成时，仍可继续编辑或复制正文 | 尚无（标题在本地同步生成，`TestTypeKeepsAndTitles` 只验证生成结果） | M6 第 2 步 | 手工 |
| A15 | 修改旧片段后，能在版本历史中找回修改前的内容 | `internal/ui/history_test.go` `TestHistoryListsAndCompares`、`TestHistoryRestoreAndUndo`、`TestHistoryFromLibrary`；`internal/snippet/snippet_test.go` `TestRecordRestore`；`internal/history/history_test.go` `TestSnapshotDedupes` | — | 通过 |
| A16 | 按 ⌘T / Ctrl+T 直接打开可输入的空白待办清单，无需命名 | `internal/ui/todo_test.go` `TestTodoEnterAddsAndSaves`（经 `newTodo`：⌘T 后焦点直接在「Add an item」）及其余 `TestTodo*` | — | 通过 |
| A17 | 待办清单同样自动留存，出现在列表与搜索中；打开时也出现在「最近」切换器中 | 留存：`internal/ui/todo_test.go` `TestTodoEnterAddsAndSaves`、`TestTodoBodyRoundTrips`；列表与搜索：`internal/ui/library_test.go` `TestLibraryFiltersAndKeys`（「待办」过滤）；`internal/todo/todo_test.go` `TestParse`、`TestRoundTrip`。切换器中的待办：尚无断言 | — | 未做：给 `TestSwitcher` 加一个待办 tab 的断言 |
| A18 | Agent 能搜索并读取全部片段，包括非代码内容（随手记录、待办） | `cmd/lastab/main_test.go` `TestListSearchRead`；安装：`internal/install` 的 `TestCLI*`、`TestSkill*`，`internal/ui/agent_test.go` `TestInstallSkillOnClick` 等 | M7（Skill → CLI 全链路） | 通过；M7 手工 |
| A19 | 关闭「允许 Agent 访问」后，Agent 无法读取任何片段 | `cmd/lastab/main_test.go` `TestAccessOffPrintsNothingButTheError`；`internal/ui/prefs_test.go` `TestEditorFontAndAgent` | — | 通过 |
| A20 | 默认情况下，Agent 读到的疑似密钥被遮蔽 | `cmd/lastab/main_test.go` `TestListSearchRead`（`read` 与 `search` 预览遮蔽；允许原文后才输出原文）；`internal/redact/redact_test.go` `TestMasks`、`TestPrivateKey`、`TestNoFalsePositives` | — | 通过 |
| A21 | （新）系统切换浅色 / 深色时应用即时跟随；设置中可固定为浅色或深色 | 固定部分：`internal/ui/prefs_test.go` `TestSettingsOpenAndGeneral`；`internal/theme/theme_test.go` `TestBuild` | M5 第 1–2 步 | 手工 |
| A22 | （新）深色模式下启动或新建片段时不出现白屏闪烁 | — | M5 第 3–4 步 | 手工 |
| A23 | （新）切换界面语言后，界面文字与相对时间即时变化，正文不变 | 部分：`internal/ui/prefs_test.go` `TestSettingsOpenAndGeneral`（切换到 English 并写入设置） | M5 第 5–6 步 | 手工 |
| A24 | 粘贴含引号、破折号、Tab 的命令或 JSON，复制出去后与原文逐字节一致 | 行尾：`internal/ui/app_test.go` `TestCopyAllKeepsLineEndings`；存储：`internal/store/store_test.go` `TestCreateSaveGet` | M3 | 手工 |
| A25 | 未输入内容的新建在离开后不出现在列表中 | `internal/ui/app_test.go` `TestBlankTabIsNotStored`；`internal/snippet/snippet_test.go` `TestIsBlank` | — | 通过 |
| A26 | 可用中文、英文或代码符号片段搜到对应正文 | `internal/store/store_test.go` `TestSearch`、`TestShortSearch`；`internal/search/search_test.go` `TestPreviewCJK`、`TestPreviewHighlights`；`internal/ui/library_test.go` `TestLibrarySearchFindsClosed`。1 万条下的延迟：`internal/store` 与 `internal/search` 的基准测试（开发文档 12.1.1） | — | 通过 |
| A27 | 异常退出后再打开，能看到此前已自动留存的内容 | `internal/snippet/snippet_test.go` `TestSessionCrash`；`internal/ui/app_test.go` `TestCloseIsNotDeleteAndRestore`（同一数据库重新打开恢复 tab）、`TestSaveFailureStaysWithRetry` | M6 第 3 步 | 通过；M6 手工 |
| A28 | 含 `Authorization` 等密钥的片段，列表中不明文显示密钥 | `internal/ui/library_test.go` `TestLibraryMasksAndCopies`；`internal/search/search_test.go` `TestPreviewMasks`、`TestPreviewWindow`；`internal/title/title_test.go` `TestMake` | — | 通过 |
| A29 | macOS 与 Windows 上以上行为一致 | CI 在 macOS 与 Windows 上跑全部测试（`.github/workflows/ci.yml`） | 上述每项手工检查在两个平台各做一遍 | 手工 |
| S1 | 成功标准的对照测试：约 20 个片段，在 Sublime 与 Lastab 中找回 3 分钟前编辑的那条，Lastab 更快 | — | M8 | 手工 |

## 手工检查

通用准备：

- 用待验收提交构建应用：`go run ./scripts/build -platform darwin/universal`（macOS）或 `go run ./scripts/build -platform windows/amd64`（Windows）。安装后启动，确认菜单「关于」里的版本与提交一致。
- 每项检查在结果表里加一行：日期、机器与系统版本、Lastab 提交、检查人、结果（通过 / 不通过）、备注（不通过时写现象，附录屏或截图路径）。
- 不通过时先对照开发文档第 12 节的「不通过时的备选」，再决定修复或豁免。

M0 决策点（开发计划 M0「验收」）要求 **M1 呼出速度、M3 文本保真、M2 中文输入法** 三项必须通过；M4 不通过时按备选把切换器默认键改为 ⌃\`，计划不变。

### M1 新建 / 呼出速度对比 Sublime Text（A1，验证项 1）

准备：

1. 同一台机器，接通电源，关闭低电量模式；退出其他占用 CPU 的应用。
2. 安装 Sublime Text 4（记下版本号），启动后关掉所有窗口，保持在后台运行。Lastab 同样启动后保持常驻（关闭窗口但不退出）。
3. 打开按键显示工具（macOS：KeyCastr，Windows：Carnac），让按键在屏幕上出现。
4. 用 60 fps 以上录屏：macOS 用 QuickTime「新建屏幕录制」或 ⌘⇧5；Windows 用 OBS（60 fps）。

步骤（每种情形各做 10 次，两款应用交替进行）：

1. **应用在前台**：激活应用，按 ⌘N / Ctrl+N，紧接着键入 `x`。
2. **应用在后台**（Lastab 用全局呼出热键，默认 ⌃⌥N / Ctrl+Alt+N；Sublime 用 Dock / 任务栏点击后按 ⌘N，或配置等效的全局热键）：先切到 Finder / 资源管理器，再呼出并键入 `x`。
3. 录屏逐帧查看：从按键显示工具出现该按键的那一帧，到新编辑区出现光标、并且 `x` 显示出来的那一帧，记下帧数，换算成毫秒。

通过标准：每种情形下，Lastab 10 次的中位数不大于 Sublime 的中位数（相差在 1 帧以内视为持平）。

| 日期 | 机器 / 系统 | Lastab 提交 | Sublime 版本 | 情形 | Lastab 中位数（ms） | Sublime 中位数（ms） | 结果 | 检查人 | 备注 |
| --- | --- | --- | --- | --- | ---: | ---: | --- | --- | --- |
| | | | | 前台 ⌘N | | | | | |
| | | | | 后台呼出 | | | | | |

### M2 中文输入法（A3、A26 的真实输入，验证项 5）

输入法：macOS 自带「简体拼音」、搜狗输入法 Mac 版（记下版本），Windows 自带「微软拼音」。每种输入法在以下四个输入位置各做一遍：编辑器正文、⌘K 搜索框、⌘\` 切换器的过滤框、重命名标题的输入框。

步骤：

1. 切到该输入法，键入 `zhongwen`，用空格选第一个候选「中文」。确认出现且只出现一次「中文」，没有残留的 `zhongwen` 字母。
2. 键入 `shurufa`，用数字键 `2`（或候选列表中的第二项）选词，确认上屏的是所选的词。
3. 组合过程中观察候选窗：应紧贴光标下方（或上方，空间不足时），不出现在窗口左上角或屏幕角落。在编辑器靠近窗口底部、右侧的位置各试一次。
4. 键入 `ceshi` 后按 Backspace 两次，再继续键入 `shi`：组合串正确删改，上屏结果与预期一致。
5. 键入 `abc` 后按 Enter（拼音输入法的「输入英文」）：上屏 `abc`；在 ⌘K 搜索框和切换器过滤框中，这次 Enter 只用于上屏，**不**触发「打开选中项」。
6. 组合中按 Esc：只取消组合；切换器与 ⌘K 不应因这次 Esc 关闭。
7. 连续快速键入一句 `jintiankaihui，xiawusandian。`（含全角标点），选词上屏：无丢字、无重复、标点为全角。
8. 在搜索框中用输入法输入「周会」，确认结果列表随上屏的文字更新（组合中的拼音不应触发搜索结果闪烁为空）。

通过标准：四个位置、三种输入法下，以上每一步都无丢字、无重复、候选窗位置正确，Enter / Esc 行为如上。

| 日期 | 系统 | 输入法与版本 | 输入位置 | 第 1–8 步结果（不通过的步号） | 结果 | 检查人 | 备注 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| | macOS | 简体拼音 | 编辑器 | | | | |
| | macOS | 简体拼音 | ⌘K 搜索 | | | | |
| | macOS | 简体拼音 | 切换器过滤 | | | | |
| | macOS | 简体拼音 | 重命名 | | | | |
| | macOS | 搜狗 | 编辑器 | | | | |
| | macOS | 搜狗 | ⌘K 搜索 | | | | |
| | macOS | 搜狗 | 切换器过滤 | | | | |
| | macOS | 搜狗 | 重命名 | | | | |
| | Windows | 微软拼音 | 编辑器 | | | | |
| | Windows | 微软拼音 | Ctrl+K 搜索 | | | | |
| | Windows | 微软拼音 | 切换器过滤 | | | | |
| | Windows | 微软拼音 | 重命名 | | | | |

### M3 文本保真与无智能替换（A24，验证项 3、4）

准备（macOS）：在「系统设置 → 键盘 → 文字输入 → 输入法 → 编辑」中**打开**「自动更正拼写」「自动大写单词首字母」「连按两下空格键插入句号」「使用智能引号和破折号」，让系统尽量去替换。Windows 上打开「设置 → 时间和语言 → 输入 → 键入」中的自动更正与文本建议。

样本（保存为 `sample.txt`，含直引号、两个连字符、Tab、行尾空格、全角字符、emoji 与 CRLF）：

```bash
printf 'curl -H "Content-Type: application/json" --data '"'"'{"name": "it'"'"'s -- ok"}'"'"'\r\n\tindented\ttab  \r\nhello world. new sentence -- dash...\n全角：「引号」，emoji 🙂\n' > sample.txt
```

步骤：

1. **粘贴**：`pbcopy < sample.txt`（Windows：`Get-Content -Raw sample.txt | Set-Clipboard`），在 Lastab 新建片段并粘贴，再按「复制全部」，然后 `pbpaste > out.txt && cmp sample.txt out.txt`（Windows：`Get-Clipboard -Raw | Set-Content -NoNewline out.txt`，再 `fc.exe /b sample.txt out.txt`）。无输出即一致。
2. **键入**：在新片段里手动键入 `"quote" 'single' -- dash... hello. world`，在句号后连按两下空格，并在行首键入小写单词。复制全部后检查：引号仍是直引号 `"` `'`，`--` 未变成破折号，`...` 未变成省略号，没有自动插入句号，小写未被改成大写，没有拼写更正。
3. **编辑后再复制**：在第 1 步的片段中间插入一个字符再删除，复制全部，再做一次 `cmp`。
4. **选区复制**：选中第 1 步片段的第二行（含 Tab 与行尾空格），⌘C，用 `pbpaste | xxd` 确认 Tab（`09`）、行尾两个空格（`20 20`）与 `\r\n`（`0d 0a`）都在。
5. 关闭再重新打开该片段（或退出后重启应用），重复第 1 步的复制与 `cmp`。

通过标准：每一步逐字节一致，键入的内容没有任何替换。

| 日期 | 系统 | Lastab 提交 | 第 1–5 步结果（不通过的步号） | 结果 | 检查人 | 备注 |
| --- | --- | --- | --- | --- | --- | --- |
| | macOS | | | | | |
| | Windows | | | | | |

### M4 ⌘\` 接管与键盘布局（A7、A10，验证项 2）

键盘布局（macOS「系统设置 → 键盘 → 文字输入」中添加）：ABC（美式）、简体拼音、英国、德语（QWERTZ）、法语（AZERTY）、日语（罗马字，JIS 键盘若有就用实体键盘）、Dvorak。

步骤（每种布局各做一遍）：

1. 准备：Lastab 打开 3 个片段；另开一个有两个窗口的应用（如 Finder 两个窗口），确认系统的 ⌘\`（同一应用内切换窗口）在那个应用里有效。
2. 切到 Lastab，按住 ⌘ 再按数字 1 左边的那个键（德语、法语布局下这个键上印的不是 \`，照样按这个物理键）：「最近」切换器弹出，系统**没有**切换窗口。记下这一布局下菜单栏中「最近」显示的快捷键文字。
3. 切换器打开时再按 ⌘\`：选中项下移；按 ⌘⇧\`：选中项上移。
4. 松开 ⌘，按 Enter：回到选中的片段，切换器关闭。
5. 再次打开切换器后**松开 ⌘**，按 Tab、Shift+Tab 选择：只移动切换器中的选中项，**不**出现系统的 ⌘Tab 应用切换面板；按 Esc 关闭切换器，焦点回到原来的片段。
6. 在设置中把「最近」改成别的组合再改回 ⌘\`，重复第 2 步。

通过标准：所有布局下第 2–6 步都按描述工作。任一布局下第 2 步被系统抢走时，按开发文档的备选把默认改为 ⌃\`，并在备注写明是哪种布局。

| 日期 | macOS 版本 | 键盘布局 | 菜单显示的快捷键 | 第 2–6 步结果（不通过的步号） | 结果 | 检查人 | 备注 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| | | ABC | | | | | |
| | | 简体拼音 | | | | | |
| | | 英国 | | | | | |
| | | 德语 | | | | | |
| | | 法语 | | | | | |
| | | 日语 | | | | | |
| | | Dvorak | | | | | |

### M5 外观与语言（A21–A23，验证项 9、10）

1. 设置中外观选「跟随系统」。在系统设置中把外观从浅色切到深色、再切回：Lastab 主窗口、切换器、⌘K、设置窗口都立即跟随，无需重启。
2. 设置中固定为「浅色」，再把系统切到深色：Lastab 保持浅色；改为「深色」后同理。
3. 系统为深色、Lastab 跟随系统时，退出 Lastab 再启动（录屏 60 fps）：逐帧查看，第一帧起就是深色，没有白色帧。
4. 深色下按 ⌘N 与 ⌘T 新建：新建过程中没有白色帧。
5. 设置中把语言从「简体中文」切到「English」再切回：菜单、侧栏、⌘K、设置与相对时间（如「3 分钟前」/「3 min ago」）立即变化；片段正文与标题内容不变。
6. 在中文界面下检查中英混排（如「已留存 · UTF-8」「1 万条」）的细空格与等宽数字，与 `design/` 中的设计稿一致。

通过标准：每步如上；第 3、4 步录屏里没有白色帧。

| 日期 | 系统 | Lastab 提交 | 第 1–6 步结果（不通过的步号） | 结果 | 检查人 | 备注 |
| --- | --- | --- | --- | --- | --- | --- |
| | macOS | | | | | |
| | Windows | | | | | |

### M6 默认视图、标题生成与异常恢复（A13、A14、A27）

1. 启动应用、用全局热键呼出、⌘N 新建：每次面对的都是编辑视图，光标在正文中；列表与 ⌘K 不会自己弹出。
2. 新建片段，快速粘贴一段 20 KB 的 JSON，立刻继续键入并按「复制全部」：键入不卡顿，复制得到完整正文；标题在之后出现，且没有写进正文。
3. 在一个片段中键入一行文字，等状态栏显示「已留存」，然后强制结束进程（macOS：`kill -9 $(pgrep -x Lastab)`；Windows：任务管理器「结束任务」或 `taskkill /F /IM Lastab.exe`）。重新启动：该片段与其他打开的 tab 都在，内容是已留存的版本；应用提示上次异常退出（如有该提示）。

通过标准：每步如上。

| 日期 | 系统 | Lastab 提交 | 第 1–3 步结果（不通过的步号） | 结果 | 检查人 | 备注 |
| --- | --- | --- | --- | --- | --- | --- |
| | macOS | | | | | |
| | Windows | | | | | |

### M7 Agent 全链路（A18，验证项 11）

1. 在设置中打开「允许 Agent 访问」，按提示安装命令行与 Skill。
2. 在 Claude Code 中提问「在 Lastab 里找一下周会纪要」：Claude 调用 Skill → `lastab search`，返回正确的片段；再请它读出全文，`lastab read` 的结果中密钥被遮蔽。
3. 设置的访问记录中出现这两次调用（客户端为 `claude-code`）。
4. 关闭「允许 Agent 访问」后重复第 2 步：Claude 得到「访问已关闭」的错误，读不到任何内容。

| 日期 | 系统 | Lastab 提交 | Claude Code 版本 | 第 1–4 步结果 | 结果 | 检查人 | 备注 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| | | | | | | | |

### M8 找回对照（S1，开发文档 11.2）

1. 在 Sublime 与 Lastab 中各开约 20 个片段（内容相同：命令、JSON、随手记录混合），依次编辑。
2. 3 分钟后，由另一人说出其中一条的内容要点，检查人分别在 Sublime 与 Lastab 中找回它并开始编辑，计时（从听到要点到光标落在那条片段中）。
3. 换不同的片段重复 5 次，两款应用交替先后。

通过标准：Lastab 的中位数用时短于 Sublime。

| 日期 | 系统 | Lastab 提交 | Sublime 中位数（s） | Lastab 中位数（s） | 结果 | 检查人 | 备注 |
| --- | --- | --- | ---: | ---: | --- | --- | --- |
| | | | | | | | |
