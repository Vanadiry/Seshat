package config

const DefaultConfigTOML = `# Seshat 配置文件

[server]
bind_addr = "127.0.0.1"
port = 4000
concurrency = 32
# data_home = "" — 留空则使用 SESHAT_HOME/data
data_home = ""

# base_url = "" 表示不请求上游，仅浏览本地缓存
[upstream]
base_url = "https://api.bgm.tv"
user_agent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

# backend_url 置空则前端请求本地 /api/v0；设为 https://api.bgm.tv 则前端直连官方
# fallback_url: 本地 404 时回退请求此地址，置空则不启用回退
# prefer_lang: "original" 原文优先, "chinese" 中文优先
[frontend]
backend_url = ""
fallback_url = ""
prefer_lang = "original"

[user]
username = ""
fetch_collections = false
`

const TrackerTemplate = `# Tracker: %s
name = "%s"
ids = []
`
