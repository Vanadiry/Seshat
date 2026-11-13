// 页面总滚动条：隐藏原生后自行绘制，悬停或滚动时出现
function initPageScrollbar() {
    if (window.__pageScrollbarInited) return;
    if (!window.matchMedia || !window.matchMedia("(pointer: fine)").matches) {
        return;
    }
    var bar = document.createElement("div");
    bar.id = "page-scrollbar";
    bar.setAttribute("aria-hidden", "true");
    var thumb = document.createElement("div");
    thumb.className = "thumb";
    bar.appendChild(thumb);
    document.body.appendChild(bar);
    window.__pageScrollbarInited = true;

    var dragging = false;
    var dragStartY = 0;
    var dragStartScroll = 0;
    var dragPerPixel = 1;
    var hideTimer = 0;
    var raf = 0;

    function docEl() {
        return document.scrollingElement || document.documentElement;
    }
    function headerEl() {
        var topbar = document.getElementById("topbar");
        if (!topbar) return null;
        return topbar.closest(".sticky") || topbar;
    }
    function metrics() {
        var doc = docEl();
        var header = headerEl();
        var top = header
            ? Math.max(0, Math.round(header.getBoundingClientRect().bottom))
            : 0;
        var height = Math.max(0, window.innerHeight - top);
        bar.style.top = top + "px";
        bar.style.height = height + "px";
        return {
            height: height,
            scrollHeight: doc.scrollHeight,
            clientHeight: doc.clientHeight,
            scrollTop: window.pageYOffset || doc.scrollTop || 0
        };
    }
    function update() {
        if (document.body.style.overflow === "hidden") {
            hide(true);
            return;
        }
        var m = metrics();
        if (m.height <= 0 || m.scrollHeight <= m.clientHeight + 1) {
            hide(true);
            return;
        }
        var thumbHeight = Math.max(
            24,
            Math.round((m.clientHeight / m.scrollHeight) * m.height)
        );
        var maxTop = Math.max(0, m.height - thumbHeight);
        var maxScroll = Math.max(1, m.scrollHeight - m.clientHeight);
        var top = Math.round(
            (Math.min(m.scrollTop, maxScroll) / maxScroll) * maxTop
        );
        thumb.style.height = thumbHeight + "px";
        thumb.style.top = top + "px";
    }
    function show() {
        update();
        var m = metrics();
        if (m.height <= 0 || m.scrollHeight <= m.clientHeight + 1) return;
        bar.classList.add("visible");
        clearTimeout(hideTimer);
        if (!dragging) {
            hideTimer = setTimeout(function () {
                hide(false);
            }, 900);
        }
    }
    function hide(force) {
        if (dragging && !force) return;
        clearTimeout(hideTimer);
        bar.classList.remove("visible");
    }
    function onScroll() {
        if (raf) return;
        raf = requestAnimationFrame(function () {
            raf = 0;
            show();
        });
    }
    function onPointerMove(e) {
        if (window.innerWidth - e.clientX <= 24) show();
    }
    function onResize() {
        update();
        if (bar.classList.contains("visible")) show();
    }
    function startDrag(e) {
        e.preventDefault();
        var m = metrics();
        var thumbHeight = thumb.getBoundingClientRect().height;
        var maxTop = Math.max(1, m.height - thumbHeight);
        var maxScroll = Math.max(1, m.scrollHeight - m.clientHeight);
        dragging = true;
        dragStartY = e.clientY;
        dragStartScroll = m.scrollTop;
        dragPerPixel = maxScroll / maxTop;
        show();
        document.body.style.userSelect = "none";

        function move(ev) {
            var next =
                dragStartScroll + (ev.clientY - dragStartY) * dragPerPixel;
            next = Math.max(0, Math.min(maxScroll, next));
            window.scrollTo(0, next);
        }
        function end() {
            dragging = false;
            document.body.style.userSelect = "";
            window.removeEventListener("pointermove", move);
            window.removeEventListener("pointerup", end);
            window.removeEventListener("pointercancel", end);
            show();
        }
        window.addEventListener("pointermove", move);
        window.addEventListener("pointerup", end);
        window.addEventListener("pointercancel", end);
    }

    thumb.addEventListener("pointerdown", startDrag);
    bar.addEventListener("pointerenter", function () {
        clearTimeout(hideTimer);
    });
    bar.addEventListener("pointerleave", function () {
        if (dragging) return;
        hideTimer = setTimeout(function () {
            hide(false);
        }, 300);
    });
    bar.addEventListener("click", function (e) {
        if (e.target === thumb) return;
        var r = thumb.getBoundingClientRect();
        var dir = e.clientY < r.top ? -1 : 1;
        window.scrollBy({
            top: dir * window.innerHeight * 0.9,
            behavior: "smooth"
        });
        show();
    });
    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onResize);
    window.addEventListener("pointermove", onPointerMove, { passive: true });
    if (window.ResizeObserver) {
        var ro = new ResizeObserver(function () {
            update();
            if (bar.classList.contains("visible")) show();
        });
        ro.observe(document.body);
        ro.observe(document.documentElement);
        var header = headerEl();
        if (header) ro.observe(header);
    }
    update();
}

if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initPageScrollbar);
} else {
    initPageScrollbar();
}

showConfigWarn();
