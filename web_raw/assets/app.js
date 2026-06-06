// Seshat shared JS — top bar, fetch dialog, SSE progress, API helpers

// ── Theme: follow system when set to "auto" ──
var _themeMedia = window.matchMedia("(prefers-color-scheme: light)");
_themeMedia.addEventListener("change", function (e) {
    if ((localStorage.getItem("theme") || "auto") !== "auto") return;
    document.documentElement.classList.toggle("light", e.matches);
});

document.addEventListener("dragstart", function (e) {
    if (e.target.tagName === "IMG") e.preventDefault();
});

// Drag-to-scroll on .thin-scroll containers
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
    // Prevent click after drag
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

// ── UI text (edit here to customize dialog messages) ──
var MSG = {
    // Top bar
    customBackendWarn: "自定义后端模式，所有数据将从远程获取",
    fallbackWarn: "回退模式，缺失数据将从回退端点获取",
    bothWarn: "自定义后端和回退模式，请确保你明白此配置",
    // Fetch dialog
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
    confirmGap: "将遍历所有已缓存条目，检查并拉取有更新的人物/角色/剧集等数据。",
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

    // Confirm messages
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

    // Validation
    errInvalidTrackerName: "Tracker 名称仅允许大小写字母、数字、短横线和下划线。",
    errUserFetchFailed: "刷新用户数据失败: ",
    errNoUsername:
        "未在配置中设置用户名（[user] username），无法刷新用户数据。",

    // Progress
    progressConnecting: "连接中...",
    progressDone: "完成",

    // Infobox
    infoboxGender: "性别",
    infoboxBloodType: "血型",
    infoboxBirthday: "生日",
    bloodTypeMap: { 1: "A", 2: "B", 3: "AB", 4: "O" },

    // Episode types
    epTypes: {
        0: "本篇",
        1: "SP",
        2: "OP",
        3: "ED",
        4: "预告",
        5: "MAD",
        6: "其他"
    },
    // Subject types (used in character appearances)
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

// ── Loading bar ──
var _ldCount = 0, _ldTimer = null;
function _ldShow() {
    clearTimeout(_ldTimer);
    _ldTimer = setTimeout(function () {
        var bar = document.getElementById("load-bar");
        if (bar) bar.classList.add("active");
    }, 200);
}
function _ldHide() {
    _ldCount--;
    if (_ldCount <= 0) { _ldCount = 0; clearTimeout(_ldTimer);
        var bar = document.getElementById("load-bar");
        if (bar) bar.classList.remove("active");
    }
}

async function api(url) {
    _ldCount++; _ldShow();
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
    } finally { _ldHide(); }
}
// apiLocal always uses local backend for list/index data
function apiLocal(url) {
    return fetch("/api" + url).then(function (r) {
        return r.ok ? r.json() : null;
    });
}
function img(kind, id, size) {
    return API + "/v0/" + kind + "s/" + id + "/image?type=" + (size || "grid");
}
// imgOnError returns onerror attribute. If fallback is configured, first error retries via
// FALLBACK_URL; second error shows local placeholder. Without fallback, shows placeholder directly.
function imgOnError(kind, id, size) {
    if (!window.FALLBACK_URL) return "onerror=\"this.src='/assets/no-image.png'\"";
    var fb =
        window.FALLBACK_URL +
        "/v0/" +
        kind +
        "s/" +
        id +
        "/image?type=" +
        (size || "grid");
    return (
        "onerror=\"if(!this._r){this._r=1;this.src='" +
        fb +
        "';addRemoteGlobe(this);}else{this.src='/assets/no-image.png';}\""
    );
}

// ── Remote globe marker helpers ──
// globeIcon returns an <img> tag for the globe marker. Use in headers and section titles.
function globeIcon(cls) {
    return (
        '<img src="/assets/global.svg" class="' +
        (cls || "w-5 h-5") +
        ' shrink-0" title="此数据来自远程">'
    );
}
// addRemoteGlobe wraps a remote-fallback image and adds a globe overlay at top-left corner.
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

