# PM Jira — 个人项目管理工具

一个为项目经理个人使用设计的轻量项目管理网站，最终可编译为 Windows 单文件 exe，双击即可启动。数据存储在本地 SQLite 中，无需单独安装数据库。

- 单一可执行文件，页面、样式、脚本和 SQL 迁移全部嵌入二进制
- 纯 Go + SQLite（`modernc.org/sqlite`，无 CGO），支持 Windows 交叉编译
- 服务端渲染 HTML，使用 HTMX 和少量原生 JavaScript 提供渐进增强
- 默认适合本机个人使用；如需局域网访问，请显式配置监听地址并设置管理密码

## 功能一览

| 模块 | 说明 |
|------|------|
| **仪表盘** | 进行中、已完成、超期、总数统计；超期提醒、3 天内到期提醒、事务状态分布、最近更新、近期版本 |
| **个人事务** | 标题、Markdown 描述、状态、优先级、截止日期；按截止日期排序并支持搜索、状态、优先级筛选 |
| **看板** | 待办 / 进行中 / 审核中 / 已完成四列看板，拖拽卡片即可修改状态 |
| **事务详情** | Markdown 描述、评论、变更历史、附件和快捷改状态 |
| **版本计划** | 独立维护版本号、Markdown 说明、转测日期、发布日期和状态；支持分组、平铺、路线图视图 |
| **版本编号** | 支持 `R###C##B###`、`SPC`、`CP` 预发布格式和 `R##C##` 关键节点格式 |
| **超期提醒** | 仪表盘和事务列表对超期或临近截止的事务进行提示 |
| **快捷导航** | 在设置中维护常用网页，支持分类、页面内嵌 iframe 或新窗口打开 |
| **附件** | 事务详情页上传、下载、删除附件；图片/PDF 可在线预览，单文件上限 50 MB |
| **导出报表** | 按当前筛选条件导出事务 CSV，带 UTF-8 BOM，Excel 可直接打开中文 |
| **人员速查** | 登录后可维护姓名、工号、多个角色、团队，以及接口人链接清单 |
| **管理密码** | 可选的匿名只读 / 管理员写入模式，登录有效期默认 30 天 |

事务与版本是两个独立模块：版本不关联事务，也不计算事务完成率。版本路线图展示版本自身的日期、状态和说明，不展示关联事务卡片。

## Markdown

事务描述和版本说明支持服务端 Markdown 渲染，并提供编辑 / 预览切换。支持标题、列表、引用、粗体、斜体、删除线、链接、代码块、表格和任务列表。

- 数据库保存原始 Markdown，重新编辑时不会被转换为 HTML
- 预览和最终页面使用同一套 Goldmark + Bluemonday 渲染与安全过滤
- 原始 HTML、脚本、事件属性、危险链接和远程图片会被过滤
- 图片请使用事务附件功能上传

## 管理密码

在 exe 同级目录的 `jira_config.json` 中设置密码：

```json
{
  "password": "你的管理密码",
  "session_days": 30
}
```

- `password` 留空时不启用保护，所有访问者都拥有写权限
- 设置密码后，普通页面可匿名只读访问
- 新建/编辑/删除、设置、Markdown 预览 POST、评论、附件变更等写操作需要登录
- 「人员速查」模块整体仅登录可见
- 登录态采用签名 Cookie，默认有效 30 天；修改密码后旧登录立即失效
- 本工具面向个人本地使用，不提供多账号或细粒度权限，不建议直接暴露到公网

## 一键使用（Windows）

`build.sh` 或 `build.bat` 会生成：

| 文件 | 说明 | 如何停止 |
|------|------|---------|
| `pmjira.exe` | Windows 控制台版，显示运行日志 | 关闭窗口或按 `Ctrl+C` |
| `pmjira-gui.exe` | Windows 无控制台版，后台运行 | 网页「设置 → 停止服务」 |

把 exe 复制到一个可写目录后双击运行。首次启动会在同级目录创建：

- `pmjira.db`：SQLite 数据库
- `jira_config.json`：管理密码配置示例
- `attachments/`：附件文件
- `pmjira.log`：GUI 版日志

备份时请同时备份数据库、配置文件和 `attachments/` 目录。配置文件包含明文密码，请妥善保护。

## 从源码构建

