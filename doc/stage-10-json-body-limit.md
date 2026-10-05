# 第十课：给现有 JSON 接口加上请求大小边界

本轮是一次具体的迭代：注册、登录和发布视频原本能正常工作，但没有明确限制请求体大小。现在增加 64 KiB 上限，超限返回 413。业务字段规则和数据库结构沿用前面课程。

我们不会为了先达到某个架构标签而一次性搭齐所有组件。后续继续从业务功能、实际问题和验证结果决定改进顺序；plan.md 中的工程能力是迭代方向，不是开始做业务前必须全部具备的前置条件。

## 1. 从一个独立例子理解“读取上限”

假设收件箱只收最多 10 页材料。检查信封上写的“共 5 页”不够，实际打开后可能有 100 页。必须在读取材料时限制数量。

HTTP 的 Content-Length 类似信封上填写的长度。有些请求根本不声明长度，因此仅检查请求头不能代替限制实际读取。

本课用标准库 http.MaxBytesReader 包装请求体，再调用 io.ReadAll。ReadAll 读取的是受限的 reader，而不是直接无限读取原始请求体；多出上限时得到 *http.MaxBytesError。

KiB 是字节单位：64 KiB = 64 × 1024 = 65536 字节。它不是 65536 个汉字；UTF-8 的一个汉字通常占多个字节。JSON 的字段名、引号、空格也计入长度。

## 2. 业务上希望看到什么

| 输入 | 结果 | 会进入 Service 吗 |
| --- | --- | --- |
| 合法的小 JSON | 沿用原业务结果 | 会 |
| 超过 65536 字节 | 413 + error | 不会 |
| 小于上限但不是合法 JSON | 400 + error | 不会 |
| 一次发送两个 JSON 对象 | 400 + error | 不会 |
| 发布视频没有有效 Token | 原有认证先返回 401 | 不会 |

64 KiB 是当前仅包含用户名、密码、标题、播放地址的 JSON 接口的初始边界，不是永远不变的标准。将来文件上传使用独立的上传策略，不应该把真实视频塞进这些 JSON 接口里，也不能直接沿用 64 KiB 文件上限。

## 3. 先决定数据和代码放在哪里

本课不增加业务结构体、字段、表或关系：限制属于 HTTP 输入边界，不需要写进 users 或 videos。

新增 [request/json.go](../internal/httpapi/request/json.go) 第 13 行 MaxJSONBytes，类型 int64，以字节为单位；它是后端策略，客户端不能通过请求参数修改。

统一读取函数放在 internal/httpapi/request 包，因为账号和视频 Handler 都需要它。它既不依赖 account，也不依赖 video，因此可以被两个业务包共同调用，不产生循环导入。它不是 Router，也不是挂在路由组上的中间件；它是 Handler 主动调用的 HTTP 辅助函数。

函数签名在同文件第 16 行：

```go
func BindJSON(c *gin.Context, target any, invalidMessage string) bool
```

- c：这次请求和响应的操作入口。
- target：接收 JSON 的结构体指针，例如 &req；any 允许传入注册请求或视频发布请求等不同类型。
- invalidMessage：原接口使用的错误提示，不把 JSON 原文回传。
- bool 返回值：true 表示可以继续业务；false 表示已经写出错误响应，调用者应立即 return。

## 4. 构建顺序：先写共用读取函数，再接入现有 Handler

1. [request/json.go](../internal/httpapi/request/json.go) 第 16 行：实现上限和错误分类。
2. [account/handler.go](../internal/account/handler.go) 第 27、65 行：登录和注册接入共用函数。
3. [video/handler.go](../internal/video/handler.go) 第 50 行：视频发布接入共用函数。
4. [json_body_test.go](../internal/httpapi/json_body_test.go) 第 17、58 行：检查长度边界和正式路由。

路由不变，也不需要重新建表。下面用注册请求串起实际运行顺序。

## 5. 从注册 Handler 进入读取函数，再回到 Handler

打开 [account/handler.go](../internal/account/handler.go) 第 63 行 Register。第 64 行准备 RegisterRequest；第 65 行调用 request.BindJSON(c, &req, ...)。

进入 [request/json.go](../internal/httpapi/request/json.go) 第 16 行后：

