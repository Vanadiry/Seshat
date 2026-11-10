// Seshat 共享脚本，包含顶栏、拉取弹窗、SSE 进度和 API 辅助函数

// HTML 转义，避免 innerHTML 注入
function escapeHtml(str) {
    if (!str) return "";
    return String(str)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;");
}

function openUrl(url) {
    fetch("/api/v0/open-url", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url: url })
    });
}

// 主题跟随系统，值为 auto 时生效
var _themeMedia = window.matchMedia("(prefers-color-scheme: light)");
_themeMedia.addEventListener("change", function (e) {
    if ((localStorage.getItem("theme") || "auto") !== "auto") return;
    document.documentElement.classList.toggle("light", e.matches);
});

document.addEventListener("dragstart", function (e) {
    if (e.target.tagName === "IMG") e.preventDefault();
});

// 在 .thin-scroll 容器上拖拽滚动
(function () {
    var el,
        startX,
        scrollLeft,
        down = false,
        dragged = false;
    document.addEventListener("mousedown", function (e) {
        el = e.target.closest(".thin-scroll");
        if (!el) return;
        down = true;
        dragged = false;
        startX = e.pageX - el.getBoundingClientRect().left;
        scrollLeft = el.scrollLeft;
    });
    document.addEventListener("mousemove", function (e) {
        if (!down || !el) return;
        var x = e.pageX - el.getBoundingClientRect().left;
        if (!dragged && Math.abs(x - startX) < 5) return;
        dragged = true;
        e.preventDefault();
        el.style.cursor = "grabbing";
        el.style.userSelect = "none";
        el.scrollLeft = scrollLeft - (x - startX);
    });
    document.addEventListener("mouseup", function () {
        if (!down) return;
        down = false;
        if (el) {
            el.style.cursor = "";
            el.style.userSelect = "";
        }
        el = null;
    });
    // 拖拽结束后阻止误触点击
    document.addEventListener(
        "click",
        function (e) {
            if (dragged) {
                e.stopPropagation();
                e.preventDefault();
                dragged = false;
            }
        },
        true
    );
})();

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
    { key: "events", label: "事件通知" }
];
var CAPS_ALL = CAPS.map(function (c) {
    return c.key;
});
var CAPS_LABELS = {};
CAPS.forEach(function (c) {
    CAPS_LABELS[c.key] = c.label;
});

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
    tracker: "feature"
};