需要 **Go 1.26.5**。本项目无 CGO 依赖，编译不需要 GCC。

### Linux/macOS

```bash
go mod download
go test ./...
go vet ./...
CGO_ENABLED=0 go build .
./build.sh
```

`build.sh` 产出本平台 `pmjira`，以及 Windows 控制台版和 GUI 版。

### GitHub Releases

向仓库推送形如 `v1.0.0` 的 Git tag 后，GitHub Actions 会自动运行测试和静态检查，并创建 Release。每个 Release 包含：

- `pmdesk-linux-amd64`：Linux amd64 版
- `pmdesk-windows-amd64.exe`：Windows amd64 控制台版
- `pmdesk-windows-amd64-gui.exe`：Windows amd64 无控制台版
- `SHA256SUMS.txt`：上述文件的 SHA-256 校验值

Release 二进制页脚显示对应的 tag 版本号。工作流不会在普通分支推送时发布 Release。

### Windows

```bat
build.bat
```

### 手动交叉编译 GUI 版

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags="-s -w -H windowsgui -X main.guiBuild=true" -o pmjira-gui.exe .
```

### 本地运行

开发时建议显式指定运行数据路径，避免 `go run` 的临时可执行文件目录造成困惑：

```bash
go run . -addr 127.0.0.1:8765 -no-browser \
  -db ./pmjira.db -config ./jira_config.json
```

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-addr` | `:8765` | 监听地址；端口占用时自动回退到空闲端口。个人使用建议显式传 `127.0.0.1:8765` |
| `-db` | exe 同级 `pmjira.db` | SQLite 数据库路径 |
| `-config` | exe 同级 `jira_config.json` | 管理密码和登录有效期配置 |
| `-no-browser` | `false` | 启动时不自动打开浏览器 |

示例：

```bash
pmjira.exe -addr 127.0.0.1:9000 -db D:\data\pm.db
```

## 使用提示

- **超期判定**：设置了截止日期、状态不是已完成且日期早于今天，即视为超期。
- **事务排序**：事务列表按截止日期升序，未设置截止日期的事务排在最后。
- **版本排序**：版本按转测日期升序，未设置转测日期的版本排在最后；分组视图另按 R/C 层级展示。
- **数据库迁移**：启动时自动按编号应用 `internal/store/migrations/` 中尚未执行的迁移；不要修改已经应用过的迁移文件。升级到 0008 时会移除历史上已废弃的负责人、目标版本和史诗关联字段；如需保留旧数据，请先备份数据库。
- **附件清理**：删除事务时会清理其附件文件和数据库元数据。
- **快捷导航内嵌**：部分网站禁止 iframe 内嵌，这类网址请选择新窗口打开。

## 项目结构

```text
main.go                         入口、参数、监听、浏览器启动和优雅退出
internal/store/                 SQLite 数据层
  migrations/                   版本化 SQL 迁移（当前至 0008）
  issues.go                     事务、筛选、评论关联和活动记录
  versions.go                   独立版本与版本号解析
  people.go                     人员与接口人链接
internal/web/                   HTTP 路由、处理器、模板和静态资源
  templates/                    html/template 页面与 partial
  static/                       app.css、app.js、htmx.min.js
  markdown.go                   Markdown 渲染、净化和预览接口
  *_test.go                     Store、路由、认证、附件和 Markdown 测试
build.sh / build.bat            本地和 Windows 构建脚本
.github/workflows/ci.yml        GitHub Actions 持续集成
.github/workflows/release.yml   按 v* tag 构建并发布 Release
```

## GitHub 开源仓库准备

仓库辅助文件包括：

- `LICENSE`：项目许可证
- `CONTRIBUTING.md`：开发、测试和迁移约定
- `SECURITY.md`：本地部署与安全报告说明
- `.github/workflows/ci.yml`：测试、vet 和无 CGO 构建
- `.github/workflows/release.yml`：按 `v*` tag 构建 Linux/Windows 资产、生成校验和并发布 Release

生成的 exe、数据库、配置、日志、附件和覆盖率文件不会纳入版本控制。Release 资产由 GitHub Actions 在推送版本 tag 后自动生成。

## 技术栈

Go 1.26.5、标准库 `net/http`、`html/template`、Goldmark、Bluemonday、`modernc.org/sqlite`、HTMX。
