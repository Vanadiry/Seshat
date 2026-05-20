# Changelog

## [v0.2.6] - 2026-05-20

### Breaking Changes

- Name index 格式从 `{name: id}` 改为 `{id: [names]}`，需要重建索引。

### Added

- Name index 同时收录中文名，并完善前端以支持中文匹配高亮

### Fixed

- 前端修复 linkifyByMap 和 linkifyURL 在连续调用时破坏已生成 a 标签的问题
- 前端高亮 Infobox 添加黑名单，用于排除日期等错误高亮
- 修正前端 Infobox 和简介的高亮：
  优先匹配 URL，避免链接截断
  角色和人物名字合并为单次匹配，解决跨 map 短词抢先截断长词的问题
  Infobox 内大于 2 字的角色和人物优先匹配，余下的按预设分隔符分段匹配，避免错误拆字
  Infobox 中人物名字后面跟着的“集数”，在匹配阶段被保护，防止错误高亮集数数字

## [v0.2.5] - 2026-05-16

### Fixed

- 优化 OpenAPI 文档
- 压缩前端数据

## [v0.2.4] - 2026-05-15

### Fixed

- 让toml支持多行数组
- 更新默认UA以符合番组计划规范
- 修正其他Tracker在执行更新时会写入_seshat.json的问题

## [v0.2.3] - 2026-05-15

### Fixed

- 修正获取用户信息时会错误写入Tracker的问题
- 修正测试中设置返回值不符合预期导致的问题

## [v0.2.2] - 2026-05-15

### Added

- 后端启动时自动打开浏览器
- 优化前端标题展示

### Changed

- 添加 Tailwind 本地编译
- 使用 `-ldflags="-s -w"` 精简二进制

## [v0.2.1] - 2026-05-15

### Fixed

- 修正统计页面卡片布局细节

## [v0.2.0] - 2026-05-14

### Added

- 统计页面：条目/图片/ELO/Tracker 数据仪表盘
- ELO 排名百分位展示
- 设置页面与设置读写 API
- 设置页面
- 前后端配置分离
- 用户收藏 API
- Tracker 导入收藏功能
- ELO 评分页面（PK 对比、历史记录、排名筛选）
- ELO 评分重建 API

### Changed

- ELO 排名返回三向分组（有评级 / 无评级 / 仅评级）
- HTML 后缀 URL 统一 301 重定向为清洁 URL
- 搜索改为 POST 并拆分为独立分类端点
- 进度条重构为右下角浮动通知横幅
- 用户数据路径移至 `HOME/user/`
- ELO 数据路径移至 `HOME/user/elo/`
- 日志文件路径从 `data/logs/` 移至 `HOME/logs/`
- 图像缺失返回 302 重定向到占位图
- 页面路由简化，移除逐条 .html 路由

### Fixed

- embedFS 为 nil 时前端路由 panic
- 评分历史及剧集方格布局问题
- 搜索页、页面跳转样式细节
- 用户数据存储路径不一致

## [v0.1.0] - 2026-05-13

### Added

- Go 后端：配置管理、日志系统、embed 静态文件服务、CORS 中间件
- 从 Bangumi API 拉取并缓存动画 / 角色 / 人物数据
- 并发拉取与下载（可配置并发数）
- 任务锁防止并行执行任务
- SSE 实时进度推送
- 活跃任务查询 API
- ELO 评分系统（随机配对、提交比较、排名计算）
- 图片下载与缓存服务
- RESTful 缓存读取 API（`/api/v0/{domain}/{id}/{file}`）
- 图片服务 API（`/api/v0/{domain}/{id}/image?type=`）
- 搜索、统计、ELO、Tracker、用户、设置 API
- OpenAPI 文档
- 前端：首页动画列表、动画 / 角色 / 人物详情页
- 剧集列表、关联条目、角色及制作人员展示
- 标签浏览与搜索
- Infobox 嵌套解析、BBCode 链接、URL 自动识别
- Tracker 管理（创建 / 查看 / 拉取）
- Tracker 名称校验
- 语言偏好设置
- 优先显示语言配置
- 姓名索引及中文名提取
- 响应式布局、暗色主题
- 47 个测试用例（config、cache、server API）