// 解析请求归属，返回 remote、local 或 hidden
function resolve(key) {
    var mode = ROUTE[key];
    if (!mode) return "hidden";
    if (!window.BACKEND_URL) return "local";
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

// 界面文案，弹窗提示在这里修改
var MSG = {
    // 配置提示
    configWarnBackendTitle: "自定义后端模式",
    configWarnBackendBody: "数据从自定义后端获取，部分功能按后端能力显示",
    configWarnFallbackTitle: "回退模式",
    configWarnFallbackBody: "本地缺失的数据将从回退端点获取",
    configWarnBothTitle: "自定义后端和回退模式",
    configWarnBothBody: "数据从自定义后端获取，本地缺失时从回退端点获取",
    configWarnFailedTitle: "自定义后端连接失败",
    configWarnFailedBody: "能力相关功能已禁用",
    // 拉取弹窗
    dialogTitle: "管理数据",
    idLabel: "拉取动画",
    idPlaceholder: "动画 ID，如 51 或 51, 288",
    trackerLabel: "拉取或创建 Tracker",
    trackerPlaceholder: "Tracker 名称",
    btnUserFetch: "刷新用户数据",
    btnUpdate: "增量更新",
    btnGap: "补充数据",
    btnMeta: "刷新元数据",
    btnRefreshAll: "刷新全部",
    confirmGap:
        "将遍历所有已缓存条目，检查并拉取有更新的人物/角色/剧集等数据。",
    confirmMeta: "将重新拉取所有条目的基本信息（不包含图片）。",
    btnDangerZone: "危险区",
    btnRebuildIndex: "重建索引",
    btnRebuildAll: "重建全部",
    btnExecute: "执行",
    btnCancel: "取消",
    btnClose: "关闭",
    btnImportColl: "导入收藏",
    confirmImportColl:
        "将从用户收藏列表中读取数据，并存入 Tracker。\n之后拉取数据时将能够拉取你的收藏。",
    importCollDone: function (n) {
        return "已将 " + n + " 个条目导入至 tracker/user.json";
    },

    // 确认提示
    confirmFetchId: function (ids) {
        return (
            "将从拉取动画 " +
            ids
                .split(",")
                .map(function (s) {
                    return "#" + s.trim();
                })
                .join(", ") +
            " 的完整数据"
        );
    },
    confirmTrackerFetch: function (name) {
        return "将拉取 Tracker [" + name + "] 中记录的全部动画数据？";
    },
    confirmTrackerCreate: function (name) {
        return "未找到 Tracker [" + name + "]。\n是否创建？";
    },
    trackerCreated: function (name) {
        var d = window.SESHAT_HOME
            ? window.SESHAT_HOME + "/tracker"
            : "~/.vSoft/Seshat/tracker";
        return (
            "Tracker [" +
            name +
            "] 已创建。请在 " +
            d +
            "/" +
            name +
            ".toml 中填写动画 ID，然后返回此处重新拉取。"
        );
    },
    confirmUserFetch: function () {
        return "将拉取 " + window.USERNAME + " 的用户数据与收藏列表。";
    },
    confirmUpdate: "将对比本地与上游数据，并添加缺失的数据。",
    confirmRefreshAll:
        "将从上游拉取全部 Tracker 数据，覆盖已有内容。本地多余数据不会删除。",
    confirmRebuildIndex: "将从本地 JSON 中重建所有索引。",
    confirmRebuildAll: function () {
        var d = window.SESHAT_HOME || "~/.vSoft/Seshat";
        return "将删除 " + d + "/data 的全部数据，并完整重建。";
    },

    // 校验
    errInvalidTrackerName:
        "Tracker 名称仅允许大小写字母、数字、短横线和下划线。",
    errUserFetchFailed: "刷新用户数据失败: ",
    errNoUsername:
        "未在配置中设置用户名（[user] username），无法刷新用户数据。",

    // 进度
    progressConnecting: "连接中...",
    progressDone: "完成",

    // 信息栏
    infoboxGender: "性别",
    infoboxBloodType: "血型",
    infoboxBirthday: "生日",
    bloodTypeMap: { 1: "A", 2: "B", 3: "AB", 4: "O" },

    // 章节类型
    epTypes: {
        0: "本篇",
        1: "SP",
        2: "OP",
        3: "ED",
        4: "预告",
        5: "MAD",
        6: "其他"
    },
    // 条目类型，用于角色出演
    subjectTypes: { 1: "书籍", 2: "动画", 3: "音乐", 4: "游戏", 6: "三次元" },
    genderMap: { male: "男", female: "女" },
    careerMap: {
        actor: "演员",
        artist: "艺术家",
        illustrator: "插画家",
        producer: "制作人",
        seiyu: "声优",
        writer: "作家"
    }
};

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

// 收藏类型与徽标
var COLL_TYPES = {
    1: "想看",
    2: "看过",
    3: "在看",
    4: "搁置",
    5: "抛弃"
};
var COLL_CLS = {
    1: "bg-pink-bg text-tier-gold",
    2: "bg-pink-bg text-tier-green",
    3: "bg-tier-blue-bg text-tier-blue",
    4: "bg-surface-raised text-sub",
    5: "bg-warn-bg text-warn"
};
function collBadge(type) {
    if (!COLL_TYPES[type]) return "";
    return (
        '<span class="' +
        (COLL_CLS[type] || "") +
        ' px-1.5 py-px rounded text-[10px] font-semibold">' +
        COLL_TYPES[type] +
        "</span>"
    );
}

var _remoteURLs = {};
function isRemote(url) {
    return !!_remoteURLs[url];
}
function markRemote(url) {
    _remoteURLs[url] = true;
}

function authOpts() {
    var tok = window.ACCESS_TOKEN;
    if (!tok) return {};
    return { headers: { Authorization: "Bearer " + tok } };
}

// 加载条
var _ldCount = 0,
    _ldTimer = null;
function _ldShow() {
    clearTimeout(_ldTimer);
    _ldTimer = setTimeout(function () {
        var bar = document.getElementById("load-bar");
        if (bar) bar.classList.add("active");
    }, 200);
}
function _ldHide() {
    _ldCount--;
    if (_ldCount <= 0) {
        _ldCount = 0;
        clearTimeout(_ldTimer);
        var bar = document.getElementById("load-bar");
        if (bar) bar.classList.remove("active");
    }
}

async function api(url) {
    _ldCount++;
    _ldShow();
    try {
        var r = await fetch(API + url, API !== "/api" ? authOpts() : {});
        if (!r.ok && window.FALLBACK_URL) {
            var fr = await fetch(window.FALLBACK_URL + url, authOpts());
            if (fr.ok) {
                markRemote(url);
                return fr.json();
            }
        }
        if (!r.ok) return null;
        return r.json();
    } finally {
        _ldHide();
    }
}
// apiLocal 始终请求本机后端，用于列表和索引数据
function apiLocal(url) {
    return fetch("/api" + url).then(function (r) {
        return r.ok ? r.json() : null;
    });
}
// errText 从错误响应里取可读文案
function errText(d, status) {
    if (d && d.description) return d.description;
    if (d && d.title) return d.title;
    if (d && d.error) return d.error;
    return "请求失败 (" + status + ")";
}
// readJSON 解析响应体，失败时返回 null
function readJSON(res) {
    return res.json().catch(function () {
        return null;
    });
}
// getInstanceInfo 复用一次 /v0/info 请求
var _infoPromise = null;
function getInstanceInfo() {
    if (!_infoPromise) _infoPromise = apiLocal("/v0/info");
    return _infoPromise;
}
function img(kind, id, size) {
    return API + "/v0/" + kind + "s/" + id + "/image?type=" + (size || "grid");
}
// imgFallbackSrc 返回图片的远端回退地址，未配置时为空
function imgFallbackSrc(kind, id, size) {
    if (!window.FALLBACK_URL) return "";
    return (
        window.FALLBACK_URL +
        "/v0/" +
        kind +
        "s/" +
        id +
        "/image?type=" +
        (size || "grid")
    );
}
// imgOnError 返回 onerror 属性，配置回退时第一次错误走
// FALLBACK_URL，第二次错误显示本地占位图。未配置回退时直接显示占位图
function imgOnError(kind, id, size) {
    var fb = imgFallbackSrc(kind, id, size);
    if (!fb) return "onerror=\"this.src='/assets/no-image.png'\"";
    return (
        "onerror=\"if(!this._r){this._r=1;this.src='" +
        fb +
        "';addRemoteGlobe(this);}else{this.src='/assets/no-image.png';}\""
    );
}
// attachImgFallback 给已有 img 应用同一套回退链
function attachImgFallback(el, kind, id, size) {
    removeRemoteGlobe(el);
    var fb = imgFallbackSrc(kind, id, size);
    var retried = false;
    el.onerror = function () {
        if (!fb || retried) {
            this.src = "/assets/no-image.png";
            return;
        }
        retried = true;
        this.src = fb;
        addRemoteGlobe(this);
    };
}

// 远端标记辅助函数
// globeIcon 返回地球标记的 img 标签，用于标题和分区标题
function globeIcon(cls) {
    return (
        '<img src="/assets/global.svg" class="' +
        (cls || "w-5 h-5") +
        ' shrink-0" title="此数据来自远程">'
    );
}
// addRemoteGlobe 包装远端回退图片，在左上角加地球标记
function addRemoteGlobe(el) {
    if (el._g) return;
    el._g = 1;
    var wrap = document.createElement("span");
    var s =
        "position:relative;display:inline-block;line-height:0;vertical-align:top;";
    if (el.classList.contains("w-full")) s += "width:100%;";
    if (el.style.width) s += "width:" + el.style.width + ";";
    if (el.style.height) s += "height:" + el.style.height + ";";
    wrap.style.cssText = s;
    el.parentNode.insertBefore(wrap, el);
    wrap.appendChild(el);
    var globe = document.createElement("img");
    globe.src = "/assets/global.svg";
    globe.style.cssText =
        "position:absolute;top:4px;left:4px;z-index:10;width:20px;height:20px;pointer-events:none;";
    globe.title = "图片来自远程";
    wrap.appendChild(globe);
}

// removeRemoteGlobe 移除 addRemoteGlobe 加的标记，便于复用元素
function removeRemoteGlobe(el) {
    if (!el._g) return;
    var wrap = el.parentNode;
    if (wrap && wrap.parentNode) {
        wrap.parentNode.insertBefore(el, wrap);
        wrap.parentNode.removeChild(wrap);
    }
    el._g = 0;
}

// Infobox key blacklist：这些字段只做 URL/BBCode，不做名字匹配
var INFOBOX_NO_MATCH_KEYS = [
    "放送开始",
    "播放开始",
    "放送结束",
    "播放结束",
    "放送星期",
    "播放星期",
    "话数"
];

// 名称到 ID 的映射，包含条目、角色、人物
var _subjectMap = null,
    _charMap = null,
    _personMap = null;
function loadPersonMap() {
    if (window.AUTO_LINK_NAMES === "false") return Promise.resolve();
    if (_personMap && Object.keys(_personMap).length) return Promise.resolve();
    return Promise.all([
        apiFor("names", "/v0/subjects/name"),
        apiFor("names", "/v0/characters/name"),
        apiFor("names", "/v0/persons/name")
    ]).then(function (results) {
        _subjectMap = invertNameMap(results[0]);
        _charMap = invertNameMap(results[1]);
        _personMap = invertNameMap(results[2]);
        _personRegex = null;
        _charPersonMerged = null;
    });
}

// 反转 name map：{id: [names]} → {name: id}，后者覆盖
function invertNameMap(raw) {
    var m = {};
    if (!raw) return m;
    for (var id in raw) {
        if (!raw.hasOwnProperty(id)) continue;
        var names = raw[id];
        for (var i = 0; i < names.length; i++) {
            if (names[i]) m[names[i]] = parseInt(id);
        }
    }
    return m;
}

// Merged char + person map（person 优先）
var _charPersonMerged = null;
function getCharPersonMergedMap() {
    if (_charPersonMerged) return _charPersonMerged;
    _charPersonMerged = {};
    if (_charMap) {
        for (var k in _charMap) {
            if (_charMap.hasOwnProperty(k))
                _charPersonMerged[k] = "/character?id=" + _charMap[k];
        }
    }
    if (_personMap) {
        for (var k in _personMap) {
            if (_personMap.hasOwnProperty(k))
                _charPersonMerged[k] = "/person?id=" + _personMap[k];
        }
    }
    return _charPersonMerged;
}

// Subject 自由匹配（最长优先，不限制边界）
function linkifySubjectFree(text) {
    return linkifyByMap(text, _subjectMap, "/subject?id=");
}

// 分段匹配：直接查合并 map，集数已在顶层保护
function matchSegmentCore(seg, merged) {
    seg = seg.trim();
    if (!seg) return seg;
    var id = merged[seg];
    if (!id) return seg;
    return (
        '<a href="' + id + '" class="text-pink hover:underline">' + seg + "</a>"
    );
}

// Char+Person 自由匹配（仅 ≥3 字，最长优先）
function linkifyCharPersonFree3Plus(text) {
    if (!text) return text;
    var merged = getCharPersonMergedMap();
    if (!merged || !Object.keys(merged).length) return text;
    var subMap = {};
    for (var k in merged) {
        if (merged.hasOwnProperty(k) && k.length >= 3) subMap[k] = merged[k];
    }
    return linkifyByMap(text, subMap, "");
}

// 分隔符集合
var _SEPARATOR_RE = /([、,，\/\(\)（）\[\]])/;

// Char+Person 分段匹配：按分隔符拆段，每段整体匹配
function linkifyCharPersonSegments(text) {
    if (!text) return text;
    var merged = getCharPersonMergedMap();
    if (!merged || !Object.keys(merged).length) return text;
    var parts = text.split(/(<a\b[^>]*>.*?<\/a>)/);
    for (var i = 0; i < parts.length; i++) {
        if (parts[i].indexOf("<a") === 0) continue;
        var t = parts[i];
        if (!t) continue;
        var segs = t.split(_SEPARATOR_RE);
        for (var j = 0; j < segs.length; j++) {
            if (/^[、,，\/\(\)（）\[\]]$/.test(segs[j])) continue;
            if (!segs[j].trim()) continue;
            segs[j] = matchSegmentCore(segs[j], merged);
        }
        parts[i] = segs.join("");
    }
    return parts.join("");
}

// Infobox value 处理：BBCode → URL → 黑名单判定 → 保护集数 → 1a subject 自由匹配 → 1b ≥3 字自由匹配 → 2 分段匹配 → 还原集数
function processInfoboxValue(v, key) {
    var t = linkifyBBCode(v);
    t = linkifyURL(t);
    if (INFOBOX_NO_MATCH_KEYS.indexOf(key) >= 0) return t;
    // 保护集数括号（纯数字+逗号+连字符），避免内部数字被匹配
    var epsBlocks = [];
    t = t.replace(/\([\d\-,]+\)/g, function (m) {
        epsBlocks.push(m);
        return "\x00EPS" + (epsBlocks.length - 1) + "\x00";
    });
    t = linkifySubjectFree(t);
    t = linkifyCharPersonFree3Plus(t);
    t = linkifyCharPersonSegments(t);
    t = t.replace(/\x00EPS(\d+)\x00/g, function (_, i) {
        return epsBlocks[parseInt(i)];
    });
    return t;
}
function lookupLocalName(name) {
    if (_subjectMap && _subjectMap[name])
        return "/subject?id=" + _subjectMap[name];
    if (_charMap && _charMap[name]) return "/character?id=" + _charMap[name];
    if (_personMap && _personMap[name]) return "/person?id=" + _personMap[name];
    return null;
}

// 名称匹配用的正则构建
var _personRegex = null;
function buildPersonRegex() {
    if (_personRegex) return _personRegex;
    var names = Object.keys(_personMap);
    if (!names.length) return null;
    names.sort(function (a, b) {
        return b.length - a.length;
    });
    var escaped = names.map(function (n) {
        return n.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    });
    _personRegex = new RegExp(escaped.join("|"), "g");
    return _personRegex;
}

function buildRegexForMap(map) {
    if (!map) return null;
    var names = Object.keys(map);
    if (!names.length) return null;
    names.sort(function (a, b) {
        return b.length - a.length;
    });
    var escaped = names.map(function (n) {
        return n.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    });
    return new RegExp(escaped.join("|"), "g");
}

function linkifyByMap(text, map, hrefPrefix) {
    if (!text || !map || !Object.keys(map).length) return text;
    var re = buildRegexForMap(map);
    if (!re) return text;
    var parts = text.split(/(<a\b[^>]*>.*?<\/a>)/);
    for (var i = 0; i < parts.length; i++) {
        if (parts[i].indexOf("<a") === 0) continue;
        parts[i] = parts[i].replace(re, function (match) {
            return (
                '<a href="' +
                hrefPrefix +
                map[match] +
                '" class="text-pink hover:underline">' +
                match +
                "</a>"
            );
        });
    }
    return parts.join("");
}

// 保留 linkifyPerson 兼容旧代码，信息栏仍在用
function linkifyPerson(text) {
    return linkifyByMap(text, _personMap, "/person?id=");
}

// 应用三种名称匹配，跳过已在 a 标签里的文本
function linkifyAllNames(text) {
    if (!text) return text;
    var parts = text.split(/(<a\b[^>]*>.*?<\/a>)/);
    for (var i = 0; i < parts.length; i++) {
        if (parts[i].indexOf("<a") === 0) continue;
        var t = parts[i];
        t = linkifyByMap(t, _subjectMap, "/subject?id=");
        t = linkifyByMap(t, _charMap, "/character?id=");
        t = linkifyPerson(t);
        parts[i] = t;
    }
    return parts.join("");
}

// linkifyURL：检测 URL 并转为可点击链接
function linkifyURL(text) {
    if (!text) return text;
    var parts = text.split(/(<a\b[^>]*>.*?<\/a>)/);
    for (var i = 0; i < parts.length; i++) {
        if (parts[i].indexOf("<a") === 0) continue;
        parts[i] = parts[i].replace(
            /(https?:\/\/[^\s<]+)/g,
            '<a href="$1" target="_blank" rel="noopener" class="text-pink hover:underline break-all"><img src="/assets/link.svg" class="w-3.5 h-3.5 inline-block mr-0.5 align-text-bottom">$1</a>'
        );
    }
    return parts.join("");
}

// linkifyBBCode：转换 [url=...]...[/url]，优先匹配本地 name
function linkifyBBCode(text) {
    if (!text) return text;
    return text.replace(
        /\[url=(https?:\/\/[^\]]+)\](.+?)\[\/url\]/g,
        function (match, url, inner) {
            var local = lookupLocalName(inner);
            if (local)
                return (
                    '<a href="' +
                    local +
                    '" class="text-pink hover:underline">' +
                    inner +
                    "</a>"
                );
            return (
                '<a href="' +
                url +
                '" target="_blank" rel="noopener" class="text-pink hover:underline"><img src="/assets/link.svg" class="w-3.5 h-3.5 inline-block mr-0.5 align-text-bottom">' +
                inner +
                "</a>"
            );
        }
    );
}

