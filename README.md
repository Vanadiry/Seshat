# Seshat

离线优先的 Bangumi 数据管理与浏览工具。  
Go 后端 + Tauri 桌面壳 + Tailwind CSS 前端。所有数据存放在本地。

特别感谢 [番组计划](https://bgm.tv/) 的数据源。

<img src="/docs/image/readme-02.png" width="600"/>
<div style="display:flex">
<img src="/docs/image/readme-01.png" width="300"/><img src="/docs/image/readme-03.png" width="300"/>
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

### Seshat Desktop

这是推荐使用的版本，是单独的桌面应用。  
只需要下载安装包，并按各个平台的安装方式，安装即可使用。

<details><summary>对于 macOS</summary>

打开程序时，可能会提示已损坏/无法打开/无法验证等，这是因为我没钱买一年 99 美元的苹果证书。

1. 双击 `dmg` 镜像，将 Seshat 软件拖入“应用程序”文件夹。
2. 打开终端，输入 `xattr -cr /Applications/Seshat`
3. 在终端输入 `xattr -cr /Applications/Seshat\ Desktop.app`，回车即可。

若依然无法启动，先双击一下 Seshat 软件，然后打开“设置→隐私与安全性”，滑动到底部，点击仍要运行即可。

</details>

### Seshat Server

Server 版本和 Desktop 版本行为一模一样，只是没有内置 Tauri 壳，可以作为后端使用。

下载对应系统的二进制文件后运行，macOS/Linux 请先赋予可执行权限。  
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

需要 `go`、`rust` 环境，以及 `pnpm` 包管理器。

```bash
pnpm install
python3 scripts/build.py server    # Seshat Server
python3 scripts/build.py desktop   # Seshat Desktop
```
