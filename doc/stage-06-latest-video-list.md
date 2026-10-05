# 第六课：游客分页浏览最新视频

新增公开接口：`GET /videos?limit=10&offset=0`。

它解决的业务问题是：短视频客户端打开首页时，任何人都应该能先看到最新视频；用户继续下滑时，客户端再请求下一页，而不是一次获取数据库的全部视频。

## 先理解分页，不看项目代码

假设数据库按从新到旧有 5 条视频：`E、D、C、B、A`。一次最多显示 2 条：

| 调用方请求 | 后端返回 items | 含义 |
| --- | --- | --- |
| `limit=2&offset=0` | `E、D` | 从第 0 条开始，取 2 条 |
| `limit=2&offset=2` | `C、B` | 跳过前 2 条，再取 2 条 |
| `limit=2&offset=4` | `A` | 跳过前 4 条，只剩 1 条 |

- `limit`：本次最多需要几条，默认 10，允许 1–50。
- `offset`：前面已经跳过几条，默认 0，不能为负数。
- `has_more`：这页后面是否还存在视频。
- `next_offset`：调用方下一次直接使用的 offset。

假设数据库中最新到最旧依次是：ID 5 的《海边日落》、ID 4 的《猫咪午睡》、ID 3 的《做饭记录》、ID 2 的《晨跑》。

调用方第一次请求：`GET /videos?limit=2&offset=0`。响应中的 `items` 就是**第一页真正查到的视频**：

```json
{
  "items": [
    {
      "id": 5,
      "author_id": 1,
      "title": "海边日落",
      "playback_url": "https://media.example.com/sunset.mp4",
      "created_at": "2026-09-29T15:00:00+08:00"
    },
    {
      "id": 4,
      "author_id": 2,
      "title": "猫咪午睡",
      "playback_url": "https://media.example.com/cat.mp4",
      "created_at": "2026-09-29T14:00:00+08:00"
    }
  ],
  "next_offset": 2,
  "has_more": true
}
```

客户端把 `items` 显示为首页的前两张视频卡片。由于 `has_more=true`，用户滑到底部时，客户端再请求 `GET /videos?limit=2&offset=2`：

```json
{
  "items": [
    {"id": 3, "author_id": 1, "title": "做饭记录", "playback_url": "https://media.example.com/cooking.mp4"},
    {"id": 2, "author_id": 3, "title": "晨跑", "playback_url": "https://media.example.com/running.mp4"}
  ],
  "next_offset": 4,
  "has_more": false
}
```

这时客户端不是替换第一页，而是追加：`[海边日落, 猫咪午睡] + [做饭记录, 晨跑]`。`has_more=false` 表示不再继续请求。

`next_offset=2` 不是视频 ID；它只表示“下一次已经跳过前 2 条”。

## 业务过程和当前代码放在一起看

### 1. Router 把视频列表定义为公开接口

业务上，游客也要能浏览首页，所以列表不能放进需要 Token 的 `protected` 路由组。

代码在 [internal/httpapi/router.go](../internal/httpapi/router.go) 第 14 行的 `NewRouter`：

- 第 19 行的 `router.GET("/videos", videos.ListNewest)` 是公开路由，没有 `RequireUser`。
- 第 24 行的 `protected.POST("/videos", videos.Publish)` 是发布接口，仍然需要登录。

同一个 URL `/videos` 可以有不同 HTTP 方法：`GET` 表示读取列表，`POST` 表示创建视频。Gin 根据“方法 + 路径”区分它们。

### 2. Handler 读取并检查分页参数

业务上，客户端只能请求合理大小的页面。若允许 `limit=1000000`，一次请求可能占用过多数据库和网络资源；若允许负数 offset，分页没有业务含义。

代码在 [internal/video/handler.go](../internal/video/handler.go) 第 24 行的 `ListNewest`：

请先只打开 [internal/video/handler.go](../internal/video/handler.go)，从第 24 行顺着读到第 36 行。**Service 调用和 200 响应都在同一个 `ListNewest` 函数里。** 执行顺序是：