// formatSummary：BBCode → name 匹配 → 换行
function formatSummary(text) {
    if (!text) return text;
    return linkifyAllNames(linkifyBBCode(text)).replace(
        /\r\n|\n/g,
        '<br><span style="display:block;height:0.5em"></span>'
    );
}

// infoboxData：提取侧栏信息（infobox + 顶层字段）
function infoboxData(d) {
    var items = [];
    // 顶层字段
    if (d.gender)
        items.push([MSG.infoboxGender, MSG.genderMap[d.gender] || d.gender]);
    if (d.blood_type) {
        var bt = MSG.bloodTypeMap;
        items.push([MSG.infoboxBloodType, bt[d.blood_type] || d.blood_type]);
    }
    if (d.birth_mon || d.birth_day) {
        var bd = [];
        if (d.birth_year) bd.push(d.birth_year + "年");
        if (d.birth_mon) bd.push(d.birth_mon + "月");
        if (d.birth_day) bd.push(d.birth_day + "日");
        items.push([MSG.infoboxBirthday, bd.join("")]);
    }
    // 信息栏数组
    var ib = d.infobox;
    if (typeof ib === "string") {
        try {
            ib = JSON.parse(ib);
        } catch (e) {
            ib = [];
        }
    }
    if (Array.isArray(ib)) {
        for (var i = 0; i < ib.length; i++) {
            if (!ib[i].key) continue;
            var v = ib[i].value;
            if (typeof v === "string") {
                items.push([ib[i].key, processInfoboxValue(v, ib[i].key), 0]);
            } else if (Array.isArray(v)) {
                if (v.length > 0 && typeof v[0] === "object") {
                    if (v[0].k) {
                        // Key-value pairs: [{k: "纯假名", v: "..."}]
                        items.push([ib[i].key, "", 0]);
                        for (var j = 0; j < v.length; j++) {
                            if (v[j].k)
                                items.push([
                                    v[j].k,
                                    processInfoboxValue(
                                        v[j].v || "",
                                        ib[i].key
                                    ),
                                    1
                                ]);
                        }
                    } else if (v[0].v) {
                        // 只有值的对象，例如 [{v: "クラナド"}]
                        items.push([ib[i].key, "", 0]);
                        for (var j = 0; j < v.length; j++) {
                            if (v[j].v)
                                items.push([
                                    "",
                                    processInfoboxValue(v[j].v, ib[i].key),
                                    1
                                ]);
                        }
                    } else {
                        // 兜底显示数组里的第一个字符串
                        for (var j = 0; j < v.length; j++) {
                            if (typeof v[j] === "string") {
                                items.push([
                                    ib[i].key,
                                    processInfoboxValue(v[j], ib[i].key),
                                    0
                                ]);
                                break;
                            }
                        }
                    }
                } else {
                    // 纯字符串数组
                    for (var j = 0; j < v.length; j++) {
                        if (typeof v[j] === "string") {
                            items.push([
                                ib[i].key,
                                processInfoboxValue(v[j], ib[i].key),
                                0
                            ]);
                            break;
                        }
                    }
                }
            }
        }
    }
    return items;
}

