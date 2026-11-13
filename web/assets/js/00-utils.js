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

// 可点击元素的键盘激活
function clickKeyNav(e) {
    if (e.target !== e.currentTarget) return;
    if (e.key !== "Enter" && e.key !== " ") return;
    e.preventDefault();
    e.currentTarget.click();
}

// 返回按钮：有历史时回退，否则走 href
function backNav(e) {
    if (history.length > 1) {
        e.preventDefault();
        history.back();
    }
}

// 是否减少动态效果
function prefersReducedMotion() {
    return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

// 卡片键盘导航
function cardKeyNav(e) {
    if (e.target !== e.currentTarget) return;
    if (e.key !== "Enter" && e.key !== " ") return;
    var href = e.currentTarget.getAttribute("data-href");
    if (!href) return;
    e.preventDefault();
    location.href = href;
}

// 列表分页：追加渲染 + 展开更多 / 展开全部
function createPager(opts) {
    var container = document.getElementById(opts.containerId);
    var pager = document.getElementById("pager");
    var pageSize = opts.pageSize || 100;
    var unit = opts.unit || "个项目";
    var getItems = opts.getItems;
    var renderItem = opts.renderItem;
    var showing = 0;

    function more(n) {
        var items = getItems();
        var end = Math.min(showing + n, items.length);
        var html = "";
        for (var i = showing; i < end; i++) html += renderItem(items[i]);
        container.insertAdjacentHTML("beforeend", html);
        showing = end;
        update();
    }

    function all() {
        more(getItems().length - showing);
    }

    function update() {
        var items = getItems();
        if (items.length <= pageSize) {
            pager.innerHTML =
                '<span class="text-xs text-sub">共 ' +
                items.length +
                " " +
                unit +
                "</span>";
            return;
        }
        var html =
            '<span class="text-xs text-sub">当前展示 ' +
            showing +
            "，共 " +
            items.length +
            " " +
            unit +
            "</span>";
        if (showing < items.length) {
            html +=
                ' <button type="button" data-pager="more" class="ml-3 px-3 py-1 rounded-lg border border-bord bg-transparent text-sub text-xs cursor-pointer hover:bg-active hover:text-text">展开更多</button>';
            html +=
                ' <button type="button" data-pager="all" class="ml-2 px-3 py-1 rounded-lg border border-bord bg-transparent text-sub text-xs cursor-pointer hover:bg-active hover:text-text">展开全部</button>';
        }
        pager.innerHTML = html;
    }

    function reset() {
        showing = 0;
        container.innerHTML = "";
        pager.innerHTML = "";
        more(pageSize);
    }

    pager.addEventListener("click", function (e) {
        var btn = e.target.closest("[data-pager]");
        if (!btn) return;
        if (btn.getAttribute("data-pager") === "more") more(pageSize);
        else all();
    });

    return { reset: reset, more: more, all: all };
}

// 区块标题：竖条 + 标题 + 可选附加内容
function sectionTitle(text, opts) {
    opts = opts || {};
    return (
        '<div class="flex items-center ' +
        (opts.gap || "gap-2") +
        " " +
        (opts.outer || "mb-3") +
        '">' +
        '<span class="section-bar"></span>' +
        '<span class="font-bold' +
        (opts.label ? " " + opts.label : "") +
        '">' +
        text +
        "</span>" +
        (opts.extra || "") +
        "</div>"
    );
}

// 角色和人物列表卡片
function entityCard(kind, item) {
    var name = escapeHtml(item.name);
    var nameCn = escapeHtml(item.name_cn);
    var sub = subName(name, nameCn);
    return (
        '<a href="/' +
        kind +
        "?id=" +
        item.id +
        '" class="fade-in flex items-center gap-3 bg-surface rounded-lg overflow-hidden cursor-pointer border border-bord hover:border-pink hover:-translate-y-0.5 transition-all h-[72px] no-underline">' +
        '<img alt="" src="' +
        img(kind, item.id, "grid") +
        '" loading="lazy" ' +
        imgOnError(kind, item.id, "grid") +
        ' class="w-[72px] h-[72px] object-cover bg-surface shrink-0">' +
        '<div class="flex-1 min-w-0 pr-3 py-1">' +
        '<div class="text-sm font-semibold truncate">' +
        primaryName(name, nameCn) +
        "</div>" +
        (sub
            ? '<div class="text-xs text-sub truncate mt-0.5">' + sub + "</div>"
            : "") +
        "</div></a>"
    );
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
