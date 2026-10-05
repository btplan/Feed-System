# 阶段 3：带 Token 查看当前用户资料

本阶段接口是 `GET /users/me`。调用方不传 `user_id`；后端只根据已经验证的 Token 判断“当前是谁”，因此用户不能通过改 URL 读取别人的资料。

## 先理解 Token 保存在哪里

登录接口返回 `access_token` 后，**调用方**负责保存它。后端不会把每个调用方的 Token 存进 Go 文件或 users 表。

当前没有浏览器前端或手机 App，所以我们用 PowerShell 变量模拟调用方保存 Token：

```powershell
$login = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18080/users/login" -ContentType "application/json" -Body '{"username":"clean_user_01","password":"passw0rd!"}'
$accessToken = $login.access_token
```

`$accessToken` 存在于当前 PowerShell 窗口；关闭窗口后它消失。真实浏览器通常存到内存、`localStorage` 或 Cookie，具体选择要由前端安全策略决定。

下一次请求由调用方把它放进请求头：

```text
Authorization: Bearer <accessToken 的值>
```

## 业务过程和当前代码放在一起看

### 1. Router 规定资料接口必须认证

业务上，“查看自己的资料”必须先知道调用者是谁，因此游客不能直接进入资料 Handler。

代码在 [internal/httpapi/router.go](../internal/httpapi/router.go) 第 14 行的 `NewRouter`：

- 第 20–21 行创建 `protected` 路由组，并挂载认证中间件 `middleware.RequireUser(tokens)`。
- 第 22 行把 `GET /users/me` 交给 `accounts.Profile`。

`/users/me` 位于 `protected` 中，所以 Gin 的执行顺序是“中间件成功，再执行 Profile”。它不在 `account/handler.go` 配置 URL；Router 专门负责这件事。

### 2. 认证中间件确认 Token 是否可信

业务上，Token 不是“客户端说自己是谁就信谁”。后端必须检查请求头有 `Bearer ` 前缀、Token 未被修改、它确实由本服务密钥签发，并且尚未过期。

**中间件文件**是 [internal/httpapi/middleware/auth.go](../internal/httpapi/middleware/auth.go)，第 14 行的 `RequireUser`：

1. 读取 HTTP 的 `Authorization` 请求头；没有 `Bearer ` 就用 401 终止请求。
2. 调用 [internal/auth/jwt.go](../internal/auth/jwt.go) 第 46 行的 `TokenManager.Parse`。
3. `Parse` 使用本项目启动时创建的 TokenManager 和 `jwt_secret` 验证 JWT 签名、算法和过期时间。签名不匹配、Token 被改、格式错误或过期，都会返回错误。
4. 验证成功后，`RequireUser` 将 Claims 中的 `user_id` 放进 Gin **本次请求上下文**，再调用 `c.Next()` 让请求继续。

这个用户 ID 只在本次 HTTP 请求中临时存在；它不是数据库字段，也不是全局变量。

### 3. Profile Handler 读取已验证身份，不接受客户端指定用户

业务上，中间件已经确定当前用户 ID，Handler 只需取出它并查询资料。

代码在 [internal/account/handler.go](../internal/account/handler.go) 第 42 行的 `Profile`：它调用 [internal/httpapi/middleware/auth.go](../internal/httpapi/middleware/auth.go) 第 32 行的 `CurrentUserID` 读取上下文中的 ID。客户端即使添加 `?user_id=999` 或 JSON 的 `user_id`，这个 Handler 也不会读取它。

然后 Handler 调用账号业务层，成功返回 200；如果 Token 指向已不存在的用户，返回 401；数据库意外错误才返回 500。

### 4. Service 和 Repository 查询当前用户

业务上，Token 只能证明“当初登录的是谁”；返回资料前仍要从数据库读取当前记录，才能得到最新 username、created_at 等信息。

- [internal/account/service.go](../internal/account/service.go) 第 53 行的 `Profile` 负责“此用户是否还存在”的业务判断。
- [internal/account/repository.go](../internal/account/repository.go) 第 32 行的 `FindByID` 用 Gorm 从 `users` 表取一行记录。
- `User.PasswordHash` 的 JSON 标签是 `json:"-"`，所以响应不会泄露密码哈希。

调用图：

```text
PowerShell 变量 $accessToken
  → Authorization: Bearer <token>
  → httpapi/router.go：protected 路由组
  → httpapi/middleware/auth.go：RequireUser
  → auth/jwt.go：TokenManager.Parse
  → account/handler.go：Profile
  → account/service.go：Profile
  → account/repository.go：FindByID
  → 200 JSON user
```

## 手动验收

启动服务：

```powershell
.\run.ps1
```

另开一个 PowerShell，登录并保存 Token：

```powershell
$login = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18080/users/login" -ContentType "application/json" -Body '{"username":"clean_user_01","password":"passw0rd!"}'
$accessToken = $login.access_token
```

带 Token 查询：

```powershell
curl.exe -i "http://127.0.0.1:18080/users/me" -H "Authorization: Bearer $accessToken"
```

预期 `200 OK`，响应有 `id`、`username`、`created_at`，没有 `password_hash`。

不带请求头再试：

```powershell
curl.exe -i "http://127.0.0.1:18080/users/me"
```

预期 `401 Unauthorized`，而且 `account.Handler.Profile` 不会被执行。