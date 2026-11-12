# Seshat API 设计说明

本文记录 OpenAPI 之外的行为约定，供对接 Seshat 的开发者参考。
本文由 AI 撰写。

## 图片

拉取时只下载 `large`、`grid`、`small`，跳过 `medium` 和 `common`。

请求尺寸缺失时按顺序回退：

| 请求尺寸 | 回退顺序 |
| --- | --- |
| `large` | `large`、`small`、`grid` |
| `small` | `small`、`large`、`grid` |
| `grid` | `grid`、`small`、`large` |
| `medium`、`common` | `large`、`small`、`grid` |

响应不说明实际尺寸。没有本地文件时，Subject 注入 `/assets/no-image.png`，Character 和 Person 注入空字符串。

`images` 不写入数据文件，读取详情和关联列表时按当前请求的协议和主机临时注入，`X-Forwarded-Proto`、`X-Forwarded-Host` 优先。  
剧集、`characters/{id}/subjects.json` 和 `persons/{id}/subjects.json` 的 schema 没有 `images`，不注入。

本地图片接口返回 200 和文件本身，官方返回 302，本地带有 `Cache-Control: no-store`。

用户头像只保存一张 large 文件，`large`、`medium`、`small` 三个地址都指向它。

## 错误响应

```json
{
  "title": "Not Found",
  "description": "数据不存在",
  "details": { "path": "/api/v0/subjects/999", "method": "GET" },
  "request_id": "3f9c1a7b2d4e5f60"
}
```

`title` 由状态码推导，英文。`description` 为中文，直接显示给用户。`details` 可选。`request_id` 与响应头 `X-Request-ID` 一致。

客户端传入合法的 `X-Request-ID` 时复用，否则生成 16 位十六进制。合法长度为 1 到 64，只允许字母、数字、`-`、`_`、`.`。

404 的 `details` 带 `path` 和 `method`。请求体超限返回 413，`details.max_bytes` 带上限。其他 JSON 解析失败返回 400，ELO 数据不足返回 409。

`settings` 的批量结果 `{success, error}` 是部分成功响应，不使用上面的结构。

## 用户数据

用户资料、头像、收藏和摘要存放在 `<data_home>/user`，`settings` 与 `elo` 仍在主目录的 `user/settings`、`user/elo`。

`users/{username}`、`avatar`、`collections` 校验路径 username，与配置用户不一致或未配置时返回 404。本地为单用户实例。

旧路径 `user/info` 不自动迁移，迁移步骤见 [MIGRATION.md](/MIGRATION.md)。

## 剧集

`/api/v0/episodes` 返回 `Paged_Episode`。

- `type` 可选，按章节类型过滤
- `limit` 范围 1 到 200
- `offset` 大于等于 0
- 不传 `limit` 返回全部，响应里的 `limit` 写实际条数

拉取时每页 100 条，任意一页失败时不覆盖已有的 `episodes.json`。

## 收藏

完整收藏保存在 `data/user/collections.json`，保存上游条目并递归剥离 `images`，返回时注入 `subject.images`。

`data/user/collections_summary.json` 保存 `subject_id`、`subject_type`、`type`、`rate`，缺失时从完整文件派生。

- 默认返回 `Paged_UserCollection`
- 支持 `subject_type`、`type`、`limit`、`offset`
- `limit` 范围 1 到 100，不传返回全部
- `?view=summary` 返回 `{total, data}`

Tracker 同步和统计页只处理 `subject_type=2` 的动画收藏。旧版 `{subjects: {id: type}}` 需要重新执行“刷新用户数据”。

## 私有列表

Seshat 自有列表统一返回 `{total, limit, offset, data}`，不分页时 `limit` 和 `offset` 为 0。

覆盖 `/v0/{subjects,characters,persons}/list`、`/v0/tags`、`/v0/tracker`、`/v0/tasks` 和 `/v0/elo/history`。`elo/ranking` 使用 `data`，另有 `no_rating`、`orphans`、`total_compares`。

`/v0/tracker` 的每项包含 `id`、`name`、`type`、`count`。同名 TOML 和 JSON 会合并，`type` 为 `auto` 或 `manual`。

