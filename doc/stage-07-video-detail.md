# 第七课：从列表选中一条视频，查看详情

用户打开列表，看到“海边日落”，点击进入详情。调用方从这条列表数据中取出 `id`，再请求 `GET /videos/5`（假设该视频 ID 为 5）。后端按编号查数据库，返回这条视频的最新资料。

当前没有前端。我们用 curl 模拟“点击”：先请求列表，找到一个真实 ID，再把它填到详情 URL。列表已经含有标题和播放地址，因此并非播放前必须再查详情；独立详情接口还支持分享链接、刷新页面后按 ID 重新加载资料。

本课返回的是视频资料 JSON。真正播放时，播放器使用 JSON 中的 `playback_url` 获取视频文件。之前的 example.com 示例地址只是学习用字符串，不能保证能播放。

## 新概念先用最小例子理解

图书馆接口 `GET /books/8` 表示查询编号 8 的书。Gin 路由写成 `/books/:id`，其中 `:id` 是占位符：

```go
rawID := c.Param("id") // 得到字符串 "8"
```

这是“路径参数”。第六课的 `?limit=2&offset=0` 是“查询参数”，通过 `c.Query` 读取。它们的位置、读取方式和用途都不同：

| 请求 | 参数含义 | Gin 读取方式 |
| --- | --- | --- |
| /videos/5 | 选定编号为 5 的视频 | c.Param("id") |
| /videos?limit=2 | 列表最多显示 2 条 | c.Query("limit") |

另一个要区分的概念是“函数返回”和“HTTP 响应”。普通 Go 函数的 `return video, err` 只把值交还给调用它的 Go 代码；只有 Handler 执行 `c.JSON(...)`，才把数据写进 HTTP 响应交给 curl 或前端。

## 先确定结果应该是什么

| 情况 | HTTP 结果 | 原因 |
| --- | --- | --- |
| 正整数编号且视频存在 | 200 + 视频 JSON | 查到了具体视频 |
| 正整数编号但视频不存在 | 404 + error | 请求有效，但没有目标记录 |
| abc、0、负数、超过 int64 上限 | 400 + error | 视频编号不合法 |
| 数据库意外故障 | 500 + error | 服务无法完成查询 |

详情是公开资料，所以允许游客查询，不经过 RequireUser，也不需要从 Token 取用户 ID。这里的视频 ID 和作者 ID 是两个不同的编号。

## 先补齐数据设计：Video 为什么有这些字段

前面直接复用 Video 跳过了设计过程。这里回看第四课的数据设计；本课没有新增表或字段。

### 先用一个独立例子理解实体和关系

图书馆需要保存“书名”和“出版社”。一本书属于一家出版社，一家出版社出版多本书，这叫一对多。可以建立 publishers 和 books 两张表，在 books 中保存 publisher_id 表示对应哪家出版社。

结构体是 Go 程序内组织数据的方式；数据库表是持久保存记录的方式。结构体不会仅因被声明就自动建表，需要代码调用迁移。两者通过 Gorm 的映射连接起来。

### 第一步：从业务问题推导需要保存的数据

发布一条视频后，我们需要回答：

1. “到底是哪条视频？”——需要独立的视频编号 ID。
2. “谁发布的？”——需要作者编号 AuthorID。
3. “页面显示什么标题？”——需要 Title。
4. “播放器去哪取视频？”——需要 PlaybackURL。
5. “什么时候发布，列表怎么按时间排序？”——需要 CreatedAt。

这些问题决定了字段，而不是先随意写一个结构体再找用途。

### 第二步：决定 users 和 videos 的关系

业务上，一个用户能发布多条视频，每条视频归一个作者，所以是 users 到 videos 的一对多关系：

```text
users：id=1，username=alice
  ← videos：id=5，author_id=1，title=海边日落
  ← videos：id=6，author_id=1，title=做饭记录
```

视频的 id=5 和作者的 id=1 是两回事。不能把作者 ID 当视频 ID；也不能给 author_id 加唯一约束，否则一个用户最多只能有一条视频。

当前代码只保存 AuthorID 并建立普通索引，**没有定义数据库外键约束**。index 只帮助查询，不会检查 users 中是否存在该作者。当前发布链从验证过的 Token 取得作者 ID，但不会再检查用户是否已被删除。这是当前实现的边界；将来做删除账号时需要处理关联记录和约束。

### 第三步：确定字段类型、值的来源和约束

打开 [视频 entity.go](../internal/video/entity.go) 第 6 行，看 Video；对照 [账号 entity.go](../internal/account/entity.go) 第 6 行的 User。

