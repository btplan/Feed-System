# 换电脑后继续学习

## 要迁移的内容

1. Clipflow 源码、go.mod、go.sum、configs、run.ps1、AGENTS.md 和整个 doc 目录。
2. 参考项目 feedsystem_video_go 的代码，放在新电脑上可访问的目录中；继续只读使用。
3. 需要保留原始对话时，额外保留旧电脑的本地会话备份；Git 仓库不会自动包含 Codex 的会话文件。

建议两个项目并排放置，根目录和盘符可变：

```text
任意项目目录/
  Feed-System/
  feedsystem_video_go/
```

在 VS Code 打开 Feed-System。新对话中给出参考项目实际路径，先让助手实际列目录并读 README 验证访问。
如果工作区权限不允许读取同级目录，再在当前编辑器/权限设置中加入参考目录。
路径说明和 AGENTS.md 是工作约定，不等于操作系统的只读权限。

## 数据库与运行缓存的迁移

当前 .gitignore 忽略整个 .run/，数据库、上传文件、依赖缓存、编译产物和日志都不随 Git 提交。
需要保留练习数据时，停止服务后单独备份配置指定的 SQLite 数据库（当前为 .run/clipflow-learning.db）及需要保留的媒体文件。若有 WAL 文件，先使用一致性备份方式；不要只复制仍在写入的主文件。
configs/local.yaml 同样不提交，新电脑先从 configs/local.example.yaml 创建本机配置，再按第九课设置环境密钥。

当前 go.mod 要求 Go 1.27.0，旧机验证版本为 1.27.1；新机器先检查 Go 版本。
Windows 可在项目目录运行 `./run.ps1`；其他系统由助手按新环境给等价启动命令。
当前地址在 `configs/local.yaml` 中，示例为 127.0.0.1:18080；不再使用早期的 CLIPFLOW_ADDR。

## 对话历史与接续方式

已确认用户使用 VS Code 的 Codex 扩展。
本机存在 `C:\Users\Plan_\.codex\sessions`、`archived_sessions` 与 `session_index.jsonl`，
以及状态数据库。这些属于本地 Codex 数据，和项目源码是不同存储。
仅凭这些目录存在，不能断言服务端没有保留内容，也不能保证复制某个目录后新版本 UI 一定能恢复原会话。

如重视原始历史，先保留旧机，并在关闭相关程序后私下备份会话数据；
不要把 `.codex/auth.json` 等登录凭据或整个用户配置目录提交进项目仓库。
准确的恢复操作需按旧机与新机的扩展版本、运行环境（Windows/WSL/SSH）再核实。
此次未复制、打包或上传任何会话文件。

学习接续不依赖完整历史导入：AGENTS.md 和 doc 保存当前状态，新对话读它们后即可接着讲。
这恢复的是学习背景与任务进度，不是声称新对话自动拥有所有旧聊天消息。

官方资料核实于 2026-09-29：

- [项目与会话](https://learn.chatgpt.com/docs/projects)：IDE 使用当前文件夹或工作区；长期指导放进 AGENTS.md 或版本控制文档。
- [Codex IDE](https://learn.chatgpt.com/docs/codex/ide)：区分本地工作与委派到云端。
- [Codex cloud](https://learn.chatgpt.com/docs/cloud)：云端任务可回看结果并继续反馈；不构成本地对话自动同步的保证。

## 新电脑开场提示（复制后替换路径）

```text
这是我从另一台电脑迁移来的 Clipflow 学习项目。
请先读 AGENTS.md、doc/README.md、doc/progress.md、doc/step-by-step.md、doc/migration.md，
并检查实际代码，不要只根据旧对话猜进度。

当前项目路径：<新电脑上的 Feed-System 路径>
参考项目路径：<新电脑上的 feedsystem_video_go 路径>，只读，禁止修改。
请先验证两个目录能读，并检查 Go 和项目启动环境。

我已有 SQL 基础和 Go 基本语法，但不会独立设计架构或写完整业务。
前期你负责设计、写代码、验证，我跟着操作。每轮只教一个小步。
每个新概念先举独立最小例子，每个函数上方写一句中文注释。
当前是 Gin + Gorm + SQLite，代码已恢复到第九课；原第十至十二课已撤销。目标先补齐业务 Demo，下一课计划做评论，已有 YAML 配置，
助手验证过代码，但我的理解与操作验收尚未完成。
请接着当前小步教，不要直接生成下一阶段。
```