1. 第 17 行包装 c.Request.Body，限制读取长度。它不等于此时已经读完内容。
2. 第 18 行 defer reader.Close() 表示函数退出时关闭 reader；第 19 行才真正读取完整请求体。
3. 第 20–24 行用 errors.As 判断是否为超限错误；是则返回 413，并 return false。errors.As 用于识别错误类型，不用匹配错误文字。
4. 第 25–28 行：其他读取错误返回 400 和 false。
5. 第 29–32 行 json.Unmarshal 将完整 JSON 放入 target；格式错误返回 400 和 false。要求整段内容是一个 JSON 值，所以两个对象连在一起也失败。
6. 第 33–36 行调用 Gin 的结构体验证器，保留原先 ShouldBindJSON 对 binding 标签的检查。业务层的用户名长度、密码和 URL 规则仍由 Service 处理。
7. 第 37 行 return true，回到账号 Handler 第 65 行。

如果返回 false，Handler 的 if 条件成立，执行 return。AbortWithStatusJSON 会终止 Gin 后续处理链，但不会自动停止当前 Go 函数，所以调用者的 return 仍然必须写。

如果返回 true，Handler 跳过错误分支，继续原来的 h.service.Register。Service → Repository 创建用户后返回，Handler 最终返回 201。

```text
POST /users/register
→ Router 找到 Register
→ Handler 准备 req，调用 request.BindJSON
  → 限量读取完整 body
  → 超限：写出 413，返回 false
  → 合法：解析到 req，返回 true
← 回到 Handler 的 if
  → false：Handler return，结束
  → true：Service → Repository → 返回注册结果 → 201
```

登录第 27 行、发布视频第 50 行采用相同控制流程；无需每处重写长度判断。

## 6. 手动验收

先按第九课设置服务端环境密钥，启动 .\run.ps1。若已有旧服务运行，先 Ctrl+C，再启动更新后的版本。在另一个 PowerShell 作为客户端执行以下请求。

先观察一个正常的小请求，curl.exe 默认 GET，-i 显示状态码：

```powershell
curl.exe -i "http://127.0.0.1:18080/videos?limit=1"
```

应仍为 200；这次改造仅接入三个 JSON 写入/登录接口，不影响列表读取。

为了避免在终端粘贴巨大字符串和转义引号，先生成一个临时 JSON 文件。下面用 70000 个字母作为用户名；这是故意构造超限请求，不是合法注册资料：

```powershell
$bodyPath = Join-Path $env:TEMP 'feed-system-large-request.json'
$payload = @{ username = ('a' * 70000); password = 'passw0rd!' } | ConvertTo-Json -Compress
[IO.File]::WriteAllText($bodyPath, $payload, (New-Object System.Text.UTF8Encoding($false)))
curl.exe -i -X POST "http://127.0.0.1:18080/users/register" -H "Content-Type: application/json" --data-binary "@$bodyPath"
```

$bodyPath 指向临时文件；ConvertTo-Json 将字段转为合法 JSON；UTF8Encoding(false) 避免写入 BOM；--data-binary @文件路径 让 curl 原样发送文件内容。预期 413，error 为 JSON request body exceeds 65536 bytes。

为什么不是“用户名太长”的 400？因为请求整体在进入注册 Service 之前就被大小限制拦住。

再生成短用户名请求，发送同样的 curl 命令：

```powershell
$payload = @{ username = 'ab'; password = 'passw0rd!' } | ConvertTo-Json -Compress
[IO.File]::WriteAllText($bodyPath, $payload, (New-Object System.Text.UTF8Encoding($false)))
curl.exe -i -X POST "http://127.0.0.1:18080/users/register" -H "Content-Type: application/json" --data-binary "@$bodyPath"
```

预期 400，此时已经通过 JSON 读取，进入 Service 后因用户名不足 3 个字符被拒绝。

用 SQLite 工具在操作前后执行 SELECT COUNT(*) FROM users;，两次失败请求均不应增加用户记录。

## 7. 验证范围与下一次迭代

自动测试覆盖正常请求、恰好 65536 字节、超过一字节、未声明长度、非法 JSON、空体、必填标签校验和两个对象；正式 Router 测试覆盖注册、登录、发布的超限路径，并通过未配置 Service 确认没有误入业务层。

本课限制的是单次读取量，不是连接持续时间、请求频率或全部并发请求的总内存；慢请求超时和请求限流是不同问题，应在相应迭代单独实现。JSON 解析也有额外内存开销，64 KiB 不等于每个请求只占 64 KiB 内存。

练习：一个 70 KiB 的请求，其 username 恰好是合法的 alice，但附带一个很长的无用字段，会先返回 413 还是进入注册？沿 Handler 的调用和返回位置说明。