1. 第 25 行：`limit, offset, err := listPage(c)`。先从 URL 读取分页参数。
2. 第 26–29 行：参数不合法时，立刻返回 `400 Bad Request`；函数在这里结束，后面的 Service 不会被调用。
3. 第 30 行：参数合法时，才执行 `response, err := h.service.ListNewest(...)`。这就是 Handler 调用 Service 的具体代码；`response` 用来接住 Service 返回的 `ListResponse`。
4. 第 31–35 行：数据库查询发生意外错误时，返回 500；函数也在这里结束。
5. 第 36 行：只有前面都成功，执行 `c.JSON(http.StatusOK, response)`。Gin 把 `response` 编码为 JSON，返回 `200 OK`。

`listPage` 本身在同文件第 65 行。它只负责把 URL 中的字符串转换成合法的 `limit`、`offset`；它不查询数据库，也不调用 Service。没传参数时给 `limit=10`、`offset=0`。

这里不读 Token，也不调用认证中间件，因为浏览列表不是受保护业务。

### 3. Service 多取一条，判断是否还有下一页

业务上，如果客户端拿到正好 10 条视频，单凭这 10 条无法判断数据库中是否还有第 11 条。后端需要一个可靠的 `has_more`。

代码在 [internal/video/service.go](../internal/video/service.go) 第 43 行的 `ListNewest`：

1. 它要求 Repository 实际读取 `limit + 1` 条。
2. 如果拿到的条数大于 `limit`，说明至少还有一条未返回：`has_more=true`。
3. 返回前裁掉额外那一条，只把最多 `limit` 条给调用方。
4. `next_offset = 当前 offset + 实际返回的 items 数量`。

例如 `limit=2&offset=0` 取到了 `E、D、C` 三条：Service 知道有更多数据，返回 `E、D`、`has_more=true`、`next_offset=2`。`C` 不会出现在本次响应，而会在下一页出现。

### 4. Repository 用 Gorm 排序、分页并读取 videos 表

业务上，“最新”必须有确定顺序。先按 `created_at DESC`，若两条视频创建时间恰好相同，再按 `id DESC`。第二个排序条件防止同一时间的视频顺序随机变化。

代码在 [internal/video/repository.go](../internal/video/repository.go) 第 25 行的 `ListNewest`：

- `Order("created_at DESC")`：较新的创建时间排前面。
- `Order("id DESC")`：相同时间时，ID 较大的排前面。
- `Limit(limit + 1)`：多读一条让 Service 计算 `has_more`。
- `Offset(offset)`：跳过之前页面已经浏览过的数量。
- `Find(&videos)`：让 Gorm 执行 SELECT，并填充 `[]Video`。

### 5. 响应结构告诉调用方怎样请求下一页

业务上，调用方需要视频数组，也需要知道是否继续加载和下一次该传什么参数。

代码在 [internal/video/entity.go](../internal/video/entity.go) 第 21 行的 `ListResponse`：

```text
items        本页实际显示的视频
next_offset  下一页请求应使用的 offset
has_more     true 时客户端可以继续请求；false 时停止加载
```

完整调用图：

```text
GET /videos?limit=2&offset=0
  → httpapi/router.go：公开 GET /videos
  → video/handler.go：ListNewest + listPage
  → video/service.go：多取一条，计算 has_more / next_offset
  → video/repository.go：ORDER BY、LIMIT、OFFSET、SELECT
  → 200 JSON ListResponse
```

## 手动验收

启动服务：

```powershell
.\run.ps1
```

这个接口不需要登录，也不需要 Token。另开一个 PowerShell：

```powershell
curl.exe -i "http://127.0.0.1:18080/videos?limit=1&offset=0"
```

预期 `200 OK`，响应有 `items`、`next_offset`、`has_more`。如果 `has_more` 为 `true`，把响应的 `next_offset` 填进下一次请求：

```powershell
curl.exe -i "http://127.0.0.1:18080/videos?limit=1&offset=1"
```

再验证参数保护：

```powershell
curl.exe -i "http://127.0.0.1:18080/videos?limit=0"
```

预期 `400 Bad Request`。

## 本阶段练习

先预测：如果数据库有 3 条视频，调用 `GET /videos?limit=2&offset=0`，你认为 `items` 有几条、`has_more` 是什么、`next_offset` 是多少？再根据 Service 的“多取一条”逻辑解释原因。
