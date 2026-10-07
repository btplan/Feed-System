# 第九课：让登录凭证有明确的验证规则

业务链还是“登录 → 获取 Token → 携带 Token 查询自己的资料”。这次补齐它的验证规则，属于 plan.md 阶段 A 的第一课；不是整个认证系统已经达到最终验收。

本课会让旧 Token 失效，请重新登录。新访问令牌有效期从 24 小时改为 15 分钟；暂时过期后重新登录，自动续期和退出撤销将在阶段 F 设计。原有用户、密码、视频数据不变，不需要重新注册已有用户。

## 1. 先用门票理解业务规则

假设一个园区有游乐场和员工办公室。门票即使盖了真实的章，也不意味着可以进入办公室。门口需要检查：谁盖的章、允许进入哪一处、有效到什么时候。

项目同理：签名有效只是一个条件，还要检查 Token 的签发者、用途和时间。不能让请求自己决定后端接受哪种算法，也不能让缺少过期时间的凭证一直可用。

另一个概念是环境变量。独立例子：

```powershell
$env:GREETING = "hello"
```

这会给当前 PowerShell 设置一个环境变量。它启动的 Go 进程可以通过 os.Getenv("GREETING") 读取。YAML 不会自动读取环境变量；需要 Go 代码显式读取。

本课用环境变量注入服务端签名密钥，公开配置文件只保存端口和数据库路径。环境变量不是加密容器；部署时应由受控的密钥管理或运行环境注入，不打印密钥。

## 2. 先设计数据：哪些属于配置，哪些属于 Token

打开 [jwt.go](../internal/auth/jwt.go) 第 23 行的 Claims，和 [config.go](../internal/config/config.go) 第 14 行的 Config。

| 名称 | 所在位置 / Go 类型 | 来源与规则 | 作用 |
| --- | --- | --- | --- |
| JWTSecret | Config.Auth / string；TokenManager 内转为 []byte | 环境变量解码后的至少 32 字节密钥 | 服务端签名与验签，不放入 Token 或响应 |
| user_id | Claims.UserID / int64 | 已通过密码验证的用户，必须大于 0 | 后续识别当前用户 |
| username | Claims.Username / string | 登录查到的用户，不能空白 | 身份辅助信息，不作为权限判断的依据 |
| iss | RegisteredClaims.Issuer / string | 本服务固定签发 clipflow | 约束预期签发者 |
| aud | RegisteredClaims.Audience / ClaimStrings | 包含 clipflow-api | 约束预期接收 API，类型是字符串列表 |
| iat | RegisteredClaims.IssuedAt / *jwt.NumericDate | 签发时生成，必须存在且不在未来 | 表示签发时间 |
| exp | RegisteredClaims.ExpiresAt / *jwt.NumericDate | 必须存在，晚于 iat，最多相差 15 分钟 | 到达这个时间后拒绝使用 |

NumericDate 在 JSON 中表示 Unix 时间，即从 1970-01-01 UTC 起经过的秒数。指针为 nil 表示没有该声明，和一个实际的时间值不同。

alg 是 JWT 头部字段，本课只接受 HS256；它不属于 Claims。iss、aud 本身也不是秘密，无法替代签名验证。持有同一签名密钥的程序具有签发能力，因此各环境应使用各自密钥。

这次没有新数据库表、字段或关系：Claims 是 Token 的内容，Config 是进程配置。只有在设计服务端会话、撤销记录时，才需要另行讨论持久化结构。

## 3. 构建顺序：配置 → 签发 → 验证 → 业务链回归

### 第一步：移除 YAML 的公开示例密钥

[configs/local.yaml](../configs/local.yaml) 保留服务地址和数据库路径，删除 auth.jwt_secret。代码位置：[config.go](../internal/config/config.go) 第 27 行 Load。

- 第 29 行读取 YAML。
- 第 33–36 行启用 KnownFields：字段拼错会报错；旧 auth.jwt_secret 也不能继续作为配置输入。
- 第 41 行读取 CLIPFLOW_JWT_SECRET 并 Base64 解码。
- 第 42–44 行检查格式和解码后的长度；不满足就返回错误，main 终止启动。
- 第 45 行将解码结果保存到 cfg.Auth.JWTSecret，第 46 行返回 cfg。

Base64 只是把随机字节表示成适合传递的文本，不是加密。长度检查不能判断密钥是否真正随机；请按下文用密码学随机数生成，不能随手重复一个字符凑长度。

[main.go](../cmd/api/main.go) 第 16 行接住 Load 的结果，第 37 行调用 NewTokenManager，将密钥交给签发和验证共用的对象。TokenManager 的构造函数接收已准备的密钥；实际启动入口通过 Load 完成格式与长度验证。

### 第二步：登录签发时写入约定声明

[账号 service.go](../internal/account/service.go) 第 46 行在密码验证成功后调用 tokens.Issue。

进入 [jwt.go](../internal/auth/jwt.go) 第 35 行 Issue：

1. 第 36–38 行确认用户身份不是非法输入。
2. 第 39 行只取得一次当前时间 now。
3. 第 40–49 行构造 Claims，第 44、45 行写入 iss 和 aud；第 46 行 exp=now+15分钟，第 47 行 iat=now。
4. 第 50 行明确用 HS256 构造 Token，第 51 行用服务端密钥签名。
5. 第 55 行返回字符串，回到账号 Service 第 46 行；Service 组装 LoginResponse，回到 [账号 handler.go](../internal/account/handler.go) 的 Login，最终第 38 行写出登录 JSON。

用户名和时间只是声明内容；签名才保护这些内容不被随意修改。JSON 字段名称 access_token、token_type、user 沿用原接口。

### 第三步：请求到来时按同一规则验证