// primaryName / subName：根据 display_lang 配置决定主副标题顺序
function primaryName(name, nameCN) {
    if (window.PREFER_LANG === "chinese") return nameCN || name;
    return name || nameCN;
}
function subName(name, nameCN) {
    if (window.PREFER_LANG === "chinese") {
        return nameCN && name && name !== nameCN ? name : "";
    }
    return nameCN && name !== nameCN ? nameCN : "";
}

// extractCN：从 infobox 提取简体中文名
function extractCN(d) {
    var ib = d.infobox;
    if (typeof ib === "string") {
        try {
            ib = JSON.parse(ib);
        } catch (e) {
            return "";
        }
    }
    if (!Array.isArray(ib)) return "";
    for (var i = 0; i < ib.length; i++) {
        if (ib[i].key === "简体中文名" && typeof ib[i].value === "string")
            return ib[i].value;
    }
    return "";
}

// 加载条和顶栏
(function () {
    var bar = document.createElement("div");
    bar.id = "load-bar";
    document.body.appendChild(bar);
})();

document.addEventListener("DOMContentLoaded", () => {
    const tb = document.getElementById("topbar");
    if (!tb) return;

    const path = location.pathname;
    function navCls(href) {
        var p = path;
        if (p === "/subject") p = "/";
        else if (p === "/character") p = "/characters";
        else if (p === "/person") p = "/persons";
        else if (p === "/tag") p = "/tags";
        if (href === "/" && p === "/") return "bg-active text-text";
        if (href !== "/" && p.startsWith(href)) return "bg-active text-text";
        return "hover:bg-active hover:text-text";
    }

    tb.style.cssText = "position:sticky;top:0;z-index:50";
    var navStyle = navCls;
    tb.innerHTML = `
    <div class="bg-surface-raised border-b border-bord">
      <div class="max-w-[1200px] mx-auto px-5 flex items-center gap-3 h-12 relative">
        <h1 class="text-lg font-bold cursor-pointer shrink-0" onclick="location.href='/'">Seshat</h1>
        <nav class="flex gap-1 ml-2 items-center">
          <a href="/" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/")}">动画</a>
          <a href="/characters" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/characters")}">角色</a>
          <a href="/persons" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/persons")}">人物</a>
          <a href="/tags" data-feature="tags" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/tags")}">标签</a>
          <a href="/stats" data-feature="stats" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/stats")}">统计</a>
        </nav>
        <div class="flex-1"></div>
        <nav class="flex gap-1 items-center">
          <a href="/rating" data-feature="elo" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/rating")}">评分</a>
          <a href="/search" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/search")}">搜索</a>
        </nav>
        <button id="btn-fetch" data-feature-all="fetch,task" class="px-4 py-1.5 rounded-lg bg-pink text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90">管理数据</button>
        <div class="relative shrink-0">
          <div class="relative group w-8 h-8 shrink-0 cursor-pointer rounded-lg focus-visible:outline focus-visible:outline-2 focus-visible:outline-pink" role="button" tabindex="0" aria-label="用户菜单" onclick="toggleUserMenu(event)" onkeydown="if(event.key==='Enter'||event.key===' '){event.preventDefault();toggleUserMenu(event)}">
            <img id="user-avatar" src="" class="w-8 h-8 rounded-lg object-cover bg-surface shrink-0" style="display:none">
            <div id="user-placeholder" class="w-8 h-8 rounded-lg shrink-0" style="background:linear-gradient(135deg,#ff6b6b,#feca57,#48dbfb,#ff9ff3,#54a0ff);background-size:300% 300%;animation:rainbow 3s ease infinite"></div>
            <div class="pointer-events-none absolute inset-0 rounded-lg bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity duration-150 flex items-center justify-center">
              <svg viewBox="0 0 24 24" class="w-4 h-4 text-white" fill="currentColor" aria-hidden="true">
                <circle cx="5" cy="12" r="1.7"></circle>
                <circle cx="12" cy="12" r="1.7"></circle>
                <circle cx="19" cy="12" r="1.7"></circle>
              </svg>
            </div>
          </div>
          <div id="user-dropdown" class="absolute right-0 top-full mt-2 bg-surface-raised border border-bord-strong rounded-xl shadow-2xl py-2 w-[180px] z-[300] overflow-hidden" style="opacity:0;pointer-events:none;transition:opacity .2s ease">
            <div id="user-dropdown-name" class="px-4 pt-1 pb-2 text-sm font-bold text-center" style="display:none"></div>
            <div id="user-dropdown-name-sep" class="border-t border-bord-light my-1" style="display:none"></div>
            <a href="/settings" class="block no-underline px-4 py-1.5 text-sm text-sub hover:bg-active hover:text-text">设置</a>
            <a href="#" onclick="location.reload();return false" class="block no-underline px-4 py-1.5 text-sm text-sub hover:bg-active hover:text-text">刷新页面</a>
            <div class="border-t border-bord-light my-1"></div>
          </div>
        </div>
      </div>
    </div>
    <style>
      @keyframes rainbow { 0%{background-position:0% 50%} 50%{background-position:100% 50%} 100%{background-position:0% 50%} }
      @keyframes imgFadeIn { from{opacity:0} to{opacity:1} }
      img, .grid > *, #results > div, #view > div, #appearances-section > div, #chars-section > div, #subjects-section > div, #stats-list > div, #history-area > div { animation: imgFadeIn .35s ease }
    </style>`;

    document.getElementById("btn-fetch").addEventListener("click", openFetch);

    capabilitiesReady().then(function () {
        applyFeatureVisibility(tb);
    });

    // 页面刷新后重新连接进行中的任务
    apiFor("task", "/v0/tasks").then(function (d) {
        if (d && d.data && d.data.length) {
            for (var i = 0; i < d.data.length; i++) {
                startProgress(d.data[i].id, d.data[i].label);
            }
        }
    });

    // 加载用户头像
    if (window.USERNAME) {
        api("/v0/users/" + window.USERNAME).then(function (u) {
            if (u && u.id) {
                var nameEl = document.getElementById("user-dropdown-name");
                nameEl.textContent = u.nickname || u.username;
                nameEl.style.display = "";
                document.getElementById(
                    "user-dropdown-name-sep"
                ).style.display = "";
                var av = document.getElementById("user-avatar");
                av.onerror = function () {
                    av.style.display = "none";
                    document.getElementById("user-placeholder").style.display =
                        "";
                };
                av.src =
                    API + "/v0/users/" + window.USERNAME + "/avatar?type=large";
                av.style.display = "";
                document.getElementById("user-placeholder").style.display =
                    "none";
            }
        });
    }

    // 全局事件总线（SSE）
    initEventBus();
});

