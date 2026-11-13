// 信息栏键黑名单：这些字段只做 URL 和 BBCode，不做名字匹配
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

// 反转名称映射：{id: [names]} 到 {name: id}，后者覆盖
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

// 角色与人物合并后的 map，人物优先
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

// 条目自由匹配，最长优先，不限制边界
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

// 角色与人物自由匹配，仅 ≥3 字，最长优先
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

// 角色与人物分段匹配：按分隔符拆段，每段整体匹配
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

// 信息栏值处理：BBCode、URL、黑名单判定、保护集数、1a 条目自由匹配、1b ≥3 字自由匹配、2 分段匹配、还原集数
function processInfoboxValue(v, key) {
    var t = linkifyBBCode(v);
    t = linkifyURL(t);
    if (INFOBOX_NO_MATCH_KEYS.indexOf(key) >= 0) return t;
    // 保护集数括号，纯数字、逗号、连字符，避免内部数字被匹配
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

// linkifyPerson 供信息栏继续使用
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
            '<a href="$1" target="_blank" rel="noopener" class="text-pink hover:underline break-all"><img alt="" src="/assets/link.svg" class="w-3.5 h-3.5 inline-block mr-0.5 align-text-bottom">$1</a>'
        );
    }
    return parts.join("");
}

// linkifyBBCode：转换 [url=...]...[/url]，优先匹配本地名称
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
                '" target="_blank" rel="noopener" class="text-pink hover:underline"><img alt="" src="/assets/link.svg" class="w-3.5 h-3.5 inline-block mr-0.5 align-text-bottom">' +
                inner +
                "</a>"
            );
        }
    );
}

// formatSummary：依次做 BBCode、名称匹配、换行
function formatSummary(text) {
    if (!text) return text;
    return linkifyAllNames(linkifyBBCode(text)).replace(
        /\r\n|\n/g,
        '<br><span style="display:block;height:0.5em"></span>'
    );
}

// infoboxData：提取侧栏信息，包含信息栏和顶层字段
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
                        // 键值对：[{k: "纯假名", v: "值"}]
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
