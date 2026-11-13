const API = window.BACKEND_URL || "/api";

// 能力表，顺序即设置页的展示顺序
var CAPS = [
    { key: "fetch", label: "数据拉取" },
    { key: "tracker", label: "Tracker 管理" },
    { key: "lists", label: "列表索引" },
    { key: "names", label: "名称索引" },
    { key: "elo", label: "ELO 评分" },
    { key: "tags", label: "标签" },
    { key: "stats", label: "统计" },
    { key: "task", label: "任务" },
    { key: "events", label: "事件通知" },
    { key: "bundle", label: "数据包" }
];
var CAPS_ALL = CAPS.map(function (c) {
    return c.key;
});
var CAPS_LABELS = {};
CAPS.forEach(function (c) {
    CAPS_LABELS[c.key] = c.label;
});
CAPS_LABELS.upstream = "上游搜索";

// 按能力表顺序输出中文名，未知能力排在后面
function capLabels(features) {
    var on = {};
    for (var i = 0; i < features.length; i++) on[features[i]] = true;
    var out = [];
    for (var j = 0; j < CAPS.length; j++) {
        if (on[CAPS[j].key]) out.push(CAPS[j].label);
    }
    for (var k = 0; k < features.length; k++) {
        if (!CAPS_LABELS[features[k]]) out.push(features[k]);
    }
    return out;
}

var CAPS_TTL = 10 * 60 * 1000;
var CAPS_FAIL_TTL = 30 * 1000;
var CAPS_KEY = "seshat_caps:" + API;
var _caps = null;
var _capsPromise = null;
var _capsSettled = false;
var _probeOverlay = null;
var _probeOverlayAt = 0;
var _probeShowTimer = null;

// 读取会话缓存，过期返回 null
function _readCaps() {
    try {
        var raw = sessionStorage.getItem(CAPS_KEY);
        if (!raw) return null;
        var c = JSON.parse(raw);
        if (!c || typeof c.at !== "number" || typeof c.ttl !== "number") {
            return null;
        }
        if (Date.now() - c.at > c.ttl) return null;
        return c;
    } catch (_) {
        return null;
    }
}

// 探测提示，只在缓存未命中且需要探测时出现
function _showProbeOverlay() {
    _probeShowTimer = setTimeout(function () {
        if (_capsSettled || _probeOverlay) return;
        var el = document.createElement("div");
        el.id = "probe-overlay";
        el.setAttribute("role", "status");
        el.setAttribute("aria-live", "polite");
        el.className =
            "fixed inset-0 z-[200] bg-bg text-text flex items-center justify-center";
        el.innerHTML =
            '<div class="text-sm text-sub"><span class="spin"></span>正在连接后端</div>';
        document.body.appendChild(el);
        _probeOverlay = el;
        _probeOverlayAt = Date.now();
    }, 200);
}

function _hideProbeOverlay() {
    clearTimeout(_probeShowTimer);
    if (!_probeOverlay) return;
    var wait = Math.max(0, 200 - (Date.now() - _probeOverlayAt));
    setTimeout(function () {
        if (!_probeOverlay) return;
        _probeOverlay.remove();
        _probeOverlay = null;
    }, wait);
}

// 写入会话缓存，失败结果到期后允许重试
function _saveCaps(caps, ttl) {
    caps.at = Date.now();
    caps.ttl = ttl;
    _caps = caps;
    _capsSettled = true;
    try {
        sessionStorage.setItem(CAPS_KEY, JSON.stringify(caps));
    } catch (_) {}
    if (ttl === CAPS_FAIL_TTL) {
        setTimeout(function () {
            if (_caps === caps) {
                _caps = null;
                _capsPromise = null;
                _capsSettled = false;
            }
        }, ttl);
    }
    return caps;
}

// 探测失败时提示一次，并写入短缓存
function _failCaps() {
    showConfigFailure();
    return _saveCaps({ seshat: false, features: [] }, CAPS_FAIL_TTL);
}

