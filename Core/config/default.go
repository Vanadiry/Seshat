package config

const DefaultConfigTOML = `# Seshat 配置
# 要修改此处内容，建议先阅读文档

[server]
bind_addr = "127.0.0.1"     # 监听地址，设为0.0.0.0将对局域网开放
port = 12500                # 开放端口
concurrency = 32            # 并发数，建议32即可
data_home = ""              # 数据目录，留空则使用SESHAT_HOME/data

[upstream]
# 上游地址，控制后端拉取数据时的请求位置，建议不动。留空将不能拉取数据。
base_url = "https://api.bgm.tv"
user_agent = "Vanadiry/Seshat/v0.2.4 (https://github.com/Vanadiry/Seshat)"

[frontend]
# 后端地址。留空则请求本地/api/v0。设为https://api.bgm.tv则前端直连番组计划。
backend_url = ""
# 回退地址。当本地请求404时回退到此地址，留空则不启用回退。
fallback_url = ""
`

const DefaultPreferencesJSON = `{
  "_comment": "修改后刷新页面即可生效，无需重启后端",
  "prefer_lang": {
    "_choice": [
      {"original": "原文优先"},
      {"chinese": "中文优先"}
    ],
    "_comment": "标题展示方式",
    "value": "original"
  },
  "username": {
    "_comment": "在此处填写你的 Bangumi番组计划 ID，将能够拉取收藏和头像等信息",
    "value": ""
  }
}
`

const TrackerTemplate = `# Tracker: %s
name = "%s"
ids = []
`
