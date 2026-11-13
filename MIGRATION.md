# 迁移说明

## v2

适用于从 v1.x 迁移至 v2.x。  
v2.0.0 进行了较大重构，Seshat 虽然可以重建数据，仍建议按本文操作。

### 升级前

1. 备份 `SESHAT_HOME`，至少备份 `tracker` 和 `user/elo`
2. 关闭所有正在运行的 Seshat 实例
3. 更新至 2.0.0 或更高版本

### 用户数据

用户资料、头像和收藏从 `user/info` 移到 `data/user`，数据结构也有变化，v2 不读取旧文件。

升级后：

1. 打开“管理数据”页面的“用户数据”区域
2. 执行“刷新用户数据”
3. 确认头像、收藏和“我的评分”正常
4. 手动删除 `SESHAT_HOME/user/info`

### 配置

v2 新增 `api_token`，只在首次生成配置时自动写入。  
从 v1 升级的配置没有该字段，程序不会自动补充。

```toml
[server]
api_token = "随机字符串"
```

### 自定义后端

使用 `backend_url` 或 `fallback_url` 连接其他 Seshat 实例时，建议两端都升级到 v2。  
v2 前端连接 v1 后端可能出现功能缺失或数据解析失败。

### API

v2 对 API 做了修改，详细说明见 [Seshat API 设计说明](docs/api/design.md) 和 [OpenAPI 文档](docs/api/openapi.yaml)。

普通用户如果不把 Seshat 作为服务端集成到其他应用，可以跳过本节。