function toggleUserMenu(e) {
    e.stopPropagation();
    var dd = document.getElementById("user-dropdown");
    var show = dd.style.opacity === "0" || dd.style.opacity === "";
    dd.style.opacity = show ? "1" : "0";
    dd.style.pointerEvents = show ? "auto" : "none";
}
document.addEventListener("click", function () {
    var dd = document.getElementById("user-dropdown");
    dd.style.opacity = "0";
    dd.style.pointerEvents = "none";
});

// Cmd+R / Ctrl+R 刷新页面
document.addEventListener("keydown", function (e) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "r") {
        e.preventDefault();
        location.reload();
    }
});

// "/" 打开搜索页；已在搜索页时聚焦输入框
document.addEventListener("keydown", function (e) {
    if (e.key !== "/" || e.metaKey || e.ctrlKey || e.altKey) return;
    var t = e.target;
    if (t) {
        var tag = t.tagName;
        if (
            tag === "INPUT" ||
            tag === "TEXTAREA" ||
            tag === "SELECT" ||
            t.isContentEditable
        )
            return;
    }
    var p = location.pathname;
    if (p === "/search" || p === "/search.html") {
        var q = document.getElementById("q");
        if (q && document.activeElement !== q) {
            e.preventDefault();
            q.focus();
        }
        return;
    }
    e.preventDefault();
    location.href = "/search";
});

// 拉取弹窗：输入行 + 操作按钮 + 危险区 + 二次确认
var dlgInited = false,
    confirmCb = null;

function showLightbox(src) {
    var el = document.getElementById("lightbox");
    if (el) el.remove();
    var overlay = document.createElement("div");
    overlay.id = "lightbox";
    overlay.style.cssText =
        "position:fixed;inset:0;z-index:9999;background:rgba(0,0,0,0);display:flex;align-items:center;justify-content:center;transition:background .25s ease";
    overlay.onclick = function (e) {
        if (e.target === overlay) closeLightbox();
    };
    var wrap = document.createElement("div");
    wrap.style.cssText =
        "position:relative;display:inline-block;max-width:90vw;max-height:90vh";
    var img = document.createElement("img");
    img.src = src;
    img.style.cssText =
        "max-width:90vw;max-height:90vh;object-fit:contain;border-radius:8px;opacity:0;transform:scale(.95);transition:opacity .25s ease,transform .25s ease";
    wrap.appendChild(img);
    var bar = document.createElement("div");
    bar.style.cssText =
        "position:fixed;top:16px;left:16px;z-index:10;opacity:0;transition:opacity .25s ease";
    var closeBtn = document.createElement("button");
    closeBtn.textContent = "关闭";
    closeBtn.style.cssText =
        "padding:6px 14px;border-radius:6px;border:1px solid rgba(255,255,255,.2);background:rgba(0,0,0,.5);color:rgba(255,255,255,.85);font-size:13px;cursor:pointer";
    closeBtn.onclick = closeLightbox;
    bar.appendChild(closeBtn);
    overlay.appendChild(bar);
    overlay.appendChild(wrap);
    document.body.appendChild(overlay);
    // 入场动画
    requestAnimationFrame(function () {
        overlay.style.background = "rgba(0,0,0,.85)";
        img.style.opacity = "1";
        img.style.transform = "scale(1)";
        bar.style.opacity = "1";
    });
    setTimeout(function () {
        img.style.transition = "";
    }, 300);
    function closeLightbox() {
        overlay.style.background = "rgba(0,0,0,0)";
        img.style.opacity = "0";
        img.style.transform = "scale(.95)";
        bar.style.opacity = "0";
        setTimeout(function () {
            overlay.remove();
        }, 250);
    }
    document.addEventListener("keydown", function handler(e) {
        if (e.key === "Escape") {
            closeLightbox();
            document.removeEventListener("keydown", handler);
        }
    });
    // 缩放和拖拽
    var scale = 1,
        tx = 0,
        ty = 0,
        dragging = false,
        dx = 0,
        dy = 0;
    function apply() {
        img.style.transform =
            "translate3d(" +
            Math.round(tx) +
            "px," +
            Math.round(ty) +
            "px,0) scale(" +
            scale +
            ")";
    }
    overlay.addEventListener(
        "wheel",
        function (ew) {
            ew.preventDefault();
            scale = Math.min(5, Math.max(0.5, scale * (1 - ew.deltaY * 0.005)));
            if (scale <= 1) {
                tx = 0;
                ty = 0;
                img.style.cursor = "";
            } else {
                img.style.cursor = "grab";
            }
            apply();
        },
        { passive: false }
    );
    img.addEventListener("mousedown", function (e) {
        if (scale <= 1) return;
        e.preventDefault();
        dragging = true;
        dx = e.clientX - tx;
        dy = e.clientY - ty;
        img.style.cursor = "grabbing";
    });
    window.addEventListener("mousemove", function (e) {
        if (!dragging) return;
        tx = e.clientX - dx;
        ty = e.clientY - dy;
        apply();
    });
    window.addEventListener("mouseup", function () {
        dragging = false;
        img.style.cursor = scale > 1 ? "grab" : "";
    });
    overlay.addEventListener("dblclick", function () {
        scale = 1;
        tx = 0;
        ty = 0;
        img.style.cursor = "";
        apply();
    });
}

