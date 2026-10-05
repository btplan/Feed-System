# 第八课：点赞与取消点赞——从关系表设计到请求返回

用户看到视频后点亮爱心，表示“我喜欢这条视频”；再次操作可以取消。网络重试可能把同一请求发送两次，我们希望结果稳定：重复点赞仍只有一条记录，重复取消仍是未点赞。

本课接口：登录后 PUT /videos/:id/like 点赞，DELETE /videos/:id/like 取消。二者成功均返回 200 和操作后的状态、点赞总数。没有 JSON 请求体，也不能通过 JSON 指定用户。

## 1. 先从独立例子理解关系与约束

学生 1 选课程 5、6，学生 2 也选课程 5。这是“多对多”：学生表或课程表单独一个字段都装不下所有关系，因此用选课表存每一对编号。

| 学生编号 | 课程编号 |
| --- | --- |
| 1 | 5 |
| 1 | 6 |
| 2 | 5 |

不允许再插入第二条 (1,5)，但允许 (1,6) 和 (2,5)。约束的是“两列组合不能重复”，不是每一列单独不能重复。

点赞就是同一模型：把学生换成用户，把课程换成视频。

## 2. 先定业务数据，再写结构体

我们要记住“谁赞了哪条视频、什么时候赞的”，因此新增 video_likes 表：

```text
users.id ← video_likes.user_id
videos.id ← video_likes.video_id

users 一对多 video_likes
videos 一对多 video_likes
两张业务表通过关联表表达多对多
```

注意，video_likes.user_id 是点赞者，videos.author_id 是视频作者；点赞者可以给自己的视频点赞，本课允许这种行为。

打开 [like_entity.go](../internal/video/like_entity.go) 第 6 行的 VideoLike：

| 字段 / 数据库列 | 类型 | 来源与范围 | 设计理由 |
| --- | --- | --- | --- |
| UserID / user_id | int64 | Token 验证出的正整数用户编号 | 对应 users.id；不接受客户端随意指定 |
| VideoID / video_id | int64 | URL 的正整数视频编号 | 对应 videos.id；必须查到真实视频 |
| CreatedAt / created_at | time.Time | Gorm 插入时自动填写 | 保存这次点赞发生的时间 |

没有单独的 ID 字段，因为 (UserID, VideoID) 已能唯一识别一条关系。两列都标 primaryKey，组成一个**组合主键**；autoIncrement:false 表示编号来自已有用户和视频，不应自动生成新编号。

两个 ID 的 Go 零值都是 0，但业务中不代表合法用户或视频；认证和路径检查拒绝它。not null 只拒绝数据库 NULL，不等于“大于零”。

VideoID 另外标了 index，帮助按视频统计点赞。组合主键以 UserID 开头，因此额外的视频索引服务于另一种查询需求。

这里没有定义数据库外键，也没有级联删除。用户和视频是否存在，由本课事务内查询检查。将来支持删除用户或视频，需要再设计关联数据清理；不能以为组合主键会替我们处理它。

## 3. 数据库模型与响应结构体为什么分开

同文件 [like_entity.go](../internal/video/like_entity.go) 第 13 行的 LikeResponse 是响应结构，不建表：

| 响应字段 | 类型 | 含义 |
| --- | --- | --- |
| video_id | int64 | 本次操作的视频 |
| liked | bool | 当前调用者执行操作后的状态，true 已赞，false 未赞 |
| like_count | int64 | 当时该视频在关联表中的记录数量，可为 0 |

我们不在 videos 表加一个手动递增的点赞数。先直接 COUNT 关联记录，这样重复请求不需要另外维护计数同步。liked 也不存进关联表：有记录就是已赞，删除记录就是未赞。

## 4. 从结构体到建表，再到业务能力

打开 [repository.go](../internal/video/repository.go) 第 38 行 Migrate，第 39 行现在同时迁移 Video 和 VideoLike。启动程序原本就调用这个 Migrate，因此重启后才会在配置的数据库中创建 video_likes 表和索引。

本课代码构建顺序：

