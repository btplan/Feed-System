# 阶段 4：已登录用户发布视频

本阶段接口是 `POST /videos`。当前“发布”只保存视频元数据：标题和一个现成的 HTTP/HTTPS 播放地址；还不上传本地视频文件。文件上传、对象存储和异步转码会在后续建立在这条数据链之上。

```
请求带 Token
  → RequireUser 中间件验证 Token
  → 从 Claims 取出 user_id
  → 写入 Gin 本次请求上下文
  → Handler.Publish 读取 user_id
  → Service.Publish 用它作为 author_id
```

## 先确定发布业务规则

| 字段 | 谁提供 | 规则 | 原因 |
| --- | --- | --- | --- |
| `title` | 调用方 JSON | 去空格后 1–100 个字符 | 视频需要展示标题 |
| `playback_url` | 调用方 JSON | 带主机名的 HTTP/HTTPS URL | 当前阶段只登记可播放地址 |
| `author_id` | 后端从 Token 取得 | 调用方不能指定 | 防止伪造别人的作者身份 |
| `created_at` | Gorm/数据库 | 创建时记录 | 以后列表和推荐流需要排序 |

## 业务过程和当前代码放在一起看

### 1. 调用方保存登录 Token 并随发布请求发送

业务上，用户先登录，调用方把响应中的 `access_token` 保存起来。此阶段用 PowerShell 变量模拟调用方：

```powershell
$login = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18080/users/login" -ContentType "application/json" -Body '{"username":"clean_user_01","password":"passw0rd!"}'
$accessToken = $login.access_token
```

`$accessToken` 是调用方保存 Token 的地方，不属于后端任何 Go 文件。发布请求需要带：

```text
Authorization: Bearer <accessToken 的值>
```

登录时的签发逻辑在 [internal/auth/jwt.go](../internal/auth/jwt.go) 第 28 行 `Issue`；随后请求的验证逻辑在同文件第 46 行 `Parse`。

### 2. Router 规定发布接口属于受保护路由组

业务上，发布视频必须知道作者是谁，所以 `POST /videos` 不能是游客接口。

代码在 [internal/httpapi/router.go](../internal/httpapi/router.go) 第 14 行 `NewRouter`：

- 第 20 行创建 `protected` 路由组。
- 第 21 行把认证中间件 `middleware.RequireUser(tokens)` 挂到整个组。
- 第 23 行注册 `POST /videos`，交给 `videos.Publish`。

因此 `POST /videos` 是 Router 的职责；[internal/video/handler.go](../internal/video/handler.go) 不再出现 `router.POST` 或 `router.Group`。

### 3. HTTP 认证中间件验证 Token，并临时保存作者 ID

业务上，服务端不能相信 JSON 中的 `author_id`，甚至请求也不需要传它。唯一可信的作者身份来自经过签名验证的 Token。

**中间件是谁：**[internal/httpapi/middleware/auth.go](../internal/httpapi/middleware/auth.go) 第 14 行的 `RequireUser`。它在 `video.Handler.Publish` 之前执行：

1. 读取 `Authorization`，确认它以 `Bearer ` 开头。
2. 调用 [internal/auth/jwt.go](../internal/auth/jwt.go) 第 46 行 `Parse`，用服务端 `jwt_secret` 验证签名、算法和过期时间。被篡改、伪造或过期的 Token 都失败。
3. 成功后，将 Token Claims 内的 `user_id` 写入 Gin 本次请求上下文，调用 `c.Next()`。
4. 失败则直接返回 401，视频 Handler、Service 和数据库写入都不会发生。

第 32 行的 `CurrentUserID` 用于让后续 Handler 安全读取这个已经验证过的 ID。

### 4. Video Handler 读取 JSON 和当前用户 ID

业务上，认证完成后才读取视频资料，并把“当前用户是谁”和“视频写什么”分开：身份来自 Token，标题和地址来自 JSON。

代码在 [internal/video/handler.go](../internal/video/handler.go) 第 23 行的 `Publish`：

- 调用 `middleware.CurrentUserID`，得到中间件写入的作者 ID。
- 用 `ShouldBindJSON` 读取 `title`、`playback_url`；无效 JSON 返回 400。
- 调用 `video.Service.Publish`；成功返回 201，规则不合格返回 400。

即使请求体额外加入 `"author_id":999`，`PublishRequest` 没有这个字段且 Handler 不读取它，最终作者仍是 Token 中的用户。

### 5. Service 判断视频是否能发布

业务上，空标题、过长标题和非 HTTP/HTTPS 地址不应成为视频记录。

代码在 [internal/video/service.go](../internal/video/service.go) 第 26 行的 `Publish`：

1. 去掉标题两端空格，检查长度。
2. 调用同文件第 43 行的 `validPlaybackURL`，检查 URL 的协议和主机名。
3. 用从 Token 得到的 `authorID` 创建 `Video`，绝不使用调用方传来的作者 ID。

### 6. Repository 写入 videos 表并返回新视频

业务上，发布成功代表数据库出现一条以后可进入视频流的记录。

- [internal/video/repository.go](../internal/video/repository.go) 第 20 行 `Create` 用 Gorm 插入数据。
- [internal/video/entity.go](../internal/video/entity.go) 第 6 行的 `Video` 定义表字段；`AuthorID` 有索引，后续查询某作者的视频会用到。
- [cmd/api/main.go](../cmd/api/main.go) 第 32 行启动时迁移 `videos` 表；第 37 行组装视频 Handler；第 38 行交给 Router。

调用图：

```text
PowerShell 变量 $accessToken
  → Authorization: Bearer <token>
  → httpapi/router.go：POST /videos 在 protected 组
  → httpapi/middleware/auth.go：RequireUser
  → auth/jwt.go：Parse
  → video/handler.go：Publish
  → video/service.go：Publish
  → video/repository.go：Create
  → videos 表
  → 201 Created + Video JSON
```

## 手动验收

启动服务：

```powershell
.\run.ps1
```

登录并保存 Token：

```powershell
$login = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18080/users/login" -ContentType "application/json" -Body '{"username":"clean_user_01","password":"passw0rd!"}'
$accessToken = $login.access_token
```

带 Token 发布：

```powershell
curl.exe -i -X POST "http://127.0.0.1:18080/videos" -H "Content-Type: application/json" -H "Authorization: Bearer $accessToken" -d '{\"title\":\"我的第一条视频\",\"playback_url\":\"https://media.example.com/first.mp4\"}'
```

预期 `201 Created`，响应中 `author_id` 应是登录用户的 ID。去掉 `Authorization` 后重试，预期 `401 Unauthorized`；把 `playback_url` 改成 `ftp://media.example.com/first.mp4`，预期 `400 Bad Request`。