| Go 字段 / 数据库列 | 类型及含义 | 谁提供值 | 当前约束与设计理由 |
| --- | --- | --- | --- |
| ID / id | int64，视频编号 | 新记录由数据库生成 | primaryKey 标明主键，唯一识别视频；详情接口要求大于 0 |
| AuthorID / author_id | int64，作者编号 | 发布时从认证后的用户身份取得 | 与 User.ID 类型一致；not null 禁止 SQL NULL；index 用于按作者查视频 |
| Title / title | string，标题文字 | 请求 JSON | not null；Service 另外检查去空格后 1–100 个字符 |
| PlaybackURL / playback_url | string，地址文字 | 请求 JSON | not null；Service 检查 HTTP/HTTPS 格式，不验证远程文件实际存在 |
| CreatedAt / created_at | time.Time，创建时间 | Gorm 创建记录时自动填写 | 用于显示和排序；当前没有显式的 not null 标签 |

int64 是有符号 64 位整数，可表示 -9223372036854775808 到 9223372036854775807。编号不需要小数，因此使用整数；业务仅接受正数，Go 类型本身不会替你禁止负数。

新建 Video 尚未入库时，ID 的零值是 0；Gorm 创建记录后将生成的 ID 回填到结构体。因此“创建前暂时为 0”和“详情接口允许查询 0”是两回事。

AuthorID 的 not null 也不代表大于零：SQL NULL、数字 0、负数是不同的值。当前并未给数据库添加 author_id > 0 的 CHECK 约束。

第六课的 next_offset 虽然也是数字，但它表示跳过多少条记录，可以是 0；视频 id 表示记录身份，两者不能互换。

### 第四步：把设计写成模型，理解每一种标签

在 [视频 entity.go](../internal/video/entity.go) 第 7 行：

```go
ID int64 `gorm:"primaryKey" json:"id"`
```

ID 是 Go 字段名；int64 是 Go 类型；gorm 标签控制数据库映射；json 标签决定 HTTP JSON 字段名。当前默认命名规则把 Video 映射为 videos 表、AuthorID 映射为 author_id 列。

同文件第 15 行的 PublishRequest 只有 Title 和 PlaybackURL，因为客户端只负责这两项。它没有 ID、AuthorID 和 CreatedAt，避免把服务端生成的数据也当成客户端输入。

同文件第 21 行的 ListResponse 用于列表响应，保存 Items、NextOffset、HasMore。它不是数据库表。本项目只迁移 Video；详情目前直接返回 Video 作为 JSON。

### 第五步：迁移建表，然后才实现查询

[repository.go](../internal/video/repository.go) 第 38 行的 Migrate 调用 AutoMigrate(&Video{})。程序启动时 main 调用它，使数据库具备保存 Video 的表结构。Go 文件中的 type Video struct 声明本身不会执行建表。

新业务的完整构建顺序是：

```text
业务需要哪些数据
→ 确定实体及关系
→ 设计字段、类型、主键、约束、索引
→ 定义数据库模型和请求/响应结构体
→ 迁移数据库表
→ Repository 读写
→ Service 业务判断
→ Handler 收发 HTTP
→ Router 接入 URL
```

## 本课在已有数据模型上继续构建查询能力

本课复用现有 Video 结构和数据库连接，不建新表。代码实际按以下依赖顺序搭建：

1. [repository.go](../internal/video/repository.go) 第 43 行 `FindByID`：先实现按编号查数据库。
2. [service.go](../internal/video/service.go) 第 71 行 `Detail`：调用查询能力，把“查无记录”解释成“视频不存在”。
3. [handler.go](../internal/video/handler.go) 第 88 行 `Detail`：读取 URL 参数，调用业务，决定 HTTP 响应。
4. [router.go](../internal/httpapi/router.go) 第 20 行：把公开 URL 接到 Handler。

下面按请求实际执行的顺序看。特别注意进入一个函数后，会回到哪一行。

## 1. 用户选中视频：Router 把请求交给 Handler

调用方拿到列表里的 `items[0].id` 后，将它放进 URL。例如该值为 5，就请求 `GET /videos/5`。这个 5 是视频编号，不是 offset；列表排在第一位的视频 ID 不一定是 1。

打开 [router.go](../internal/httpapi/router.go)，看第 20 行：

```go
router.GET("/videos/:id", videos.Detail)
```

Gin 匹配路径，把字符串 "5" 放入路径参数 id，然后进入视频 Handler。路由使用 `router.GET`，因此不会经过 protected 组的认证中间件。

## 2. 先完整看一遍 Handler：读取编号 → 调用业务 → 发出响应

打开 [handler.go](../internal/video/handler.go)，从第 88 行的 `func (h *Handler) Detail` 开始：

- 第 89 行：`c.Param("id")` 取出字符串；`strconv.ParseInt(..., 10, 64)` 将它按十进制解析为 int64。无法转换或数字超出范围都会产生 err。
- 第 90–93 行：检查 err 和编号是否大于零；不合法则第 91 行写出 400，第 92 行 return 结束 Handler。数据库不会被调用。
- 第 94 行：`video, err := h.service.Detail(..., videoID)`。这里才进入 Service；等待它返回后，把两个结果接到局部变量 video 和 err。
- 第 95–103 行：检查刚才的结果。视频不存在用第 97 行返回 404；其他错误用第 100 行返回 500；成功用第 102 行 `c.JSON(http.StatusOK, video)` 返回 200 和视频对象。

