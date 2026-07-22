package config

const DefaultConfigTOML = `# Seshat 配置
# 要修改此处内容，建议先阅读文档

[server]
# bind_addr = "127.0.0.1"     # 监听地址，设为0.0.0.0将对局域网开放
# port = 12500                # 开放端口
# concurrency_info = 4        # 信息拉取并发
# concurrency_image = 16      # 图片下载并发
# data_home = ""              # 数据目录，留空则使用SESHAT_HOME/data
# log_level = "warn"          # 日志级别: "debug" / "info" / "warn" / "error"

[upstream]
# 上游地址，控制后端拉取数据时的请求位置，建议不动。留空将不能拉取数据。
# base_url = "https://api.bgm.tv"
# user_agent = "Vanadiry/Seshat/v1.3.1 (https://github.com/Vanadiry/Seshat)"

[frontend]
# 后端地址。留空则请求本地/api/v0。设为https://api.bgm.tv则前端直连番组计划。
# backend_url = ""
# 回退地址。当本地请求404时回退到此地址，留空则不启用回退。
# fallback_url = ""

[access]
# 个人访问令牌，用于拉取受限条目。在 https://next.bgm.tv/demo/access-token 生成。
# bangumi_access_token = ""
`

const TrackerTemplate = `# Tracker: %s
name = "%s"
ids = []
`
