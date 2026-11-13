// 全局事件总线
var _seenEvents = (function () {
    try {
        return (
            JSON.parse(sessionStorage.getItem("seshat_seen_events") || "{}") ||
            {}
        );
    } catch (_) {
        return {};
    }
})();
function _rememberEvent(key) {
    _seenEvents[key] = 1;
    var keys = Object.keys(_seenEvents);
    for (var i = 0; i < keys.length - 400; i++) delete _seenEvents[keys[i]];
    try {
        sessionStorage.setItem(
            "seshat_seen_events",
            JSON.stringify(_seenEvents)
        );
    } catch (_) {}
}

// 订阅一路事件，来源写进去重键
function subscribeEvents(url, source) {
    var es = new EventSource(url);
    es.onmessage = function (e) {
        var d;
        try {
            d = JSON.parse(e.data);
        } catch (_) {
            return;
        }
        // 已处理过的事件（含历史回放）不再弹出；跨翻页/刷新保留
        if (d.id) {
            var key = source + ":" + d.id;
            if (_seenEvents[key]) return;
            _rememberEvent(key);
        }
        var msg = d.message;
        if (source === "remote") msg = "远端：" + msg;
        if (d.type === "error") showError(msg);
        if (d.type === "warn") showWarn(msg);
    };
    es.onerror = function () {
        es.close();
        setTimeout(function () {
            subscribeEvents(url, source);
        }, 5000);
    };
}

function initEventBus() {
    subscribeEvents("/api/v0/events", "local");
    if (!window.BACKEND_URL) return;
    capabilitiesReady().then(function () {
        if (resolve("events") === "remote") {
            subscribeEvents(API + "/v0/events", "remote");
        }
    });
}