// ── Infobox key blacklist：这些字段只做 URL/BBCode，不做名字匹配 ──
var INFOBOX_NO_MATCH_KEYS = [
    "放送开始",
    "播放开始",
    "放送结束",
    "播放结束",
    "放送星期",
    "播放星期",
    "话数"
];

// ── Name → ID lookup maps (subjects/characters/persons) ──
var _subjectMap = null,
    _charMap = null,
    _personMap = null;
function loadPersonMap() {
    if (_personMap && Object.keys(_personMap).length) return Promise.resolve();
    return Promise.all([
        api("/v0/subjects/name"),
        api("/v0/characters/name"),
        api("/v0/persons/name")
    ]).then(function (results) {
        _subjectMap = invertNameMap(results[0]);
        _charMap = invertNameMap(results[1]);
        _personMap = invertNameMap(results[2]);
        _personRegex = null;
        _charPersonMerged = null;
    });
}

// ── 反转 name map：{id: [names]} → {name: id}，后者覆盖 ──
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

// ── Merged char + person map（person 优先）──
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

// ── Subject 自由匹配（最长优先，不限制边界）──
function linkifySubjectFree(text) {
    return linkifyByMap(text, _subjectMap, "/subject?id=");
}

// ── 分段匹配：直接查合并 map，集数已在顶层保护 ──
function matchSegmentCore(seg, merged) {
    seg = seg.trim();
    if (!seg) return seg;
    var id = merged[seg];
    if (!id) return seg;
    return (
        '<a href="' + id + '" class="text-pink hover:underline">' + seg + "</a>"
    );
}

// ── Char+Person 自由匹配（仅 ≥3 字，最长优先）──
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

// ── 分隔符集合 ──
var _SEPARATOR_RE = /([、,，\/\(\)（）\[\]])/;

// ── Char+Person 分段匹配：按分隔符拆段，每段整体匹配 ──
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

// ── Infobox value 处理：BBCode → URL → 黑名单判定 → 保护集数 → 1a subject 自由匹配 → 1b ≥3 字自由匹配 → 2 分段匹配 → 还原集数 ──
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

// ── Regex building for name matching ──
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

// Keep existing linkifyPerson for backward compat (used in infobox)
function linkifyPerson(text) {
    return linkifyByMap(text, _personMap, "/person?id=");
}

// Apply all three name matchers, skipping text already inside <a> tags
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

// ── linkifyURL：检测 URL 并转为可点击链接 ──
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

// ── linkifyBBCode：转换 [url=...]...[/url]，优先匹配本地 name ──
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

// ── formatSummary：BBCode → name 匹配 → 换行 ──
function formatSummary(text) {
    if (!text) return text;
    return linkifyAllNames(linkifyBBCode(text)).replace(
        /\r\n|\n/g,
        '<br><span style="display:block;height:0.5em"></span>'
    );
}

