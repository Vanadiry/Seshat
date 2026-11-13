// 拉取弹窗：输入行、操作按钮、危险区和二次确认
var confirmCb = null;

function showLightbox(src) {
    var el = document.getElementById("lightbox");
    if (el) el.remove();
    var reduceMotion = prefersReducedMotion();
    var returnFocus = document.activeElement;
    var overlay = document.createElement("div");
    overlay.id = "lightbox";
    overlay.setAttribute("role", "dialog");
    overlay.setAttribute("aria-modal", "true");
    overlay.setAttribute("aria-label", "图片预览");
    overlay.style.cssText =
        "position:fixed;inset:0;z-index:9999;background:rgba(0,0,0,0);display:flex;align-items:center;justify-content:center;" +
        (reduceMotion ? "" : "transition:background .25s ease");
    overlay.onclick = function (e) {
        if (e.target === overlay) closeLightbox();
    };
    var wrap = document.createElement("div");
    wrap.style.cssText =
        "position:relative;display:inline-block;max-width:90vw;max-height:90vh;max-height:min(90vh,90dvh)";
    var img = document.createElement("img");
    img.alt = "";
    img.src = src;
    img.style.cssText =
        "max-width:90vw;max-height:90vh;max-height:min(90vh,90dvh);object-fit:contain;border-radius:8px;opacity:0;transform:scale(.95);" +
        (reduceMotion
            ? ""
            : "transition:opacity .25s ease,transform .25s ease");
    wrap.appendChild(img);
    var bar = document.createElement("div");
    bar.style.cssText =
        "position:fixed;top:calc(16px + var(--safe-top));left:calc(16px + var(--safe-left));z-index:10;opacity:0;" +
        (reduceMotion ? "" : "transition:opacity .25s ease");
    var closeBtn = document.createElement("button");
    closeBtn.textContent = "关闭";
    closeBtn.style.cssText =
        "padding:10px 18px;border-radius:6px;border:1px solid rgba(255,255,255,.2);background:rgba(0,0,0,.5);color:rgba(255,255,255,.85);font-size:14px;cursor:pointer";
    closeBtn.onclick = closeLightbox;
    bar.appendChild(closeBtn);
    overlay.appendChild(bar);
    overlay.appendChild(wrap);
    document.body.appendChild(overlay);
    closeBtn.focus();
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
    var lightboxClosed = false;
    function handleLightboxKey(e) {
        if (e.key === "Escape") {
            e.preventDefault();
            closeLightbox();
            return;
        }
        if (e.key === "Tab") {
            e.preventDefault();
            closeBtn.focus();
        }
    }
    function closeLightbox() {
        if (lightboxClosed) return;
        lightboxClosed = true;
        document.removeEventListener("keydown", handleLightboxKey);
        window.removeEventListener("pointermove", onPointerMove);
        window.removeEventListener("pointerup", onPointerUp);
        window.removeEventListener("pointercancel", onPointerUp);
        overlay.style.background = "rgba(0,0,0,0)";
        img.style.opacity = "0";
        img.style.transform = "scale(.95)";
        bar.style.opacity = "0";
        setTimeout(
            function () {
                overlay.remove();
                if (
                    returnFocus &&
                    document.contains(returnFocus) &&
                    typeof returnFocus.focus === "function"
                ) {
                    returnFocus.focus();
                }
            },
            reduceMotion ? 0 : 250
        );
    }
    document.addEventListener("keydown", handleLightboxKey);
    // 缩放和拖拽：滚轮、双指捏合、单指平移、双击缩放
    var scale = 1,
        tx = 0,
        ty = 0;
    var pointers = new Map();
    var pinchStartDist = 0,
        pinchStartScale = 1,
        pinchStartTx = 0,
        pinchStartTy = 0,
        pinchStartMx = 0,
        pinchStartMy = 0,
        pinchStartCx = 0,
        pinchStartCy = 0;
    var dragStartX = 0,
        dragStartY = 0,
        dragStartTx = 0,
        dragStartTy = 0,
        dragging = false;
    var downX = 0,
        downY = 0,
        moved = false,
        lastTapTime = 0;

    overlay.style.touchAction = "none";
    img.draggable = false;

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

    function imageCenter() {
        var r = wrap.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
    }

    // 放大后限制平移范围，图片小于视口时保持居中
    function clampPan() {
        var r = wrap.getBoundingClientRect();
        var maxX = Math.max(0, (r.width * scale - window.innerWidth) / 2);
        var maxY = Math.max(0, (r.height * scale - window.innerHeight) / 2);
        tx = Math.max(-maxX, Math.min(maxX, tx));
        ty = Math.max(-maxY, Math.min(maxY, ty));
    }

    // 以屏幕坐标点为中心缩放
    function setScale(next, px, py) {
        next = Math.min(5, Math.max(1, next));
        if (next === scale) return;
        var c = imageCenter();
        var k = next / scale;
        tx = px - c.x - (px - c.x - tx) * k;
        ty = py - c.y - (py - c.y - ty) * k;
        scale = next;
        if (scale <= 1) {
            tx = 0;
            ty = 0;
        }
        clampPan();
        img.style.cursor = scale > 1 ? "grab" : "";
        apply();
    }

    function distance(a, b) {
        return Math.hypot(a.x - b.x, a.y - b.y) || 1;
    }

    function midpoint(a, b) {
        return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
    }

    function onPointerDown(e) {
        if (bar.contains(e.target)) return;
        if (e.pointerType === "mouse" && e.button !== 0) return;
        pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
        if (pointers.size === 1) {
            downX = e.clientX;
            downY = e.clientY;
            moved = false;
            dragging = scale > 1;
            dragStartX = e.clientX;
            dragStartY = e.clientY;
            dragStartTx = tx;
            dragStartTy = ty;
        } else if (pointers.size === 2) {
            dragging = false;
            var pts = Array.from(pointers.values());
            var c = imageCenter();
            pinchStartDist = distance(pts[0], pts[1]);
            pinchStartScale = scale;
            pinchStartTx = tx;
            pinchStartTy = ty;
            var m = midpoint(pts[0], pts[1]);
            pinchStartMx = m.x;
            pinchStartMy = m.y;
            pinchStartCx = c.x;
            pinchStartCy = c.y;
        }
        try {
            overlay.setPointerCapture(e.pointerId);
        } catch (_) {}
    }

    function onPointerMove(e) {
        if (!pointers.has(e.pointerId)) return;
        pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
        if (
            Math.abs(e.clientX - downX) > 8 ||
            Math.abs(e.clientY - downY) > 8
        ) {
            moved = true;
        }
        if (pointers.size >= 2) {
            var pts = Array.from(pointers.values());
            var next = Math.min(
                5,
                Math.max(
                    1,
                    pinchStartScale *
                        (distance(pts[0], pts[1]) / pinchStartDist)
                )
            );
            var k = next / pinchStartScale;
            var m = midpoint(pts[0], pts[1]);
            tx =
                m.x -
                pinchStartCx -
                (pinchStartMx - pinchStartCx - pinchStartTx) * k;
            ty =
                m.y -
                pinchStartCy -
                (pinchStartMy - pinchStartCy - pinchStartTy) * k;
            scale = next;
            if (scale <= 1) {
                tx = 0;
                ty = 0;
            }
            clampPan();
            img.style.cursor = scale > 1 ? "grab" : "";
            apply();
            return;
        }
        if (dragging && scale > 1) {
            tx = dragStartTx + (e.clientX - dragStartX);
            ty = dragStartTy + (e.clientY - dragStartY);
            clampPan();
            apply();
        }
    }

    function onPointerUp(e) {
        if (!pointers.has(e.pointerId)) return;
        pointers.delete(e.pointerId);
        if (pointers.size === 1) {
            var p = Array.from(pointers.values())[0];
            dragging = scale > 1;
            dragStartX = p.x;
            dragStartY = p.y;
            dragStartTx = tx;
            dragStartTy = ty;
            return;
        }
        if (pointers.size === 0) {
            dragging = false;
            if (!moved && e.pointerType !== "mouse") {
                var now = Date.now();
                if (now - lastTapTime < 300) {
                    setScale(scale > 1 ? 1 : 2.5, e.clientX, e.clientY);
                    lastTapTime = 0;
                    return;
                }
                lastTapTime = now;
            }
            img.style.cursor = scale > 1 ? "grab" : "";
        }
    }

    overlay.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", onPointerUp);
    window.addEventListener("pointercancel", onPointerUp);

    overlay.addEventListener(
        "wheel",
        function (ew) {
            ew.preventDefault();
            setScale(scale * (1 - ew.deltaY * 0.005), ew.clientX, ew.clientY);
        },
        { passive: false }
    );

    // 鼠标双击复位
    overlay.addEventListener("dblclick", function () {
        setScale(1, window.innerWidth / 2, window.innerHeight / 2);
    });
}

var confirmReturnFocus = null;
