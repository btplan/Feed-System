# 阶段 1：用户注册

本阶段完成一条完整业务链：新用户提交用户名和密码，后端校验输入、加密密码、写入 SQLite 的 `users` 表，并返回安全的用户资料。

## 业务规则

| 项目 | 规则 |
| --- | --- |
| username | 3～30 个 ASCII 字符，只允许字母、数字和下划线 |
| password | 8～72 个字符 |
| username 唯一性 | 数据库唯一索引保证不能重复注册 |
| password 保存方式 | 只保存 bcrypt `password_hash`，不保存原文 |

接口：`POST /users/register`

请求：

```json
{"username":"alice_01","password":"passw0rd!"}
```

成功响应：

```json
{"id":1,"username":"alice_01","created_at":"2026-09-29T14:00:00+08:00"}
```

响应中没有 `password` 或 `password_hash`。

## 先看两个最小例子

数据库唯一性：两个请求同时注册 `alice` 时，代码中的“先查再写”不足以防止并发重复；
`users.username` 的唯一索引才是最后的保证。

bcrypt：原始密码 `passw0rd!` 经 `bcrypt.GenerateFromPassword` 变为不可逆哈希。
登录时用 `bcrypt.CompareHashAndPassword` 比对输入与哈希，无法将哈希还原为原始密码。

## 先从业务逻辑理解注册

注册的目标不是“往 users 表插入一行”，而是：**让一个新用户获得可用于登录的账号，同时保证账号不会重复、密码不会以明文泄露。**

业务过程按发生顺序是：

```text
用户提交用户名和密码
  → 系统检查格式是否符合账号规则
  → 系统把密码转换为不可逆哈希
  → 数据库以 username 唯一索引拒绝重复账号
  → 成功后返回可公开的用户资料
```

据此划分代码职责：Handler 负责接收 JSON 和返回 HTTP；Service 负责格式校验、密码哈希和业务错误；Repository 负责写入数据库；数据库的唯一索引负责最终保证用户名不重复。先理解这条链，再去看代码中的每一层。
## 再到代码里看这条链

代码阅读顺序跟请求实际经过的顺序一致：

1. 点击 [internal/account/handler.go](../internal/account/handler.go)，看第 47 行。先看 HTTP 请求如何被接住、JSON 如何被读取。
2. 点击 [internal/account/service.go](../internal/account/service.go)，看第 52 行。这里决定用户名和密码是否合格，并生成密码哈希。
3. 点击 [internal/account/repository.go](../internal/account/repository.go)，看第 20 行。这里把 User 写入数据库。
4. 点击 [internal/account/entity.go](../internal/account/entity.go)，看第 6 行。回头确认 User 的字段如何映射为 users 表，以及为什么 `PasswordHash` 不会返回给客户端。
5. 点击 [internal/database/database.go](../internal/database/database.go)，看第 13 行。这里是 Gorm 打开 SQLite 的位置。
6. 最后点击 [cmd/api/main.go](../cmd/api/main.go)，看第 22 行。它只负责把数据库、迁移和路由组装起来，让整条链能启动。
## 调用图

```text
POST /users/register
  → Handler.Register：读取 JSON，确定 HTTP 状态码
  → Service.Register：校验用户名和密码，生成 bcrypt 哈希
  → Repository.Create：Gorm INSERT users
  → SQLite：唯一索引允许或拒绝写入
  → JSON 响应：201 / 400 / 409
```

启动时：

```text
main → config.Load → database.Open → account.Migrate → RegisterRoutes → router.Run
```

## 本次改动位置

| 文件 | 看什么 |
| --- | --- |
| [cmd/api/main.go](../cmd/api/main.go) 第 22 行 | 打开数据库、建 users 表、注册路由 |
| [internal/database/database.go](../internal/database/database.go) 第 13 行 | Gorm 如何打开 SQLite 文件 |
| [internal/account/entity.go](../internal/account/entity.go) 第 6 行 | User 表字段、请求与响应结构体 |
| [internal/account/handler.go](../internal/account/handler.go) 第 47 行 | HTTP 与 JSON 的入口和出口 |
| [internal/account/service.go](../internal/account/service.go) 第 52 行 | 格式校验、bcrypt 和业务错误 |
| [internal/account/repository.go](../internal/account/repository.go) 第 20 行 | Gorm 的 INSERT 与迁移 |
| [internal/account/handler_test.go](../internal/account/handler_test.go) 第 18 行 | 注册链的自动验证 |

## 手动验收

先启动：

```powershell
.\run.ps1
```

当前 YAML 的端口是 18080。新开 PowerShell，执行下面命令；Windows PowerShell 调用 `curl.exe` 时，
JSON 字段名的双引号需写成 `\"`：

```powershell
curl.exe -i -X POST "http://127.0.0.1:18080/users/register" -H "Content-Type: application/json" -d '{\"username\":\"alice_01\",\"password\":\"passw0rd!\"}'
```

第一次应返回 `201 Created`；再执行同一条，应返回 `409 Conflict`。

用无效用户名验证 400：

```powershell
curl.exe -i -X POST "http://127.0.0.1:18080/users/register" -H "Content-Type: application/json" -d '{\"username\":\"ab\",\"password\":\"passw0rd!\"}'
```

数据库文件来自 YAML 的 `database.path`，即 `.run/clipflow-learning.db`。这个新文件避免使用重置前被占用的旧数据库。
下一阶段开始前，可以用 SQLite 工具执行：

```sql
SELECT id, username, password_hash, created_at FROM users;
```

你应该看到用户名和密码哈希，绝不会看到 `passw0rd!` 原文。
