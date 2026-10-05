# 阶段 0，第 2 小步：从 YAML 读取服务地址

目标：健康检查接口仍是 `GET /healthz`，但端口不再写死在 Go 代码里。

## 脱离项目的最小例子

YAML 是一种配置文本：

```yaml
server:
  addr: "127.0.0.1:18080"
```

Go 不会自动读取它。程序需要读取文件内容，再用 YAML 库把文本转成 Go 结构体，
最后从结构体取出 `Server.Addr`。

## 本次改动

| 文件 | 作用 |
| --- | --- |
| [configs/local.yaml](../configs/local.yaml) 第 2 行 | 保存本机服务地址和未来数据库文件路径 |
| [internal/config/config.go](../internal/config/config.go) 第 25 行 | 读取并检查 YAML 配置 |
| [cmd/api/main.go](../cmd/api/main.go) 第 23 行 | 调用 Load，并将配置地址交给 Gin |
| [internal/config/config_test.go](../internal/config/config_test.go) 第 10 行 | 验证 YAML 文本能正确变成 Config |

启动调用图：

```text
main → config.Load("configs/local.yaml") → Config.Server.Addr → router.Run(addr)
```

`database.path` 已经保存在 YAML 中，但当前没有使用它，因为本阶段尚未引入 Gorm 或数据库。

## 你现在观察什么

1. 点击 [configs/local.yaml](../configs/local.yaml)，看第 2 行的 `server.addr`。
2. 点击 [internal/config/config.go](../internal/config/config.go)，看第 25 行：它如何读取文件和检查 addr。
3. 点击 [cmd/api/main.go](../cmd/api/main.go)，看第 23 行：`cfg.Server.Addr` 如何替代原先的硬编码端口。

## 验收

启动：

```powershell
.\run.ps1
```

请求：

```powershell
curl.exe -i http://127.0.0.1:18080/healthz
```

预期仍是 `200 OK` 和 `{"status":"ok"}`。然后只修改 YAML 中的 `18080` 为 `18082`，
停止并重启服务，再将 curl 请求端口改为 18082。预期新端口成功，旧端口无法连接。
