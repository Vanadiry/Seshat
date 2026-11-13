// 拉取弹窗：输入行 + 操作按钮 + 危险区 + 二次确认
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

var confirmReturnFocus = null;