// 请求 /v0/info，5 秒超时
async function _fetchCaps() {
    var ctl = new AbortController();
    var timer = setTimeout(function () {
        ctl.abort();
    }, 5000);
    try {
        var res = await fetch(API + "/v0/info", { signal: ctl.signal });
        if (res.status === 404) {
            return _saveCaps({ seshat: false, features: [] }, CAPS_TTL);
        }
        if (!res.ok) {
            return _failCaps();
        }
        var body = await res.json();
        if (!body || typeof body !== "object" || Array.isArray(body)) {
            return _saveCaps({ seshat: false, features: [] }, CAPS_TTL);
        }
        var features = [];
        if (Array.isArray(body.features)) {
            features = body.features.filter(function (f) {
                return typeof f === "string";
            });
        }
        return _saveCaps({ seshat: true, features: features }, CAPS_TTL);
    } catch (_) {
        return _failCaps();
    } finally {
        clearTimeout(timer);
    }
}

// 探测远端能力，本地直接视为全部可用
function probeCapabilities() {
    if (_capsPromise) return _capsPromise;
    if (!window.BACKEND_URL) {
        _caps = { seshat: true, local: true, features: CAPS_ALL.slice() };
        _capsSettled = true;
        _capsPromise = Promise.resolve(_caps);
        return _capsPromise;
    }
    var cached = _readCaps();
    if (cached) {
        _caps = cached;
        _capsSettled = true;
        _capsPromise = Promise.resolve(cached);
        return _capsPromise;
    }
    _showProbeOverlay();
    _capsPromise = _fetchCaps().then(function (caps) {
        _hideProbeOverlay();
        return caps;
    });
    return _capsPromise;
}

// 等待探测结束
function capabilitiesReady() {
    return probeCapabilities();
}

// 判断能力是否可用，探测完成前返回 false
function hasFeature(name) {
    if (!window.BACKEND_URL) return true;
    return !!(_caps && _caps.seshat && _caps.features.indexOf(name) !== -1);
}

// 页面加载后异步探测，不阻塞渲染
probeCapabilities();

// 路由分类，feature 缺失时 lists 和 names 退回本机，其余隐藏
var ROUTE = {
    settings: "local",
    "open-url": "local",
    core: "remote",
    lists: "fallback-local",
    names: "fallback-local",
    elo: "feature",
    events: "feature",
    fetch: "feature",
    stats: "feature",
    tags: "feature",
    task: "feature",
    tracker: "feature",
    upstream: "feature",
    bundle: "feature"
};

// 解析请求归属，返回 remote、local 或 hidden
function resolve(key) {
    var mode = ROUTE[key];
    if (!mode) return "hidden";
    if (!window.BACKEND_URL) {
        if (key === "upstream" && window.UPSTREAM_ENABLED === false) {
            return "hidden";
        }
        return "local";
    }
    if (mode === "local") return "local";
    if (mode === "remote") return "remote";
    if (mode === "fallback-local") {
        return hasFeature(key) ? "remote" : "local";
    }
    return hasFeature(key) ? "remote" : "hidden";
}

// 按归属请求，隐藏时返回 null
async function apiFor(key, url) {
    var mode = ROUTE[key];
    if (!mode) return null;
    if (
        window.BACKEND_URL &&
        (mode === "feature" || mode === "fallback-local")
    ) {
        await capabilitiesReady();
    }
    var target = resolve(key);
    if (target === "hidden") return null;
    return target === "local" ? apiLocal(url) : api(url);
}

// 隐藏缺能力的入口，支持同时要求多个能力
function featureHidden(node) {
    var all = node.getAttribute("data-feature-all");
    if (all) {
        var keys = all.split(",");
        for (var i = 0; i < keys.length; i++) {
            if (resolve(keys[i].trim()) === "hidden") return true;
        }
        return false;
    }
    var one = node.getAttribute("data-feature");
    return !!one && resolve(one) === "hidden";
}

function applyFeatureVisibility(root) {
    var nodes = (root || document).querySelectorAll(
        "[data-feature], [data-feature-all]"
    );
    for (var i = 0; i < nodes.length; i++) {
        nodes[i].style.display = featureHidden(nodes[i]) ? "none" : "";
    }
}

// 页面能力检查，缺能力时显示提示并返回 false
async function requireFeature(key) {
    if (window.BACKEND_URL) await capabilitiesReady();
    if (resolve(key) !== "hidden") return true;
    var main = document.querySelector("main");
    if (main) {
        main.innerHTML =
            '<div class="text-center py-24 text-sub">当前后端不支持此功能</div>';
    }
    return false;
}