`subjects/name`、`characters/name`、`persons/name` 为对象映射，不套列表结构。

## 数据与拉取

```text
data/api/{domain}/{id%10}/{id}/{file}
data/images/{kind}_{size}/{id%10}/{id}.jpg
data/index/*.json
```

`GET /api/v0/` 按路径映射到 `data/api` 下的数据文件。

`_seshat.json` 由统一拉取的 `source=ids` 成功后写入，只记录成功 ID，整批拉取结束后写入。前端不提供删除入口。

`info.json`、`characters.json`、`persons.json`、`episodes.json`、`subjects.json` 任一失败或写入失败，整个条目失败，不进入 `_seshat.json`。`fetch/gap` 检查核心文件，缺失时重拉。

图片只补缺失的尺寸。“重建全部”删除 `data/api`、`data/images`、`data/index`，保留 `data/user`。

## 本地搜索

`POST /v0/search/{subjects,characters,persons,tags}` 扫描本地 `info.json`，最多返回 500 条，`total` 为命中总数。

只匹配人类可读字段：

- `name`
- `name_cn`
- `infobox` 的值
- `summary`

标签按名称匹配。JSON 键名、图片地址和链接值不参与匹配。

结果可选带 `match`：

- `field` 为 `name`、`name_cn`、`infobox` 或 `summary`
- `infobox` 带 `key`
- `infobox` 和 `summary` 带 `text`

`text` 为第一个命中位置前后各 30 字，截断时补省略号。

## 上游搜索

`POST /v0/upstream/search` 使用 `[upstream] base_url`、UA、token 和代理请求官方搜索接口。  
它只返回搜索结果，不写入本地数据。`upstream` 能力缺失时，前端隐藏入口。

## 数据包

`POST /v0/bundle/export` 把一组本地条目的一跳数据导出为 tar。  
manifest 只记录包内的实体和图片，不记录来源；缺失数据在导出响应的 `missing` 中返回。

`POST /v0/bundle/import` 接收 raw tar，上限 1 GiB。导入期间注册 task，其他任务返回 409。  
只允许 `manifest.json`、`api/` 和 `images/` 下的白名单路径，拒绝链接、路径穿越、重复条目和超限文件。

## 跨源

CORS 按监听地址决定，不回显固定自身地址。

- 回环监听只放行回环 Origin，包括 `localhost`、`127.0.0.0/8` 和 `::1`
- 非回环监听放行任意合法的 `http` 或 `https` Origin，公网域名和公网 IP 都可用
- 空值表示监听所有地址，按非回环处理
- `null` 和非法 Origin 始终拒绝
- 拒绝的 Origin 发普通请求返回 403，预检返回 204 但不带允许头

允许时回显请求 Origin，并返回 `Vary: Origin`。允许的请求头为 `Content-Type`、`Authorization` 和 `X-Request-ID`，响应头暴露 `X-Request-ID`。

绑定非回环地址后，任意网页都能调用 API。当前没有鉴权，拉取、Tracker 和 ELO 都会改数据。  
公网暴露需要自行承担风险，CORS 不负责权限控制。

## 能力

`/v0/info` 的 `features` 是描述性的能力清单，前端据此决定功能入口，不用于权限。

- 核心接口始终提供，前端不判断，包括详情、图片、剧集、用户、收藏和搜索
- feature 为 Seshat 扩展：`elo`、`events`、`fetch`、`lists`、`names`、`stats`、`tags`、`task`、`tracker`、`upstream`、`bundle`
- `upstream` 仅在配置 `base_url` 后提供，对应 `POST /v0/upstream/search`
- 缺失 feature 时隐藏入口，`lists` 和 `names` 退回本机
- 旧版 Seshat 没有 `features`，按空能力处理，核心接口仍可用

## 兼容性

镜像端点尽量对齐官方 v0，主要差异如下：

- 本地图片返回 200，官方返回 302
- 官方收藏默认 30 条，最大 100
- 官方没有 `/v0/tags`，非 Seshat 后端需要隐藏标签入口
- `features` 由 `/v0/info` 返回，前端据此决定功能可用性

破坏性变更与迁移步骤见 [MIGRATION.md](/MIGRATION.md)。
