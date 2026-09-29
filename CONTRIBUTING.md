# Contributing to PM Jira

感谢你关注 PM Jira。项目是一个面向个人、本地运行的 Go 服务端渲染应用。

## 开发环境

- Go 1.26.5
- SQLite 使用纯 Go 的 `modernc.org/sqlite`，不需要 CGO 或 GCC
- Linux/macOS 可使用 `build.sh`，Windows 可使用 `build.bat`

## 常用命令

```bash
go mod download
go test ./...
go vet ./...
gofmt -w path/to/changed.go
gofmt -l .
CGO_ENABLED=0 go build .
./build.sh
```

提交前请确保测试、`go vet` 和无 CGO 构建通过。

## 代码结构

- `internal/store/`：SQLite 数据访问、模型和迁移
- `internal/web/`：HTTP 路由、处理器、模板、样式和脚本
- `main.go`：启动参数、数据库路径、监听和优雅退出

应用使用嵌入式模板、静态资源和 SQL 迁移，没有单独的前端构建步骤。

## 数据库迁移规则

- 不要修改已经发布或执行过的迁移文件。
- 数据库结构变更请新增递增编号的 SQL 文件。
- SQLite 表重建必须保留无关数据、主键和子表关系，并增加迁移测试。
- 迁移完成后应通过 `PRAGMA foreign_key_check`。

## 提交修改

1. 为行为变化补充测试，优先使用临时 SQLite 数据库和 `httptest`。
2. 保持 Go 文件经过 `gofmt`。
3. 更新 README 或配置文档中的用户可见变化。
4. 不要提交数据库、管理密码配置、日志、附件或本地构建产物。
5. Pull Request 中说明变更内容、验证命令以及任何兼容性影响。
