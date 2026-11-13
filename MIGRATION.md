# 迁移说明

## v2

适用于从 v1.x 迁移至 v2.x。  
v2.0.0 会进行较大重构，尽管 Seshat 也能够重建数据，但仍然建议你阅读迁移说明。

### 升级前

1. 备份 `SESHAT_HOME`，或至少备份你的 Tracker 和 ELO 历史数据
2. 关闭正在运行的所有 Seshat 实例
3. 更新至 2.0.0 或更高版本

### Bangumi 用户数据目录

用户资料、头像、收藏从 `user/info/` 移到 `data/user/`。
收藏数据结构也进行了修改，v2 无法识别旧的文件结构。

建议你手动删除 `SESHAT_HOME/data/user` 目录，重新拉取用户数据即可。

### API

v2 对 API 做了一定修改，请阅读 [Seshat API 设计说明](docs/api/design.md)和[OpenAPI 文档](docs/api/openapi.yaml)详细了解。  
如果你是普通用户，没有将 Seshat 作为服务端集成到你的其他应用，则不需要在意此段。  