1. [like_entity.go](../internal/video/like_entity.go) 第 6、13 行：数据模型和响应。
2. [repository.go](../internal/video/repository.go) 第 38 行：让启动迁移创建关系表。
3. [like_repository.go](../internal/video/like_repository.go) 第 12 行：原子地完成检查、写入或删除、计数。
4. [like_service.go](../internal/video/like_service.go) 第 11 行：调用数据库操作，组装业务响应。
5. [like_handler.go](../internal/video/like_handler.go) 第 14、19、24 行：接收请求，处理身份和编号，把业务结果变成 HTTP。
6. [router.go](../internal/httpapi/router.go) 第 26、27 行：将两个入口挂到认证组。

新文件都在 internal/video 中，仍是同一个 video 包；like_ 前缀把这一条业务集中起来，避免继续拉长原来的发布、列表和详情文件。

## 5. 进入请求前，理解幂等与事务

**幂等的最小例子**：把灯“设为打开”，做两次还是开着；“切换灯的开关”，做两次会关回去。我们设计的是明确设置点赞状态，而不是每次请求都翻转状态。

PUT 指定“我要处于已点赞状态”；DELETE 指定“我要处于未点赞状态”。重复 PUT 不会额外增加记录，重复 DELETE 不会误删其他人的记录。别人同时点赞时总数仍可能变化，幂等不意味着每次返回的总数永远相同。

**事务的最小例子**：转账需要扣 A 的余额并增加 B 的余额。如果第二步失败，要撤销第一步。事务就是一组操作全部成功才提交，出错就回滚。

本课把存在性检查、关系写入/删除和计数放在同一事务。统计失败时写入也回滚，避免接口说失败但点赞已被本次操作写入。事务不是性能优化，也不保证不会遇到数据库锁错误。

## 6. Router 和中间件：先知道是谁在点赞

[router.go](../internal/httpapi/router.go) 第 26 行 PUT 对应 videos.Like，第 27 行 DELETE 对应 videos.Unlike，都在 protected 组。

请求先经过 [middleware/auth.go](../internal/httpapi/middleware/auth.go) 第 15 行 RequireUser：验证 Token，将用户 ID 放入本次 Gin 上下文，再执行 Handler。失败直接返回 401。

## 7. 完整看 Handler，再跟进被调用的函数

打开 [like_handler.go](../internal/video/like_handler.go)：

- 第 14 行 Like → 第 15 行 h.setLike(c, true)，进入第 24 行的公共处理函数。
- 第 19 行 Unlike → 第 20 行 h.setLike(c, false)，也进入第 24 行。区别仅是期望的最终状态。
- 第 25 行 CurrentUserID 读出已认证用户 ID；取不到则第 27 行返回 401，第 28 行结束。
- 第 30 行解析 URL 中的视频 ID；不合法则第 32 行返回 400，第 33 行结束。
- 第 35 行调用 h.service.SetLike。现在进入 Service，等待它返回 response、err。
- 返回后从第 36 行继续：用户已不存在用第 38 行返回 401；视频不存在用第 40 行返回 404；数据库故障用第 43 行返回 500；成功用第 45 行返回 200 JSON。

setLike 执行结束后，回到 Like 第 15 行或 Unlike 第 20 行之后，入口函数随之结束。HTTP 响应已经由 setLike 写出，入口不用再次 c.JSON。

## 8. Service 调用 Repository，Repository 内部还有事务回调

[like_service.go](../internal/video/like_service.go) 第 11 行 SetLike：

1. 第 12 行调用 s.repo.SetLike，等待返回 count 和 err。
2. 第 13–15 行：失败就将错误交回 Handler 第 35 行。
3. 第 16 行：成功组装 LikeResponse，交回 Handler 第 35 行，随后由 Handler 第 45 行返回 JSON。

继续进入 [like_repository.go](../internal/video/like_repository.go) 第 12 行：