function initDialog() {
    if (dlgInited) return;
    dlgInited = true;
    var dlg = document.createElement("div");
    dlg.id = "dlg-overlay";
    dlg.className =
        "fixed inset-0 z-[100] bg-black/60 flex items-center justify-center transition-opacity duration-200";
    dlg.style.opacity = "0";
    dlg.style.pointerEvents = "none";
    dlg.innerHTML =
        '<div class="relative bg-surface-alt border border-bord rounded-xl w-[460px] max-w-[90vw] overflow-hidden">' +
        '<div class="p-6 max-h-[90vh] overflow-y-auto">' +
        '<div class="flex items-center gap-3 mb-4">' +
        '<button id="btn-close-dlg" class="w-7 h-7 rounded-full border border-bord-strong flex items-center justify-center text-sub hover:text-text hover:border-bord-xstrong shrink-0 no-underline text-xs cursor-pointer bg-transparent" title="' +
        MSG.btnClose +
        '">&times;</button>' +
        '<h3 class="text-base font-bold">' +
        MSG.dialogTitle +
        "</h3></div>" +
        // ID 输入行
        '<label class="text-xs text-sub block mb-1">' +
        MSG.idLabel +
        "</label>" +
        '<div class="flex gap-2 mb-4">' +
        '<input id="fetch-input" placeholder="' +
        MSG.idPlaceholder +
        '" pattern="[0-9, ]*" class="flex-1 px-3 py-2 rounded-lg border border-bord bg-input text-sm">' +
        '<button id="btn-do-fetch" class="w-9 h-9 rounded-lg bg-pink text-white font-bold cursor-pointer border-0 hover:opacity-90 flex items-center justify-center" title="拉取">&check;</button></div>' +
        // Tracker 输入行
        '<label data-feature="tracker" class="text-xs text-sub block mb-1">' +
        MSG.trackerLabel +
        "</label>" +
        '<div data-feature="tracker" class="flex gap-2 mb-4">' +
        '<input id="tracker-input" placeholder="' +
        MSG.trackerPlaceholder +
        '" pattern="[a-zA-Z0-9_\\-]*" class="flex-1 px-3 py-2 rounded-lg border border-bord bg-input text-sm">' +
        '<button id="btn-tracker-fetch" class="w-9 h-9 rounded-lg bg-pink text-white font-bold cursor-pointer border-0 hover:opacity-90 flex items-center justify-center" title="拉取或创建">&check;</button></div>' +
        // 操作按钮
        '<div class="flex flex-col gap-2 mt-3 mb-2">' +
        '<div class="grid grid-cols-3 gap-2">' +
        '<button id="btn-user" class="px-3 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text">' +
        MSG.btnUserFetch +
        "</button>" +
        '<button id="btn-import-coll" data-feature="tracker" class="px-3 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text">' +
        MSG.btnImportColl +
        "</button>" +
        '<button id="btn-update" class="px-3 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text">' +
        MSG.btnUpdate +
        "</button>" +
        "</div>" +
        '<div class="grid grid-cols-3 gap-2">' +
        '<button id="btn-gap" class="px-3 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text">' +
        MSG.btnGap +
        "</button>" +
        '<button id="btn-meta" class="px-3 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text">' +
        MSG.btnMeta +
        "</button>" +
        '<button id="btn-all" class="px-3 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text">' +
        MSG.btnRefreshAll +
        "</button>" +
        "</div></div>" +
        // 危险区
        '<button id="btn-danger-toggle" class="w-full px-3 py-2 rounded-lg border border-[rgba(255,0,0,.3)] bg-transparent text-[#dc2626] text-sm cursor-pointer hover:bg-pink-bg">' +
        MSG.btnDangerZone +
        "</button>" +
        '<div id="danger-zone" class="hidden mt-2 grid grid-cols-3 gap-2">' +
        '<button id="btn-index" class="px-3 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text">' +
        MSG.btnRebuildIndex +
        "</button>" +
        '<button id="btn-rebuild-elo" data-feature="elo" class="px-3 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text">重建 ELO</button>' +
        '<button id="btn-deep" class="px-3 py-2 rounded-lg bg-[#dc2626] text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90">' +
        MSG.btnRebuildAll +
        "</button>" +
        "</div>" +
        "</div>" +
        // 确认遮罩
        '<div id="confirm-overlay" class="absolute inset-0 bg-black/70 rounded-xl flex items-center justify-center z-10 transition-opacity duration-200" style="display:none;opacity:0">' +
        '<div class="bg-surface-alt border border-bord rounded-lg p-5 w-[340px] max-w-[85vw]">' +
        '<p id="confirm-msg" class="text-sm mb-4 leading-relaxed"></p>' +
        '<button id="btn-confirm-exec" class="w-full px-4 py-2 rounded-lg bg-pink text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90 mb-2">' +
        MSG.btnExecute +
        "</button>" +
        '<button id="btn-confirm-cancel" class="w-full px-4 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active">' +
        MSG.btnCancel +
        "</button>" +
        "</div></div></div>";

    dlg.addEventListener("click", function (e) {
        if (e.target === dlg) closeFetch();
    });
    document.body.appendChild(dlg);
    applyFeatureVisibility(dlg);
    capabilitiesReady().then(function () {
        applyFeatureVisibility(dlg);
    });

    document
        .getElementById("btn-close-dlg")
        .addEventListener("click", closeFetch);
    document
        .getElementById("btn-confirm-cancel")
        .addEventListener("click", closeConfirm);

    // 危险区开关，只展开不收起
    document
        .getElementById("btn-danger-toggle")
        .addEventListener("click", function () {
            document.getElementById("danger-zone").classList.remove("hidden");
            this.remove();
        });

    // 绑定带确认的操作按钮
    document
        .getElementById("btn-do-fetch")
        .addEventListener("click", function () {
            var ids = document.getElementById("fetch-input").value.trim();
            if (!ids) return;
            confirmAction(MSG.confirmFetchId(ids), doFetch);
        });
    document
        .getElementById("btn-tracker-fetch")
        .addEventListener("click", function () {
            var name = document.getElementById("tracker-input").value.trim();
            if (!name) return;
            if (!/^[a-zA-Z0-9_-]+$/.test(name)) {
                showError(MSG.errInvalidTrackerName);
                return;
            }
            // 检查 Tracker 是否存在
            api("/v0/tracker").then(function (resp) {
                var list = (resp && resp.data) || [];
                var found = false;
                if (list)
                    for (var i = 0; i < list.length; i++) {
                        if (list[i].name === name) {
                            found = true;
                            break;
                        }
                    }
                if (found) {
                    confirmAction(
                        MSG.confirmTrackerFetch(name),
                        doTrackerFetch
                    );
                } else {
                    confirmAction(MSG.confirmTrackerCreate(name), function () {
                        fetch(API + "/v0/tracker/create", {
                            method: "POST",
                            headers: { "Content-Type": "application/json" },
                            body: JSON.stringify({ name: name })
                        }).then(function (r) {
                            return r.json().then(function (d) {
                                if (!r.ok) {
                                    showError(errText(d, r.status));
                                } else {
                                    showSuccess(MSG.trackerCreated(name));
                                }
                            });
                        });
                    });
                }
            });
        });
    document.getElementById("btn-user").addEventListener("click", function () {
        if (!window.USERNAME) {
            showError(MSG.errNoUsername);
            return;
        }
        confirmAction(MSG.confirmUserFetch(), doUserFetch);
    });
    document
        .getElementById("btn-import-coll")
        .addEventListener("click", function () {
            confirmAction(
                "将用户收藏列表导入为 Tracker，之后可参与其他拉取行为。",
                function () {
                    closeFetch();
                    fetch(API + "/v0/tracker/import-collections", {
                        method: "POST"
                    }).then(function (r) {
                        return r.json().then(function (d) {
                            if (!r.ok) showError(errText(d, r.status));
                            else showSuccess(MSG.importCollDone(d.count));
                        });
                    });
                }
            );
        });
    document
        .getElementById("btn-update")
        .addEventListener("click", function () {
            confirmAction(MSG.confirmUpdate, doFetchUpdate);
        });
    document.getElementById("btn-gap").addEventListener("click", function () {
        confirmAction(MSG.confirmGap, doFetchGap);
    });
    document.getElementById("btn-meta").addEventListener("click", function () {
        confirmAction(MSG.confirmMeta, doFetchMeta);
    });
    document.getElementById("btn-all").addEventListener("click", function () {
        confirmAction(MSG.confirmRefreshAll, doRefreshAll);
    });
    document.getElementById("btn-index").addEventListener("click", function () {
        confirmAction(MSG.confirmRebuildIndex, doFetchIndex);
    });
    document.getElementById("btn-deep").addEventListener("click", function () {
        confirmAction(MSG.confirmRebuildAll(), doDeepRebuild);
    });
    document
        .getElementById("btn-rebuild-elo")
        .addEventListener("click", function () {
            confirmAction(
                "将清空当前 ELO 分数并从对战历史完整重建",
                function () {
                    closeFetch();
                    fetch(API + "/v0/elo/rebuild", { method: "POST" })
                        .then(function (r) {
                            return r.json();
                        })
                        .then(function (d) {
                            if (d.status === "ok") {
                                showSuccess(
                                    "已重建 " + d.count + " 条 ELO 分数和计数"
                                );
                            } else {
                                showError("重建失败");
                            }
                        });
                }
            );
        });
}

