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

// 本地 API 的 POST 请求统一带上令牌
(function () {
    if (!window.API_TOKEN || !window.fetch) return;
    var orig = window.fetch;
    window.fetch = function (input, init) {
        var url =
            typeof input === "string" ? input : (input && input.url) || "";
        var local =
            url.indexOf("/api/") === 0 ||
            url.indexOf(location.origin + "/api/") === 0;
        if (!local) return orig.call(window, input, init);
        var opts = init || {};
        if (String(opts.method || "GET").toUpperCase() !== "POST") {
            return orig.call(window, input, init);
        }
        var headers;
        if (opts.headers) {
            headers = new Headers(opts.headers);
        } else if (typeof Request !== "undefined" && input instanceof Request) {
            headers = new Headers(input.headers);
        } else {
            headers = new Headers();
        }
        headers.set("X-Seshat-Token", window.API_TOKEN);
        opts = Object.assign({}, opts, { headers: headers });
        return orig.call(window, input, opts);
    };
})();

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
    return "请求失败，状态码 " + status;
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
        '<img alt="" src="/assets/global.svg" class="' +
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
    globe.alt = "";
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
