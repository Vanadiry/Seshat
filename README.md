# Seshat

离线优先的 Bangumi 数据管理与浏览工具。  
使用 Go 后端和 Tailwind CSS 构建前端。所有数据存放在本地。

特别感谢 [番组计划](https://bgm.tv/) 的数据源。

<img src="/docs/image/readme-02.png" width="600"/>
<div style="display:flex">
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

下载对应系统的二进制文件后运行即可，macOS/Linux 请先赋予可执行权限。  
运行时，请保持终端窗口打开。

<details><summary>macOS</summary>

1. 打开终端，输入 `chmod +x`，然后将二进制文件拖入终端，点击回车。
2. 再将二进制文件拖入终端，点击回车。（若提示“移动到废纸篓”，继续第三步）
3. 在终端输入 `xattr -cr`，然后将二进制文件拖入终端，点击回车。
4. 再执行一遍第二步，程序应当启动。

</details>

<details><summary>Linux</summary>

1. 打开终端，输入 `chmod +x`，然后将二进制文件拖入终端，点击回车。
2. 再将二进制文件拖入终端，点击回车。

</details>

<details><summary>Windows</summary>

直接运行 exe 即可。如果被 Defender 拦截，需要手动放行。

</details>
<br />

程序会在本地启动 Go 后端，并自动拉起前端。  
如果 WebUI 没有自动打开，请访问 `http://127.0.0.1:12500`。

详细使用方法，请阅读 [docs/guide](/docs/guide.md).

## 从源码构建

```bash
python build.py
```
