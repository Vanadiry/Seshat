package config

// DefaultConfigTOML is written to ~/.vSoft/Seshat/config.toml on first run.
const DefaultConfigTOML = `# Seshat 配置文件
bind_addr = "127.0.0.1"
port = 4000
data_home = ""
username = ""
sync_enabled = false
concurrency = 64
base_url = "https://api.bgm.tv"
user_agent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
`

// TrackerTemplate is the content for a new tracker .toml file.
const TrackerTemplate = `# Tracker: %s
name = "%s"
ids = []
`
