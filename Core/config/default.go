package config

// DefaultConfigTOML is written to ~/.vSoft/Seshat/config.toml on first run.
const DefaultConfigTOML = `# Seshat 配置文件
bind_addr = "127.0.0.1"
port = 4000
data_home = ""
username = ""
sync_enabled = false
concurrency = 32
base_url = "https://api.bgm.tv"
`

// TrackerTemplate is the content for a new tracker .toml file.
const TrackerTemplate = `# Tracker: %s
name = "%s"
ids = []
`
