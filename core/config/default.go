package config

import (
	"fmt"
	"strings"
)

var DefaultConfigTOML = fmt.Sprintf(`# Seshat 配置
# 配置说明见文档

[server]
bind_addr = "127.0.0.1"     # 监听地址，设为 0.0.0.0 时对局域网开放
port = 12500                # 开放端口
api_token = ""              # 本地 API 令牌，首次创建时自动生成；留空则不校验 POST
concurrency_info = 4        # 信息拉取并发
concurrency_image = 16      # 图片下载并发
data_home = ""              # 数据目录，留空则使用 SESHAT_HOME/data
log_level = "warn"          # 日志级别："debug" / "info" / "warn" / "error"

[upstream]
# 上游地址，控制后端拉取数据时的请求位置。留空则无法拉取数据。
base_url = "https://api.bgm.tv"
user_agent = "Vanadiry/Seshat/v%s (https://github.com/Vanadiry/Seshat)"

[proxy]
# 上游拉取使用的 HTTP 代理。留空则不启用。
host = ""
port = 0

[frontend]
# 后端地址。留空则请求本地 /api/v0。设为 https://api.bgm.tv 则前端直连番组计划。
backend_url = ""
# 回退地址。本地请求 404 时回退到此地址，留空则不启用回退。
fallback_url = ""

[access]
# 个人访问令牌，用于拉取受限条目。生成地址 https://next.bgm.tv/demo/access-token
bangumi_access_token = ""
`, Version)

// TrackerFileContent 生成空 Tracker 的 TOML 内容
func TrackerFileContent(id, name string) string {
	return fmt.Sprintf("# Tracker: %s\nname = %s\nids = []\n", id, quoteTOMLString(name))
}

// quoteTOMLString 转义 TOML 基本字符串
func quoteTOMLString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
