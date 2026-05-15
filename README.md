# Seshat

离线优先的 Bangumi 数据管理与浏览工具。  
使用 Go 后端和 Tailwind CSS 构建前端。所有数据存放在本地。

特别感谢 [番组计划](https://bgm.tv/) 的数据源。

<img src="/docs/image/readme-02.png" width="608"/>
<div style="display:flex; gap:8px;">
<img src="/docs/image/readme-01.png" width="300"/><img src="/docs/image/readme-03.png" width="
300"/>
</div>

## 主要功能

- 双链和多维度检索。
- 离线优先，数据保存在本地，仅拉取时联网。
- ELO 评分排名。
- 从番组计划导入收藏。

前后端分离，Seshat 获取数据的 API 设计，与“番组计划”API 完全一致。
因此还可以直接使用 Seshat 前端，访问“番组计划”的内容。  
并支持在本地数据不足时，自动回退以从远端拉取数据。

## 使用

下载二进制文件，然后运行即可。程序会在本地开启 WebUI 前端和 Go 后端。  
访问 `http://127.0.0.1:12500` 即可打开 Seshat WebUI。

详细使用方法，请阅读 [docs/guide](/docs/guide.md).

## 从源码构建

```bash
go build
```