function confirmAction(msg, cb) {
    document.getElementById("confirm-msg").textContent = msg;
    var el = document.getElementById("confirm-overlay");
    el.style.display = "flex";
    requestAnimationFrame(function () {
        el.style.opacity = "1";
    });
    confirmCb = cb;
    document.getElementById("btn-confirm-exec").style.display = "block";
    document.getElementById("btn-confirm-cancel").textContent = MSG.btnCancel;
    document.getElementById("btn-confirm-exec").onclick = function () {
        closeConfirm();
        cb();
    };
}
// 通知组件（统一右下角）
var _toastStack = [];
var _currentToast = null;
function _repositionToasts() {
    _toastStack = _toastStack.filter(function (t) {
        return !t.closed();
    });
    var bottom = 16;
    for (var i = _toastStack.length - 1; i >= 0; i--) {
        _toastStack[i].el.style.bottom = bottom + "px";
        bottom += _toastStack[i].el.getBoundingClientRect().height + 12;
    }
}
function _makeToast(
    title,
    body,
    titleBg,
    bodyBg,
    autoCloseSec,
    replace,
    closable
) {
    if (replace !== false && _currentToast) {
        _currentToast.close();
        _currentToast = null;
    }
    var el = document.createElement("div");
    el.className =
        "fixed right-4 z-[300] rounded-lg shadow-2xl transition-all duration-300 max-w-[60vw] min-w-[280px]";
    el.style.bottom = "16px";
    el.style.opacity = "0";
    el.style.transform = "translateY(8px)";
    el.innerHTML =
        '<div class="' +
        titleBg +
        ' text-white font-semibold px-4 py-2 rounded-t-lg flex items-center justify-between text-[14px]">' +
        '<span class="truncate">' +
        title +
        "</span>" +
        (closable !== false
            ? '<span class="cursor-pointer opacity-60 hover:opacity-100 text-lg leading-none shrink-0 ml-3">✕</span>'
            : "") +
        "</div>" +
        (body
            ? '<div class="' +
              bodyBg +
              " px-4 py-2.5 rounded-b-lg text-[13px] leading-relaxed max-h-[60vh] overflow-y-auto break-words" +
              (titleBg === "bg-toast-success" ? "" : " select-text") +
              '">' +
              body +
              "</div>"
            : "");
    document.body.appendChild(el);
    requestAnimationFrame(function () {
        el.style.opacity = "1";
        el.style.transform = "translateY(0)";
    });

    var timer = null,
        closed = false;
    function close() {
        if (closed) return;
        closed = true;
        if (timer) clearTimeout(timer);
        el.style.opacity = "0";
        el.style.transform = "translateY(8px)";
        _repositionToasts();
        setTimeout(function () {
            el.remove();
            if (_currentToast === el) _currentToast = null;
            _repositionToasts();
        }, 300);
    }
    var closeBtn = el.querySelector("span[class*='cursor-pointer']");
    if (closeBtn) closeBtn.onclick = close;
    if (autoCloseSec > 0) {
        var bar = document.createElement("div");
        bar.className =
            "h-1 rounded-full bg-[rgba(128,128,128,.2)] overflow-hidden mt-1.5";
        bar.innerHTML =
            '<div class="h-full rounded-full" style="width:100%;background:var(--c-toast-success);transition:width ' +
            autoCloseSec +
            's linear"></div>';
        var bodyEl = el.querySelector("div:last-child");
        if (bodyEl) bodyEl.appendChild(bar);
        setTimeout(function () {
            bar.querySelector("div").style.width = "0";
        }, 50);
        timer = setTimeout(close, autoCloseSec * 1000 + 100);
    }
    _toastStack.push({
        el: el,
        closed: function () {
            return closed;
        }
    });
    _currentToast = el;
    _repositionToasts();

    return {
        el: el,
        update: function (newTitle, newBody, newTitleBg, newBodyBg) {
            if (closed) return;
            if (newTitleBg)
                el.querySelector("div:first-child").className =
                    newTitleBg +
                    " text-white font-semibold px-4 py-2 rounded-t-lg flex items-center justify-between text-[14px]";
            if (newBodyBg && body)
                el.querySelector("div:last-child").className =
                    newBodyBg +
                    " px-4 py-2.5 rounded-b-lg text-[13px] leading-relaxed max-h-[60vh] overflow-y-auto break-words" +
                    (newTitleBg === "bg-toast-success" ? "" : " select-text");
            if (newTitle)
                el.querySelector("div:first-child span").textContent = newTitle;
            if (newBody && body)
                el.querySelector("div:last-child").innerHTML = newBody;
        },
        addClose: function () {
            var titleBar = el.querySelector("div:first-child");
            if (!titleBar.querySelector("span[class*='cursor-pointer']")) {
                var xBtn = document.createElement("span");
                xBtn.className =
                    "cursor-pointer opacity-60 hover:opacity-100 text-lg leading-none shrink-0 ml-3";
                xBtn.textContent = "✕";
                xBtn.onclick = close;
                titleBar.appendChild(xBtn);
            }
        },
        close: close,
        closed: function () {
            return closed;
        }
    };
}

function showError(msg) {
    _makeToast("错误", msg, "bg-toast-error", "bg-toast-error-bg", 0, false);
}

function showWarn(msg) {
    _makeToast(
        "警告",
        msg,
        "bg-toast-warning",
        "bg-toast-warning-bg",
        8,
        false
    );
}

function showSuccess(msg) {
    var bodyHTML =
        msg +
        '<div class="h-1 rounded-full bg-[rgba(128,128,128,.2)] overflow-hidden mt-1.5"><div class="h-full rounded-full" style="width:100%;background:var(--c-toast-success);transition:width 5s linear"></div></div>';
    var t = _makeToast(
        "完成",
        bodyHTML,
        "bg-toast-success",
        "bg-toast-success-bg",
        0
    );
    setTimeout(function () {
        var bar = t.el.querySelector(".h-full.rounded-full");
        if (bar) bar.style.width = "0";
        t.close();
    }, 5100);
}
function closeConfirm() {
    var el = document.getElementById("confirm-overlay");
    el.style.opacity = "0";
    setTimeout(function () {
        el.style.display = "none";
    }, 200);
    confirmCb = null;
}

