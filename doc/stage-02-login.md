# 阶段 2：用户登录与 JWT

本阶段完成完整登录链：用户提交用户名和密码，后端读取 users 表、比对 bcrypt 密码哈希，成功后签发 JWT。

## 业务规则

| 情况 | 返回 |
| --- | --- |
| 用户名和密码正确 | 200，用户资料和 access_token |
| 用户不存在或密码错误 | 401，统一错误信息 |
| JSON 格式错误 | 400 |

用户名不存在和密码错误都返回同一句 `username or password is incorrect`。这避免调用者借助响应判断某个用户名是否已经注册。

## 最小概念

bcrypt 哈希不能解密。登录的实际操作是：取出数据库中的 `password_hash`，然后调用
`bcrypt.CompareHashAndPassword(hash, inputPassword)`。

JWT 是经过签名的字符串。登录成功后，服务器把 user_id、username、签发时间和过期时间写入 Claims，
再用 `jwt_secret` 签名。当前 Token 有效期为 24 小时；下一条受保护业务会学习如何验证它。

## 先从业务逻辑理解登录

不要先把登录理解成“写一个 POST 接口”。它要完成的是：**证明这次请求确实来自一个已经注册的用户，并给这个用户一张以后可重复出示的身份凭证。**

业务过程按发生顺序是：

```text
用户提交用户名和密码
  → 系统按用户名找到已注册用户
  → 用输入密码比对数据库中的密码哈希
  → 成功：签发一张带用户身份的 Token
  → 调用方保存 Token，之后访问受保护功能时出示它
```

这里有四条构建原则：

1. **HTTP 只负责收发请求。** Handler 读取 JSON、返回状态码；它不关心 Gorm 查询和密码算法。
2. **业务判断集中在 Service。** “用户不存在或密码错误，都返回同一句话”是登录规则，应该在这里统一决定。
3. **数据库细节放进 Repository。** 登录只需要“按用户名找用户”，Service 不该知道具体 SQL 或 Gorm 写法。
4. **身份凭证交给单独的 TokenManager。** Service 只要求“为这个用户签发 Token”，不把 JWT 签名细节混进登录规则。

先确定这条业务链和职责边界，再实现代码。实现时需要反过来按依赖关系准备：先有配置中的密钥，才能写 TokenManager；先有查用户的 Repository，才能写 Service；最后才由 Handler 和 main 把它们接到 HTTP 路由上。
## 按从零构建的逻辑顺序阅读

点击每一项中的文件名打开代码，再按后面的行号查看。

1. [configs/local.yaml](../configs/local.yaml) 第 7 行：先看 `auth.jwt_secret`。密钥是签名材料，生产环境不能使用本地示例值。
2. [internal/config/config.go](../internal/config/config.go) 第 12 行 与 [internal/config/config.go](../internal/config/config.go) 第 25 行：看 Config 如何接收并检查 `jwt_secret`。
3. [internal/auth/jwt.go](../internal/auth/jwt.go) 第 11 行 与 [internal/auth/jwt.go](../internal/auth/jwt.go) 第 28 行：看 Claims 的内容、`NewTokenManager` 和 `Issue` 如何生成 Token。
4. [internal/account/entity.go](../internal/account/entity.go) 第 27 行：看 LoginRequest 与 LoginResponse。
5. [internal/account/repository.go](../internal/account/repository.go) 第 25 行：看 `FindByUsername` 用 Gorm 查询一行用户。
6. [internal/account/service.go](../internal/account/service.go) 第 32 行：看 Login 如何比对 bcrypt、调用 Issue，并统一错误。
7. [internal/account/handler.go](../internal/account/handler.go) 第 19 行 与 [internal/account/handler.go](../internal/account/handler.go) 第 28 行：看 `POST /users/login` 如何把业务结果变为 200、400、401。
8. [cmd/api/main.go](../cmd/api/main.go) 第 39 行：最后看如何装配数据库、TokenManager 与路由。
9. [internal/account/handler_test.go](../internal/account/handler_test.go) 第 62 行：最后看自动化测试验证了哪些结果。

这个顺序和从零实现的依赖顺序相同：配置提供密钥，令牌工具依赖密钥，登录业务依赖令牌工具，路由依赖登录业务，main 最后装配一切。

## 调用图

```text
POST /users/login
  → Handler.Login
  → Service.Login
  → Repository.FindByUsername
  → bcrypt.CompareHashAndPassword
  → TokenManager.Issue
  → 200 JSON + access_token
```

## 手动验收

先用注册接口创建一个新用户，再登录。当前端口来自 YAML，为 18080。

```powershell
curl.exe -i -X POST "http://127.0.0.1:18080/users/login" -H "Content-Type: application/json" -d '{\"username\":\"clean_user_01\",\"password\":\"passw0rd!\"}'
```

预期 `200 OK`。正文含 `access_token`、`token_type: "Bearer"` 和 user；没有 password_hash。

错误密码：

```powershell
curl.exe -i -X POST "http://127.0.0.1:18080/users/login" -H "Content-Type: application/json" -d '{\"username\":\"clean_user_01\",\"password\":\"wrongpass\"}'
```

预期 `401 Unauthorized`，且正文不说明用户名是否存在。