// ── infoboxData：提取侧栏信息（infobox + 顶层字段）──
function infoboxData(d) {
    var items = [];
    // Top-level fields
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
    // infobox array
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
                        // Value-only objects: [{v: "クラナド"}]
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
                        // Fallback: show first string from array
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
                    // Plain string array
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

// ── primaryName / subName：根据 display_lang 配置决定主副标题顺序 ──
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

// ── extractCN：从 infobox 提取简体中文名 ──
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

// ── Loading bar + top bar ──
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
    var custom = !!window.BACKEND_URL;
    var fallback = !!window.FALLBACK_URL;
    var both = custom && fallback;
    var topBg = both
        ? "bg-warn-bar-purple"
        : custom
          ? "bg-warn-bar-red"
          : fallback
            ? "bg-warn-bar-amber"
            : "bg-surface-raised";
    var topBord = custom || fallback ? "border-bord-strong" : "border-bord";
    var warnText = both
        ? MSG.bothWarn
        : custom
          ? MSG.customBackendWarn
          : fallback
            ? MSG.fallbackWarn
            : "";
    var navStyle = function (href) {
        var c = navCls(href);
        if (custom || fallback) {
            if (c.indexOf("bg-") >= 0) return c;
            return "text-text hover:bg-active";
        }
        return c;
    };
    tb.innerHTML = `
    <div class="${topBg} border-b ${topBord}">
      <div class="max-w-[1200px] mx-auto px-5 flex items-center gap-3 h-12 relative">
        <h1 class="text-lg font-bold cursor-pointer shrink-0" onclick="location.href='/'">Seshat</h1>
        <nav class="flex gap-1 ml-2 items-center">
          <a href="/" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/")}">动画</a>
          <a href="/characters" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/characters")}">角色</a>
          <a href="/persons" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/persons")}">人物</a>
          <a href="/tags" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/tags")}">标签</a>
          <a href="/stats" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/stats")}">统计</a>
        </nav>
        <div class="flex-1"></div>
        ${warnText ? '<span class="absolute left-1/2 -translate-x-1/2 text-[#FFCA28] text-sm font-bold pointer-events-none">' + warnText + "</span>" : ""}
        <nav class="flex gap-1 items-center">
          <a href="/rating" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/rating")}">评分</a>
          <a href="/search" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/search")}">搜索</a>
        </nav>
        <button id="btn-fetch" class="px-4 py-1.5 rounded-lg bg-pink text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90">管理数据</button>
        <div class="relative">
          <img id="user-avatar" src="" class="w-8 h-8 rounded-lg object-cover bg-surface cursor-pointer shrink-0" onclick="toggleUserMenu(event)" style="display:none">
          <div id="user-placeholder" class="w-8 h-8 rounded-lg cursor-pointer shrink-0" onclick="toggleUserMenu(event)" style="background:linear-gradient(135deg,#ff6b6b,#feca57,#48dbfb,#ff9ff3,#54a0ff);background-size:300% 300%;animation:rainbow 3s ease infinite"></div>
          <div id="user-dropdown" class="absolute right-0 top-full mt-2 bg-surface-raised border border-bord-strong rounded-xl shadow-2xl py-2 w-[180px] z-[300] overflow-hidden" style="opacity:0;pointer-events:none;transition:opacity .2s ease">
            <div id="user-dropdown-name" class="px-4 pt-1 pb-2 text-sm font-bold text-center" style="display:none"></div>
            <div id="user-dropdown-name-sep" class="border-t border-bord-light my-1" style="display:none"></div>
            <a href="/settings" class="block no-underline px-4 py-1.5 text-sm text-sub hover:bg-active hover:text-text">设置</a>
            <div class="border-t border-bord-light my-1"></div>
            <a href="https://github.com/Vanadiry/Seshat" target="_blank" class="block no-underline px-4 py-1.5 text-sm text-sub hover:bg-active hover:text-text">项目 GitHub</a>
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

    // Reconnect to active tasks after page refresh
    api("/v0/tasks").then(function (d) {
        if (d && d.tasks && d.tasks.length) {
            for (var i = 0; i < d.tasks.length; i++) {
                startProgress(d.tasks[i].id, d.tasks[i].label);
            }
        }
    });

    // Load user avatar
    if (window.USERNAME) {
        api("/v0/users/" + window.USERNAME).then(function (u) {
            if (u && u.id) {
                var nameEl = document.getElementById("user-dropdown-name");
                nameEl.textContent = u.nickname || u.username;
                nameEl.style.display = "";
                document.getElementById("user-dropdown-name-sep").style.display = "";
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

// ── 拉取弹窗：输入行 + 操作按钮 + 危险区 + 二次确认 ──
var dlgInited = false,
    confirmCb = null;

function showLightbox(src) {
    var el = document.getElementById("lightbox");
    if (el) el.remove();
    var overlay = document.createElement("div");
    overlay.id = "lightbox";
    overlay.style.cssText = "position:fixed;inset:0;z-index:9999;background:rgba(0,0,0,0);display:flex;align-items:center;justify-content:center;transition:background .25s ease";
    overlay.onclick = function (e) { if (e.target === overlay) closeLightbox(); };
    var wrap = document.createElement("div");
    wrap.style.cssText = "position:relative;display:inline-block;max-width:90vw;max-height:90vh";
    var img = document.createElement("img");
    img.src = src;
    img.style.cssText = "max-width:90vw;max-height:90vh;object-fit:contain;border-radius:8px;opacity:0;transform:scale(.95);transition:opacity .25s ease,transform .25s ease";
    wrap.appendChild(img);
    var bar = document.createElement("div");
    bar.style.cssText = "position:fixed;top:16px;left:16px;z-index:10;opacity:0;transition:opacity .25s ease";
    var closeBtn = document.createElement("button");
    closeBtn.textContent = "关闭";
    closeBtn.style.cssText = "padding:6px 14px;border-radius:6px;border:1px solid rgba(255,255,255,.2);background:rgba(0,0,0,.5);color:rgba(255,255,255,.85);font-size:13px;cursor:pointer";
    closeBtn.onclick = closeLightbox;
    bar.appendChild(closeBtn);
    overlay.appendChild(bar);
    overlay.appendChild(wrap);
    document.body.appendChild(overlay);
    // animate in
    requestAnimationFrame(function () {
        overlay.style.background = "rgba(0,0,0,.85)";
        img.style.opacity = "1";
        img.style.transform = "scale(1)";
        bar.style.opacity = "1";
    });
    setTimeout(function () { img.style.transition = ""; }, 300);
    function closeLightbox() {
        overlay.style.background = "rgba(0,0,0,0)";
        img.style.opacity = "0";
        img.style.transform = "scale(.95)";
        bar.style.opacity = "0";
        setTimeout(function () { overlay.remove(); }, 250);
    }
    document.addEventListener("keydown", function handler(e) {
        if (e.key === "Escape") { closeLightbox(); document.removeEventListener("keydown", handler); }
    });
    // Zoom + drag
    var scale = 1, tx = 0, ty = 0, dragging = false, dx = 0, dy = 0;
    function apply() { img.style.transform = "translate3d(" + Math.round(tx) + "px," + Math.round(ty) + "px,0) scale(" + scale + ")"; }
    overlay.addEventListener("wheel", function (ew) {
        ew.preventDefault();
        scale = Math.min(5, Math.max(0.5, scale * (1 - ew.deltaY * 0.005)));
        if (scale <= 1) { tx = 0; ty = 0; img.style.cursor = ""; }
        else { img.style.cursor = "grab"; }
        apply();
    }, { passive: false });
    img.addEventListener("mousedown", function (e) {
        if (scale <= 1) return; e.preventDefault();
        dragging = true; dx = e.clientX - tx; dy = e.clientY - ty;
        img.style.cursor = "grabbing";
    });
    window.addEventListener("mousemove", function (e) {
        if (!dragging) return;
        tx = e.clientX - dx; ty = e.clientY - dy;
        apply();
    });
    window.addEventListener("mouseup", function () {
        dragging = false; img.style.cursor = scale > 1 ? "grab" : "";
    });
    overlay.addEventListener("dblclick", function () {
        scale = 1; tx = 0; ty = 0; img.style.cursor = ""; apply();
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
        '<div class="bg-surface-alt border border-bord rounded-xl p-6 w-[460px] max-w-[90vw] max-h-[90vh] overflow-y-auto">' +
        '<div class="flex items-center gap-3 mb-4">' +
        '<button id="btn-close-dlg" class="w-7 h-7 rounded-full border border-bord-strong flex items-center justify-center text-sub hover:text-text hover:border-bord-xstrong shrink-0 no-underline text-xs cursor-pointer bg-transparent" title="' +
        MSG.btnClose +
        '">&times;</button>' +
        '<h3 class="text-base font-bold">' +
        MSG.dialogTitle +
        "</h3></div>" +
        // ID input row
        '<label class="text-xs text-sub block mb-1">' +
        MSG.idLabel +
        "</label>" +
        '<div class="flex gap-2 mb-4">' +
        '<input id="fetch-input" placeholder="' +
        MSG.idPlaceholder +
        '" pattern="[0-9, ]*" class="flex-1 px-3 py-2 rounded-lg border border-bord bg-input text-sm">' +
        '<button id="btn-do-fetch" class="w-9 h-9 rounded-lg bg-pink text-white font-bold cursor-pointer border-0 hover:opacity-90 flex items-center justify-center" title="拉取">&check;</button></div>' +
        // Tracker input row
        '<label class="text-xs text-sub block mb-1">' +
        MSG.trackerLabel +
        "</label>" +
        '<div class="flex gap-2 mb-4">' +
        '<input id="tracker-input" placeholder="' +
        MSG.trackerPlaceholder +
        '" pattern="[a-zA-Z0-9_\\-]*" class="flex-1 px-3 py-2 rounded-lg border border-bord bg-input text-sm">' +
        '<button id="btn-tracker-fetch" class="w-9 h-9 rounded-lg bg-pink text-white font-bold cursor-pointer border-0 hover:opacity-90 flex items-center justify-center" title="拉取或创建">&check;</button></div>' +
        // Action buttons
        '<div class="flex flex-col gap-2 mt-3 mb-2">' +
        '<div class="grid grid-cols-3 gap-2">' +
        '<button id="btn-user" class="px-3 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text">' +
        MSG.btnUserFetch +
        "</button>" +
        '<button id="btn-import-coll" class="px-3 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text">' +
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
        // Danger zone
        '<button id="btn-danger-toggle" class="w-full px-3 py-2 rounded-lg border border-[rgba(255,0,0,.3)] bg-transparent text-[#dc2626] text-sm cursor-pointer hover:bg-pink-bg">' +
        MSG.btnDangerZone +
        "</button>" +
        '<div id="danger-zone" class="hidden mt-2 grid grid-cols-3 gap-2">' +
        '<button id="btn-index" class="px-3 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text">' +
        MSG.btnRebuildIndex +
        "</button>" +
        '<button id="btn-rebuild-elo" class="px-3 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text">重建 ELO</button>' +
        '<button id="btn-deep" class="px-3 py-2 rounded-lg bg-[#dc2626] text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90">' +
        MSG.btnRebuildAll +
        "</button>" +
        "</div>" +
        // Confirmation overlay
        '<div id="confirm-overlay" class="absolute inset-0 bg-black/70 rounded-xl flex items-center justify-center z-10 transition-opacity duration-200" style="display:none;opacity:0">' +
        '<div class="bg-surface-alt border border-bord rounded-lg p-5 w-[340px] max-w-[85vw]">' +
        '<p id="confirm-msg" class="text-sm mb-4 leading-relaxed"></p>' +
        '<button id="btn-confirm-exec" class="w-full px-4 py-2 rounded-lg bg-pink text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90 mb-2">' +
        MSG.btnExecute +
        "</button>" +
        '<button id="btn-confirm-cancel" class="w-full px-4 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active">' +
        MSG.btnCancel +
        "</button>" +
        "</div></div></div></div>";

    dlg.addEventListener("click", function (e) {
        if (e.target === dlg) closeFetch();
    });
    document.body.appendChild(dlg);

    document
        .getElementById("btn-close-dlg")
        .addEventListener("click", closeFetch);
    document
        .getElementById("btn-confirm-cancel")
        .addEventListener("click", closeConfirm);

    // Danger zone toggle (expand only, no collapse)
    document
        .getElementById("btn-danger-toggle")
        .addEventListener("click", function () {
            document.getElementById("danger-zone").classList.remove("hidden");
            this.remove();
        });

    // Wire up action buttons with confirmation
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
            // Check if tracker exists
            api("/v0/tracker").then(function (list) {
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
                        })
                            .then(function (r) {
                                return r.json();
                            })
                            .then(function (d) {
                                if (d.error) {
                                    showError(d.error);
                                } else {
                                    showError(MSG.trackerCreated(name));
                                }
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
                    })
                        .then(function (r) {
                            return r.json();
                        })
                        .then(function (d) {
                            if (d.error) showError(d.error);
                            else showError(MSG.importCollDone(d.count));
                        });
                }
            );
        });
    document
        .getElementById("btn-update")
        .addEventListener("click", function () {
            confirmAction(MSG.confirmUpdate, doFetchUpdate);
        });
    document
        .getElementById("btn-gap")
        .addEventListener("click", function () {
            confirmAction(MSG.confirmGap, doFetchGap);
        });
    document
        .getElementById("btn-meta")
        .addEventListener("click", function () {
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
                                showSuccess("已重建 " + d.count + " 条 ELO 分数和计数");
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
    requestAnimationFrame(function () { el.style.opacity = "1"; });
    confirmCb = cb;
    document.getElementById("btn-confirm-exec").style.display = "block";
    document.getElementById("btn-confirm-cancel").textContent = MSG.btnCancel;
    document.getElementById("btn-confirm-exec").onclick = function () {
        closeConfirm();
        cb();
    };
}
// ── 通知组件（统一右下角）──
var _currentToast = null;
function _makeToast(title, body, titleBg, bodyBg, autoCloseSec, replace, closable) {
    if (replace !== false && _currentToast) { _currentToast.remove(); _currentToast = null; }
    var el = document.createElement("div");
    el.className = "fixed bottom-4 right-4 z-[300] rounded-lg shadow-2xl transition-all duration-300 max-w-[380px] min-w-[300px]";
    el.style.opacity = "0";
    el.style.transform = "translateY(8px)";
    if (replace === false && _currentToast) {
        var curRect = _currentToast.getBoundingClientRect();
        el.style.bottom = (window.innerHeight - curRect.top + 12) + "px";
    }
    el.innerHTML =
        '<div class="' + titleBg + ' text-white font-semibold px-4 py-2 rounded-t-lg flex items-center justify-between text-[14px]">' +
        '<span class="truncate">' + title + '</span>' +
        (closable !== false ? '<span class="cursor-pointer opacity-60 hover:opacity-100 text-lg leading-none shrink-0 ml-3">✕</span>' : "") +
        '</div>' +
        (body ? '<div class="' + bodyBg + ' px-4 py-2.5 rounded-b-lg text-[13px] leading-relaxed">' + body + '</div>' : "");
    document.body.appendChild(el);
    requestAnimationFrame(function () { el.style.opacity = "1"; el.style.transform = "translateY(0)"; });

    var timer = null, closed = false;
    var fill = el.querySelector("#t-fill");
    function close() { if (closed) return; closed = true;
        if (timer) clearTimeout(timer);
        el.style.opacity = "0"; el.style.transform = "translateY(8px)";
        setTimeout(function () { el.remove(); if (_currentToast === el) _currentToast = null; }, 300);
    }
    var closeBtn = el.querySelector("span[class*='cursor-pointer']");
    if (closeBtn) closeBtn.onclick = close;
    if (autoCloseSec > 0) {
        fill.style.width = "100%";
        fill.style.background = "#68a868";
        setTimeout(function () { fill.style.transition = "width " + autoCloseSec + "s linear"; fill.style.width = "0"; }, 50);
        timer = setTimeout(close, autoCloseSec * 1000 + 100);
    }
    if (replace !== false) _currentToast = el;

    return {
        el: el,
        update: function (newTitle, newBody, newTitleBg, newBodyBg) {
            if (closed) return;
            if (newTitleBg) el.querySelector("div:first-child").className = newTitleBg + " text-white font-semibold px-4 py-2 rounded-t-lg flex items-center justify-between text-[14px]";
            if (newBodyBg && body) el.querySelector("div:last-child").className = newBodyBg + " px-4 py-2.5 rounded-b-lg text-[13px] leading-relaxed";
            if (newTitle) el.querySelector("div:first-child span").textContent = newTitle;
            if (newBody && body) el.querySelector("div:last-child").innerHTML = newBody;
        },
        addClose: function () {
            var titleBar = el.querySelector("div:first-child");
            if (!titleBar.querySelector("span[class*='cursor-pointer']")) {
                var xBtn = document.createElement("span");
                xBtn.className = "cursor-pointer opacity-60 hover:opacity-100 text-lg leading-none shrink-0 ml-3";
                xBtn.textContent = "✕";
                xBtn.onclick = close;
                titleBar.appendChild(xBtn);
            }
        },
        close: close,
        closed: function () { return closed; }
    };
}

function showError(msg) {
    _makeToast("错误", msg, "bg-toast-error", "bg-toast-error-bg", 0, false);
}

function showSuccess(msg) {
    var bodyHTML = msg + '<div class="h-1 rounded-full bg-[rgba(128,128,128,.2)] overflow-hidden mt-1.5"><div class="h-full rounded-full" style="width:100%;background:var(--c-toast-success);transition:width 5s linear"></div></div>';
    var t = _makeToast("完成", bodyHTML, "bg-toast-success", "bg-toast-success-bg", 5);
    setTimeout(function () { t.el.querySelector(".h-full.rounded-full").style.width = "0"; }, 50);
}
function closeConfirm() {
    var el = document.getElementById("confirm-overlay");
    el.style.opacity = "0";
    setTimeout(function () { el.style.display = "none"; }, 200);
    confirmCb = null;
}

function openFetch() {
    initDialog();
    var dlg = document.getElementById("dlg-overlay");
    dlg.style.display = "flex";
    requestAnimationFrame(function () { dlg.style.opacity = "1"; dlg.style.pointerEvents = "auto"; });
    document.getElementById("fetch-input").focus();
}
function closeFetch() {
    var dlg = document.getElementById("dlg-overlay");
    dlg.style.opacity = "0";
    dlg.style.pointerEvents = "none";
    setTimeout(function () { dlg.style.display = "none"; }, 200);
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
    var d = await res.json();
    if (d.error) {
        showError(d.error);
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
    var d = await res.json();
    if (d.error) {
        showError(d.error);
        return;
    }
    closeFetch();
    if (d.task_id) startProgress(d.task_id);
}

async function doUserFetch() {
    var res = await fetch(API + "/v0/fetch/user", { method: "POST" });
    var d = await res.json().catch(function () {
        return { error: "请求失败 (" + res.status + ")" };
    });
    if (d.error) {
        showError(MSG.errUserFetchFailed + d.error);
        return;
    }
    closeFetch();
    if (d.task_id) startProgress(d.task_id);
}

async function doRefreshAll() {
    closeFetch();
    var res = await fetch(API + "/v0/fetch/all", { method: "POST" });
    var d = await res.json();
    if (d.error) {
        showError(d.error);
        return;
    }
    if (d.task_id) startProgress(d.task_id);
}

async function doDeepRebuild() {
    closeFetch();
    var res = await fetch(API + "/v0/fetch/deep", { method: "POST" });
    var d = await res.json();
    if (d.error) {
        showError(d.error);
        return;
    }
    if (d.task_id) startProgress(d.task_id);
}

async function doFetchUpdate() {
    closeFetch();
    var res = await fetch(API + "/v0/fetch/update", { method: "POST" });
    var d = await res.json();
    if (d.error) { showError(d.error); return; }
    if (d.task_id) startProgress(d.task_id);
}
async function doFetchGap() {
    closeFetch();
    var res = await fetch(API + "/v0/fetch/gap", { method: "POST" });
    var d = await res.json();
    if (d.error) { showError(d.error); return; }
    if (d.task_id) startProgress(d.task_id);
}
async function doFetchMeta() {
    closeFetch();
    var res = await fetch(API + "/v0/fetch/meta", { method: "POST" });
    var d = await res.json();
    if (d.error) { showError(d.error); return; }
    if (d.task_id) startProgress(d.task_id);
}

async function doFetchIndex() {
    closeFetch();
    var res = await fetch(API + "/v0/fetch/index", { method: "POST" });
    var d = await res.json();
    if (d.error) {
        showError(d.error);
        return;
    }
    if (d.task_id) startProgress(d.task_id);
}

// ── 进度通知横幅（右下角浮动）──
function startProgress(taskId, label) {
    var bodyHTML = '<div id="pb-detail" class="text-[11px] mb-1.5"></div>' +
        '<div class="h-1 rounded-full bg-[rgba(128,128,128,.2)] overflow-hidden"><div id="pb-fill" class="h-full rounded-full" style="width:0;background:var(--c-toast-progress)"></div></div>';
    var t = _makeToast(label || MSG.progressConnecting, bodyHTML, "bg-toast-progress", "bg-toast-progress-bg text-text", 0, true, false);

    // Add cancel button
    var titleBar = t.el.querySelector("div:first-child");
    var cancelBtn = document.createElement("span");
    cancelBtn.className = "cursor-pointer opacity-60 hover:opacity-100 text-[12px] font-normal shrink-0 ml-3";
    cancelBtn.textContent = "终止";
    cancelBtn.onclick = function (e) { e.stopPropagation();
        fetch(API + "/v0/task/cancel", { method: "POST" });
        fill.style.transition = "none"; fill.style.width = "100%";
        fill.style.background = "var(--c-toast-warning)";
        if (detail) detail.textContent = "任务已终止，请重新启动 Seshat。";
        t.update(taskLabel, null, "bg-toast-warning", "bg-toast-warning-bg");
        t.addClose();
        cancelBtn.remove();
        evt.close();
    };
    titleBar.appendChild(cancelBtn);

    var fill = t.el.querySelector("#pb-fill");
    var detail = t.el.querySelector("#pb-detail");
    var taskLabel = label || "", _errShown = false;

    var evt = new EventSource(API + "/v0/task/" + taskId);
    evt.onmessage = function (e) {
        var d = JSON.parse(e.data);
        if (d.label && !taskLabel) { taskLabel = d.label; }
        if (d.error && !_errShown) {
            _errShown = true; showError(d.error);
        }
        if (d.step === "cancelled") {
            fill.style.transition = "none"; fill.style.width = "100%";
            fill.style.background = "var(--c-toast-warning)";
            if (detail) detail.textContent = "任务已终止";
            t.update(taskLabel, null, "bg-toast-warning", "bg-toast-warning-bg");
            t.addClose();
            cancelBtn.remove();
            setTimeout(function () { fill.style.transition = "width 5s linear"; fill.style.width = "0"; }, 50);
            evt.close();
            setTimeout(function () { t.close(); }, 5100);
            return;
        }
        if (d.step === "complete" || d.step === "done") {
            fill.style.transition = "none"; fill.style.width = "100%"; fill.style.background = "#22c55e";
            if (detail) detail.textContent = MSG.progressDone;
            t.update(taskLabel, null, "bg-toast-success", "bg-toast-success-bg");
            t.addClose();
            cancelBtn.remove();
            setTimeout(function () { fill.style.transition = "width 5s linear"; fill.style.width = "0"; }, 50);
            evt.close();
            setTimeout(function () { t.close(); if (typeof onFetchDone === "function") onFetchDone(); }, 5100);
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
    evt.onerror = function () { evt.close(); };
}
