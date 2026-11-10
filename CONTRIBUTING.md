# 贡献

## 功能开发

目前不接受大功能的 PR，有需求请先提交 Issue。

## 对接 Seshat

[docs/api/openapi.yaml](/docs/api/openapi.yaml) 与 [docs/api/design.md](/docs/api/design.md) 面向对接 Seshat 的开发者。

## 构建

GitHub Release 的软件包，由 GitHub Actions 构建。  
本地构建，需要 `go`、`rust`、`pnpm`、`uv`。

```bash
pnpm install
uv run scripts/build.py server/desktop/sdev/dev
```
