// 配置提示按浏览器会话只显示一次，会话 Cookie 随浏览器关闭失效
var CONFIG_WARN_KEY = "seshat_config_warn";
var _configWarnToast = null;

function configWarnMode() {
    if (window.BACKEND_URL && window.FALLBACK_URL) return "both";
    if (window.BACKEND_URL) return "backend";
    if (window.FALLBACK_URL) return "fallback";
    return "";
}

function _readCookie(name) {
    var raw = document.cookie ? document.cookie.split("; ") : [];
    for (var i = 0; i < raw.length; i++) {
        var pos = raw[i].indexOf("=");
        if (pos < 0) continue;
        if (raw[i].slice(0, pos) === name) {
            return decodeURIComponent(raw[i].slice(pos + 1));
        }
    }
    return "";
}

function _writeCookie(name, value) {
    try {
        document.cookie =
            name + "=" + encodeURIComponent(value) + "; path=/; SameSite=Lax";
    } catch (_) {}
}

function _configWarnTokens() {
    var raw = _readCookie(CONFIG_WARN_KEY);
    if (!raw) {
        try {
            raw = sessionStorage.getItem(CONFIG_WARN_KEY) || "";
        } catch (_) {
            raw = "";
        }
    }
    if (!raw) return [];
    try {
        var list = JSON.parse(raw);
        return Array.isArray(list) ? list : [];
    } catch (_) {
        return [];
    }
}

function _configWarnSeen(token) {
    return _configWarnTokens().indexOf(token) !== -1;
}

function _markConfigWarn(token) {
    var list = _configWarnTokens();
    if (list.indexOf(token) === -1) list.push(token);
    var raw = JSON.stringify(list);
    _writeCookie(CONFIG_WARN_KEY, raw);
    try {
        sessionStorage.setItem(CONFIG_WARN_KEY, raw);
    } catch (_) {}
}

function showConfigWarn() {
    var mode = configWarnMode();
    if (!mode) return;
    var token = location.host + "|" + mode;
    if (_configWarnSeen(token)) return;
    _markConfigWarn(token);
    var title = MSG.configWarnBackendTitle;
    var body = MSG.configWarnBackendBody;
    if (mode === "fallback") {
        title = MSG.configWarnFallbackTitle;
        body = MSG.configWarnFallbackBody;
    } else if (mode === "both") {
        title = MSG.configWarnBothTitle;
        body = MSG.configWarnBothBody;
    }
    _configWarnToast = _makeToast(
        title,
        body,
        "bg-toast-warning",
        "bg-toast-warning-bg",
        0,
        false
    );
    // 常驻提示不参与后续 toast 的替换
    _currentToast = null;
}

// 连接失败时更新常驻提示，没有提示时退回普通警告
function showConfigFailure() {
    if (_configWarnToast && !_configWarnToast.closed()) {
        _configWarnToast.update(
            MSG.configWarnFailedTitle,
            MSG.configWarnFailedBody
        );
        return true;
    }
    showWarn(MSG.configWarnFailedTitle + "，" + MSG.configWarnFailedBody);
    return false;
}
