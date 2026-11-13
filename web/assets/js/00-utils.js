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