这时先记住：**第 94 行是出去调用业务的位置，第 95 行是拿到结果后继续判断的位置，第 102 行才是成功响应的位置。**

## 3. 跟进 Handler 第 94 行：Service 如何解释查询结果

打开 [service.go](../internal/video/service.go) 第 71 行，注意这是 `func (s *Service) Detail`。它与 Handler 都叫 Detail，但接收者分别是 Service 和 Handler，是两个不同方法。

- 第 72 行调用 `s.repo.FindByID`。这会进入下一节的 Repository，等它返回。
- 第 73 行使用 `errors.Is` 判断是否为 Gorm 的“查无记录”错误。
- 如果查无记录，第 74 行返回 `Video{}, ErrVideoNotFound`。`Video{}` 是空结构体，伴随错误返回，并不代表找到了一条空视频。
- 其他情况下，第 76 行把查到的视频和 err 交回 Handler 第 94 行。成功时 err 为 nil；数据库故障时 err 不为 nil。

Service 不写 HTTP 状态码。它只说明“找到了”“不存在”或“查询失败”，由 Handler 决定怎样回复客户端。

## 4. 跟进 Service 第 72 行：Repository 查数据库

打开 [repository.go](../internal/video/repository.go) 第 43 行 `FindByID`：

- 第 44 行声明 `var video Video`，准备接收一条数据库记录。
- 第 45 行 `First(&video, videoID)` 按主键查询。传入 `&video`，是为了让 Gorm 把查到的字段填进这个变量。
- 同一行的 `.Error` 取得本次数据库操作的错误。
- 第 46 行 `return video, err`，回到 Service 第 72 行，继续它后面的错误判断。

相当于执行（以编号 5 为例）：

```sql
SELECT * FROM videos WHERE id = 5 LIMIT 1;
```

ID 来自解析后的整数，经 Gorm 传入查询，没有把 URL 原始字符串直接拼进 SQL。

## 5. 沿返回路径回到客户端

```text
GET /videos/5
  → Router 第 20 行：匹配 /videos/:id
  → Handler.Detail 第 89 行：解析编号
  → Handler 第 94 行调用 Service.Detail
    → Service 第 72 行调用 Repository.FindByID
      → Repository 第 45 行查询数据库
      ← Repository 第 46 行返回 Video 和 error
    ← Service 第 74 或 76 行返回业务结果
  → 回到 Handler 第 95 行判断结果
  → Handler 第 102 行发出 200 JSON（成功情况）
```

成功响应是一条视频对象，没有列表的 items、has_more、next_offset。示例值仅供理解：

```json
{
  "id": 5,
  "author_id": 1,
  "title": "海边日落",
  "playback_url": "https://media.example.com/sunset.mp4",
  "created_at": "2026-10-02T10:00:00+08:00"
}
```

id 标识这条视频；author_id 标识作者；title 是标题；playback_url 是播放文件地址；created_at 是创建时间。

## 手动验证：先找到真实 ID，再请求详情

如果旧服务还在运行，先在它的终端按 Ctrl+C，再运行下面命令。启动脚本会编译运行更新后的代码：

```powershell
.\run.ps1
```

另开 PowerShell。curl.exe 用于发送请求，-i 显示状态码和响应头；没有 -X 时默认 GET。先取一条列表数据：

```powershell
curl.exe -i "http://127.0.0.1:18080/videos?limit=1&offset=0"
```

从 items 中找到 id，把下面的 5 替换为它：

```powershell
curl.exe -i "http://127.0.0.1:18080/videos/5"
```

预期 200；详情的 id、title、author_id、playback_url 应和刚才那条列表记录一致。若 items 为空，先按第四课发布一条视频，再查询。

格式不合法的请求：

```powershell
curl.exe -i "http://127.0.0.1:18080/videos/abc"
```

预期 400。再选一个数据库中不存在的正整数 ID 请求详情，预期 404。使用 SQLite 工具打开 configs/local.yaml 指定的数据库文件，执行下列只读 SQL（同样替换 5）：

```sql
SELECT id, author_id, title, playback_url, created_at FROM videos WHERE id = 5;
```

将 SQL 查询结果与响应逐字段核对。GET 请求不会新增或修改视频记录。

## 进度与练习

助手已通过正式 Router + 独立 SQLite 数据库测试：匿名成功查询与字段一致性、404、非数字、零、负数和整数溢出的 400；go test、go vet 和构建通过。用户实际运行与理解验收待完成。

本课继续解决上一课的疑问：进入辅助函数后，必须回到调用它的函数继续执行，不能把整个业务链都当成在辅助函数里面。

练习：请求 /videos/abc 和一个不存在的正整数编号时，哪一次会执行 Repository.FindByID？请分别指出 Handler 中的返回位置。