- 第 15 行 Transaction 开始事务，并执行传进去的匿名函数。tx 是这次事务专用的数据库操作对象，因此里面统一使用 tx。
- 第 17 行查用户是否存在，结果放在 userCount；0 表示不存在，第 21 行返回业务错误。
- 第 24 行查视频，0 表示不存在，第 28 行返回视频不存在错误。
- liked=true：第 31 行创建关联对象，第 32–35 行插入。OnConflict 指定当 (user_id, video_id) 组合冲突时 DoNothing，因此重复点赞不新增、不报重复错误。其他数据库错误仍返回。
- liked=false：第 40 行按 user_id **和** video_id 删除，只取消当前用户对当前视频的点赞；没有匹配记录也算成功。
- 第 44 行统计该视频的全部点赞，结果写进 count。此处 return 是返回给 Transaction，不是直接返回给 Service。
- 回调返回 nil，Transaction 提交；返回错误则回滚。提交本身也可能失败，结果存入第 15 行的 err。
- 第 46 行才真正把 count 和 err 返回给 Service 第 12 行。

这里把两个存在性检查留在同一数据库事务里；Service 负责组装结果，Handler 负责解释 HTTP 状态。关系表的组合主键才是阻止重复记录的最终保障，不依赖“先查有没有点赞，再决定要不要插入”。

```text
PUT /videos/5/like + Token
→ RequireUser 验证身份
→ Handler.Like → setLike(true)
→ Service.SetLike
→ Repository.SetLike
  → Transaction：检查用户和视频 → 插入或忽略重复 → COUNT → 提交
← Repository 返回 count
← Service 返回 LikeResponse
→ Handler 第 45 行返回 200 JSON
```

取消的路径相同，只是 setLike(false) 会进入 DELETE 分支。

## 9. 手动验证：连续点赞两次，再连续取消两次

在服务终端 Ctrl+C 停止旧版本，运行脚本重新编译启动，也会创建新表：

```powershell
.\run.ps1
```

另开 PowerShell，先请求列表找到一条真实视频 ID。curl.exe 发送请求，-i 显示响应状态；默认 GET：

```powershell
curl.exe -i "http://127.0.0.1:18080/videos?limit=1"
```

登录命令把响应保存到 $login，再把其中的 access_token 保存到本窗口变量 $accessToken。使用你已注册的账号，示例账号不存在时换成你自己的：

```powershell
$login = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18080/users/login" -ContentType "application/json" -Body '{"username":"clean_user_01","password":"passw0rd!"}'
$accessToken = $login.access_token
$videoID = 5
```

把 $videoID 的 5 替换为列表中的真实 ID。-X PUT 指定点赞方法，-H 携带 Token；没有请求体，不需要 Content-Type：

```powershell
curl.exe -i -X PUT "http://127.0.0.1:18080/videos/$videoID/like" -H "Authorization: Bearer $accessToken"
```

原本无人点赞时，响应示例为 {"video_id":5,"liked":true,"like_count":1}。再执行同一命令，预期仍为 1（假设没有其他人同时操作）。

然后执行两次取消命令。-X DELETE 删除的是本人的点赞关系，不是视频：

```powershell
curl.exe -i -X DELETE "http://127.0.0.1:18080/videos/$videoID/like" -H "Authorization: Bearer $accessToken"
```

预期两次均 200、liked=false；原本只有自己的点赞时 like_count=0。若其他人也赞过，总数会保留他们的记录。

在 YAML 指定的 SQLite 数据库里检查（把 5 替换为实际 ID）：

```sql
SELECT user_id, video_id, created_at FROM video_likes WHERE video_id = 5;
SELECT COUNT(*) FROM video_likes WHERE video_id = 5;
PRAGMA table_info(video_likes);
PRAGMA index_list(video_likes);
```

两次点赞后只有自己的一条记录，取消后没有自己的记录。现有 GET 列表和详情本课没有增加 like_count；本课从点赞响应与 SQL 验证结果。

无 Token 返回 401；已登录时请求非法编号返回 400，合法但不存在的视频返回 404。总数是本次操作时的数据库结果，不是实时推送，其他用户稍后操作会改变它。

## 本课验证与练习

[video_like_test.go](../internal/httpapi/video_like_test.go) 第 18 行使用正式 Router 和独立 SQLite 数据库，检查重复请求、两用户相互隔离、无 Token、非法编号、不存在的视频、已不存在的用户，以及数据库直接拒绝重复组合。go test、go vet、构建均通过；用户实际操作与理解验收待完成。

预测题：用户 1 和用户 2 都点赞视频 5，用户 1 连续取消两次。最后 video_likes 剩哪一行？响应 liked 与 like_count 分别是什么？
