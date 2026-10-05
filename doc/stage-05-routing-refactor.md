# 阶段 5：把路由、认证和业务处理分开

本阶段没有增加接口，也没有改变数据库表、请求 JSON 或响应 JSON。它做的是一次结构重构：让“请求应该去哪里”和“请求到了以后做什么”不再混在同一个文件。

## 先从一次发布视频的业务过程理解为什么要拆

用户发布视频时，业务事实依然是：用户带 Token 请求 `POST /videos`；服务端确认身份；读取标题和播放地址；检查规则；写入 `videos` 表；返回新视频。

但这条链里其实有三种完全不同的问题：

| 问题 | 例子 | 应该由谁回答 |
| --- | --- | --- |
| 请求要去哪里 | `POST /videos` 应该调用哪个函数？是否必须登录？ | Router |
| 调用方是谁 | Token 是否有效？其中的用户 ID 是什么？ | HTTP 中间件 |
| 这条视频能否发布 | 标题和播放地址是否符合规则？如何写数据库？ | video 业务模块 |

以前这些问题的一部分都在 `video/handler.go`。小项目能运行，但当 URL、多种中间件和业务越来越多时，找一个接口会变成“先猜它藏在哪个业务文件里”。重构后，每个问题都有固定位置。

## 业务过程和代码位置

### 1. 服务启动时只组装对象，不决定每个 URL 的细节

业务上，程序启动必须准备数据库、JWT 密钥、账号 Handler 和视频 Handler；但启动程序不应该写每一个 URL 的规则。

代码在 [cmd/api/main.go](../cmd/api/main.go) 第 15 行的 `main`：

1. 第 29–34 行迁移 `users` 与 `videos` 表；这是启动准备。
2. 第 35 行创建 TokenManager；登录签发 Token 和认证验证 Token 都使用它。
3. 第 36–37 行把 Repository → Service → Handler 组装成账号和视频 Handler。
4. 第 38 行把已组装的 Handler 交给 `httpapi.NewRouter`；从这里开始，URL 交给 HTTP 层管理。

这叫**依赖组装**：`main` 知道怎样把对象接起来，但不参与注册、发布等业务判断。

### 2. Router 集中回答“URL 去哪里、是否必须登录”

业务上，注册和登录允许游客调用；查看自己的资料和发布视频必须先登录。

代码在 [internal/httpapi/router.go](../internal/httpapi/router.go) 第 14 行的 `NewRouter`：

- 第 16 行：`GET /healthz` 是公开健康检查。
- 第 17–18 行：`POST /users/register`、`POST /users/login` 是公开接口，直接交给账号 Handler。
- 第 20–23 行：创建 `protected` 路由组，统一挂上认证中间件；组里的 `GET /users/me` 和 `POST /videos` 都必须认证成功才能到达对应 Handler。

因此你之前问的“`POST /videos` 是不是 Router 的事”，答案现在落实为：**是，它只出现在 `router.go`。**

### 3. 认证中间件在进入业务 Handler 前确认身份

业务上，访问 `/users/me` 或发布视频前，系统要确认 Token 是本服务签发、没被修改且未过期。

代码在 [internal/httpapi/middleware/auth.go](../internal/httpapi/middleware/auth.go) 第 14 行的 `RequireUser`。它是 Gin 中间件，不属于 JWT 本身，也不属于视频业务：

1. 读取请求头 `Authorization`，确认有 `Bearer ` 前缀。
2. 调用纯 JWT 工具 [internal/auth/jwt.go](../internal/auth/jwt.go) 第 46 行的 `Parse` 验证签名和过期时间。
3. 成功后把 `user_id` 放进 Gin 的本次请求上下文，调用 `c.Next()`；失败则返回 401，业务 Handler 不会执行。

同文件第 32 行的 `CurrentUserID` 让 Handler 读取这个已验证的身份。认证中间件现在放在 `internal/httpapi/middleware/`，因为它直接依赖 Gin 的 `Context`、HTTP 请求头和 HTTP 状态码；而 `internal/auth/jwt.go` 只处理 JWT，不依赖 Gin。

### 4. Handler 只把 HTTP 输入交给所属业务

业务上，身份已经确认后，视频 Handler 只需要读取 JSON、调用发布规则、把结果变成 HTTP 响应。

代码在 [internal/video/handler.go](../internal/video/handler.go) 第 23 行的 `Publish`。它通过 `middleware.CurrentUserID` 得到作者 ID，读取 `title` 和 `playback_url`，然后调用 `video.Service.Publish`。

账号 Handler 同样只处理账号接口：

- [internal/account/handler.go](../internal/account/handler.go) 第 23 行：登录。
- 同文件第 42 行：读取当前用户资料。
- 同文件第 61 行：注册。

注意：这些 Handler 文件现在没有 `router.POST(...)`、`router.GET(...)` 或 `router.Group(...)`。这就是重构真正带来的边界。

### 5. Service 与 Repository 的职责没有改变

业务规则仍在 [internal/video/service.go](../internal/video/service.go) 第 26 行；数据库写入仍在 [internal/video/repository.go](../internal/video/repository.go) 第 20 行。重构没有移动它们，因为“视频能不能发布”和“怎样写 videos 表”本来就属于视频业务。

完整请求图现在是：

```text
cmd/api/main.go：组装对象
  → httpapi/router.go：POST /videos 属于受保护路由组
  → httpapi/middleware/auth.go：验证 Bearer Token，写入当前 user_id
  → video/handler.go：读取 JSON 与当前 user_id
  → video/service.go：校验标题和播放地址
  → video/repository.go：写入 videos 表
  → 201 Created
```

## 重构后的目录职责

```text
cmd/api/                         启动与依赖组装
internal/httpapi/router.go       URL、路由组、公开/受保护接口
internal/httpapi/middleware/     依赖 Gin 和 HTTP 的通用中间件
internal/auth/                   不依赖 Gin 的 JWT 能力
internal/account/                账号业务
internal/video/                  视频业务
internal/config/                 YAML 配置读取
internal/database/               Gorm/SQLite 连接
```

## 验证结果

已完成：

- `go test ./...`
- `go vet ./...`
- `go build -o .run\clipflow-refactor.exe ./cmd/api`
- 实际请求：`GET /healthz` 返回 `ok`；登录后 `GET /users/me` 返回 `clean_user_01`；带同一 Token 发布视频，返回的 `author_id=1`。

## 本阶段练习

不用改代码，先定位：如果下一个接口是游客可浏览的 `GET /videos`，你会先打开哪个文件加 URL？如果它不需要 Token，为什么不应该放进 `protected` 路由组？