调用方在请求头带 Authorization: Bearer Token，进入 [认证中间件](../internal/httpapi/middleware/auth.go) 第 15 行 RequireUser。

第 23 行调用 tokens.Parse，先跟到 [jwt.go](../internal/auth/jwt.go) 第 59 行：

- 第 62 行调用库函数 ParseWithClaims。它解析 Token、检查允许算法、验证签名并校验声明。此处会调用传入的匿名函数取密钥；匿名函数第 63 行 return 只是把密钥交给 JWT 库，不是将密钥交给客户端。
- 第 64 行 WithValidMethods 只接受 HS256，其他 HMAC 算法也不自动放行。
- 第 65 行 WithExpirationRequired 强制要求 exp；WithIssuedAt 检查已提供的 iat 是否在未来。
- 第 66 行要求 iss 正确、aud 包含本服务用途，两者缺失也失败。

注意 WithIssuedAt 不负责强制存在 iat，因此还需要下一步自定义校验。

### 第四步：JWT 库调用 Claims.Validate，随后回到 Parse

[jwt.go](../internal/auth/jwt.go) 第 77 行 Validate 是 JWT 库支持的自定义验证方法。本项目给 Claims 定义该方法后，库在声明验证流程中自动调用它，所以中间件里不会出现手写的 claims.Validate()。

- 第 78–80 行检查用户 ID 和用户名。
- 第 81–83 行拒绝缺失 iat 或 exp。
- 第 84 行计算 exp 与 iat 的差值，第 85–87 行拒绝不大于零或超过 15 分钟的有效期。
- 第 88 行返回 nil 表示这部分检查通过，控制权回到 JWT 库，再回到 ParseWithClaims 的调用处。

Parse 第 67 行接着检查库返回的 err；第 70 行检查 token.Valid；第 73 行成功返回 Claims 给中间件第 23 行。

若验证失败，中间件返回统一的 401，具体失败理由不会直接暴露给调用方。成功则中间件第 32 行把 user_id 放入请求上下文，继续执行个人资料 Handler，查询数据库后返回资料。

```text
服务启动：YAML + 环境变量 → config.Load → TokenManager

登录：账号 Service → Issue → 带 iss/aud/iat/exp 的 JWT → 登录响应

查询资料：RequireUser → Parse
  → JWT 库检查算法、签名、标准声明
  → Claims.Validate 检查身份和必需时间
  ← 回到 Parse，返回 Claims
← 回到 RequireUser，保存 user_id
→ Profile Handler → 查询用户 → 200 JSON
```

## 4. 自己启动和验收

先在旧服务终端 Ctrl+C 停止服务。打开项目根目录的 PowerShell，按下面步骤生成本地开发密钥，不要把密钥值复制进笔记、YAML 或聊天。

每行含义：准备 32 字节空间；创建安全随机数生成器；填充随机字节；释放生成器；编码后放进当前终端环境变量。此命令不会显示密钥：

```powershell
$secretBytes = New-Object byte[] 32
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($secretBytes)
$rng.Dispose()
$env:CLIPFLOW_JWT_SECRET = [Convert]::ToBase64String($secretBytes)
.\run.ps1
```

在同一终端中 Ctrl+C 后再次运行 run.ps1，可复用现有环境变量。关闭终端后，这段命令没有为你持久保存密钥；新开终端重新生成会使之前 Token 全部失效。部署时必须稳定注入受控密钥，不能每次启动随机换一把。阶段 F 再设计密钥轮换和会话生命周期。

另开一个 PowerShell 做客户端，不需要设置服务端密钥。使用自己已注册的账号；这里的 $accessToken 仍是登录响应，不是服务端密钥：

```powershell
$login = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18080/users/login" -ContentType "application/json" -Body '{"username":"clean_user_01","password":"passw0rd!"}'
$accessToken = $login.access_token
curl.exe -i "http://127.0.0.1:18080/users/me" -H "Authorization: Bearer $accessToken"
```

预期 200，返回当前用户资料。-i 显示响应状态和头，-H 设置请求头。等待 Token 过期再用同一个变量请求会得到 401；重新登录获取新 Token 后恢复成功。

无效 Token 的即时验证：

```powershell
curl.exe -i "http://127.0.0.1:18080/users/me" -H "Authorization: Bearer invalid"
```

预期 401。本课不修改数据库结构或视频数据；可用 SQLite 工具比较登录前后 users 的 id、username、password_hash，确认此改造没有重置账号。不要把密码哈希复制到外部。

## 5. 验证证据与剩余工作

[jwt_test.go](../internal/auth/jwt_test.go) 第 17 行通过真实认证中间件验证：正常令牌、缺失/非法 Token、错误密钥、HS384、无签名、缺少时间声明、过期、未来签发、倒置或过长时间、缺少或错误 iss/aud、非法用户 ID、空用户名。

[config_test.go](../internal/config/config_test.go) 第 12、30 行覆盖正常环境密钥及缺失、错误格式、过短密钥。测试使用独立固定测试值，不依赖你终端的实际密钥。

go test ./...、go vet ./...、构建已通过。另在隔离目录、临时随机端口与独立 SQLite 数据库实际运行了注册 → 登录 → 个人资料，成功返回用户；无效 Token 返回 401。未改动现有学习数据库。

当前未提供刷新令牌、退出撤销或多密钥平滑轮换；服务端时钟需要准确，本课时间校验不额外放宽。请求大小限制、超时、错误规范等尚未补齐。按 2026-10-08 的新计划，先增加评论等 Demo 业务，不把这些改进作为下一课的前置任务；本课也不代表整个系统已经具备上线条件。

预测题：你在“客户端终端”设置另一把 CLIPFLOW_JWT_SECRET，但不重启服务端，再用原来的有效 Token 查询资料，会因此失效吗？请沿 config.Load 的运行时机解释。