function openFetch() {
    initDialog();
    var dlg = document.getElementById("dlg-overlay");
    dlg.style.display = "flex";
    requestAnimationFrame(function () {
        dlg.style.opacity = "1";
        dlg.style.pointerEvents = "auto";
    });
    document.getElementById("fetch-input").focus();
}
function closeFetch() {
    var dlg = document.getElementById("dlg-overlay");
    dlg.style.opacity = "0";
    dlg.style.pointerEvents = "none";
    setTimeout(function () {
        dlg.style.display = "none";
    }, 200);
    closeConfirm();
}

async function doFetch() {
    var v = document.getElementById("fetch-input").value.trim();
    if (!v) return;
    closeFetch();
    var ids = v
        .split(",")
        .map(function (s) {
            return parseInt(s.trim());
        })
        .filter(Boolean);
    if (!ids.length) return;
    var res = await fetch(API + "/v0/fetch/subject", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ids: ids })
    });
    var d = await readJSON(res);
    if (!res.ok || !d) {
        showError(errText(d, res.status));
        return;
    }
    if (d.task_id) startProgress(d.task_id);
}

async function doTrackerFetch() {
    var v = document.getElementById("tracker-input").value.trim();
    if (!v) return;
    var res = await fetch(API + "/v0/fetch/tracker", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ names: [v] })
    });
    var d = await readJSON(res);
    if (!res.ok || !d) {
        showError(errText(d, res.status));
        return;
    }
    closeFetch();
    if (d.task_id) startProgress(d.task_id);
}

async function doUserFetch() {
    var res = await fetch(API + "/v0/fetch/user", { method: "POST" });
    var d = await readJSON(res);
    if (!res.ok || !d) {
        showError(MSG.errUserFetchFailed + errText(d, res.status));
        return;
    }
    closeFetch();
    if (d.task_id) startProgress(d.task_id);
}

async function doRefreshAll() {
    closeFetch();
    var res = await fetch(API + "/v0/fetch/all", { method: "POST" });
    var d = await readJSON(res);
    if (!res.ok || !d) {
        showError(errText(d, res.status));
        return;
    }
    if (d.task_id) startProgress(d.task_id);
}

async function doDeepRebuild() {
    closeFetch();
    var res = await fetch(API + "/v0/fetch/deep", { method: "POST" });
    var d = await readJSON(res);
    if (!res.ok || !d) {
        showError(errText(d, res.status));
        return;
    }
    if (d.task_id) startProgress(d.task_id);
}

async function doFetchUpdate() {
    closeFetch();
    var res = await fetch(API + "/v0/fetch/update", { method: "POST" });
    var d = await readJSON(res);
    if (!res.ok || !d) {
        showError(errText(d, res.status));
        return;
    }
    if (d.task_id) startProgress(d.task_id);
}
async function doFetchGap() {
    closeFetch();
    var res = await fetch(API + "/v0/fetch/gap", { method: "POST" });
    var d = await readJSON(res);
    if (!res.ok || !d) {
        showError(errText(d, res.status));
        return;
    }
    if (d.task_id) startProgress(d.task_id);
}
async function doFetchMeta() {
    closeFetch();
    var res = await fetch(API + "/v0/fetch/meta", { method: "POST" });
    var d = await readJSON(res);
    if (!res.ok || !d) {
        showError(errText(d, res.status));
        return;
    }
    if (d.task_id) startProgress(d.task_id);
}

async function doFetchIndex() {
    closeFetch();
    var res = await fetch(API + "/v0/fetch/index", { method: "POST" });
    var d = await readJSON(res);
    if (!res.ok || !d) {
        showError(errText(d, res.status));
        return;
    }
    if (d.task_id) startProgress(d.task_id);
}

// 数据类任务完成后的默认行为：刷新当前页面。
function onFetchDone() {
    location.reload();
}

// 进度通知横幅（右下角浮动）
function startProgress(taskId, label) {
    var bodyHTML =
        '<div id="pb-detail" class="text-[11px] mb-1.5"></div>' +
        '<div class="h-1 rounded-full bg-[rgba(128,128,128,.2)] overflow-hidden"><div id="pb-fill" class="h-full rounded-full" style="width:0;background:var(--c-toast-progress)"></div></div>';
    var t = _makeToast(
        label || MSG.progressConnecting,
        bodyHTML,
        "bg-toast-progress",
        "bg-toast-progress-bg text-text",
        0,
        true,
        false
    );

    // 添加终止按钮
    var titleBar = t.el.querySelector("div:first-child");
    var cancelBtn = document.createElement("span");
    cancelBtn.className =
        "cursor-pointer opacity-60 hover:opacity-100 text-[12px] font-normal shrink-0 ml-3";
    cancelBtn.textContent = "终止";
    cancelBtn.onclick = function (e) {
        e.stopPropagation();
        fetch(API + "/v0/task/cancel", { method: "POST" });
        fill.style.transition = "none";
        fill.style.width = "100%";
        fill.style.background = "var(--c-toast-warning)";
        if (detail) detail.textContent = "任务已终止，请重新启动 Seshat 后端。";
        t.update(taskLabel, null, "bg-toast-warning", "bg-toast-warning-bg");
        t.addClose();
        cancelBtn.remove();
        evt.close();
    };
    // 远端模式不显示终止，避免杀掉对端进程
    if (!window.BACKEND_URL) titleBar.appendChild(cancelBtn);

    var fill = t.el.querySelector("#pb-fill");
    var detail = t.el.querySelector("#pb-detail");
    var taskLabel = label || "";

    var evt = new EventSource(API + "/v0/task/" + taskId);
    evt.onmessage = function (e) {
        var d = JSON.parse(e.data);
        if (d.label && !taskLabel) {
            taskLabel = d.label;
        }
        if (d.step === "cancelled") {
            fill.style.transition = "none";
            fill.style.width = "100%";
            fill.style.background = "var(--c-toast-warning)";
            if (detail) detail.textContent = "任务已终止";
            t.update(
                taskLabel,
                null,
                "bg-toast-warning",
                "bg-toast-warning-bg"
            );
            t.addClose();
            cancelBtn.remove();
            setTimeout(function () {
                fill.style.transition = "width 5s linear";
                fill.style.width = "0";
            }, 50);
            evt.close();
            setTimeout(function () {
                t.close();
            }, 5100);
            return;
        }
        if (d.step === "complete" || d.step === "done") {
            fill.style.transition = "none";
            fill.style.width = "100%";
            fill.style.background = "#22c55e";
            if (detail) detail.textContent = MSG.progressDone;
            t.update(
                taskLabel,
                null,
                "bg-toast-success",
                "bg-toast-success-bg"
            );
            t.addClose();
            cancelBtn.remove();
            setTimeout(function () {
                fill.style.transition = "width 5s linear";
                fill.style.width = "0";
            }, 50);
            evt.close();
            setTimeout(function () {
                t.close();
                if (typeof onFetchDone === "function") onFetchDone();
            }, 5100);
            return;
        }
        var title = taskLabel;
        if (d.phase && d.phases) title += " (" + d.phase + "/" + d.phases + ")";
        if (title !== taskLabel || (d.label && !label)) t.update(title);

        if (typeof d.done === "number" && d.total && d.step !== "subject") {
            fill.style.width = Math.round((d.done / d.total) * 100) + "%";
            var parts = [];
            if (d.phase_name) parts.push(d.phase_name);
            parts.push(d.done + "/" + d.total);
            if (d.speed) parts.push(d.speed);
            if (detail) detail.textContent = parts.join("，");
        } else if (d.status && d.step !== "subject") {
            if (detail) detail.textContent = d.status;
        }
    };
    evt.onerror = function () {
        evt.close();
    };
}

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

showConfigWarn();
