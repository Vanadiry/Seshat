// 通知组件（统一右下角）
var _toastStack = [];
var _currentToast = null;

// 读取安全区变量的像素值
function _safeInset(name) {
    var v = getComputedStyle(document.documentElement).getPropertyValue(name);
    var n = parseFloat(v);
    return isNaN(n) ? 0 : n;
}

function _repositionToasts() {
    _toastStack = _toastStack.filter(function (t) {
        return !t.closed();
    });
    var bottom = 16 + _safeInset("--safe-bottom");
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
    closable,
    merge
) {
    var mergeKey = merge ? titleBg + "\n" + title + "\n" + body : "";
    if (merge) {
        for (var i = _toastStack.length - 1; i >= 0; i--) {
            var existing = _toastStack[i];
            if (!existing.closed() && existing.key === mergeKey) {
                existing.bump();
                return existing;
            }
        }
    }
    if (replace !== false && _currentToast) {
        _currentToast.close();
        _currentToast = null;
    }
    var reduceMotion = prefersReducedMotion();
    var el = document.createElement("div");
    el.className =
        "fixed right-4 z-[300] rounded-lg shadow-2xl min-w-[240px]" +
        (reduceMotion ? "" : " transition-all duration-300");
    el.style.bottom = "16px";
    el.style.right = "calc(16px + var(--safe-right))";
    el.style.maxWidth = "calc(100vw - 32px - var(--safe-right))";
    el.style.opacity = "0";
    el.style.transform = "translateY(8px)";
    var countHTML = merge
        ? '<span class="toast-count text-xs font-semibold opacity-80 shrink-0 ml-2" style="display:none"></span>'
        : "";
    var closeHTML =
        closable !== false
            ? '<button type="button" data-toast-close aria-label="关闭" class="cursor-pointer opacity-60 hover:opacity-100 text-lg leading-none shrink-0 ml-2 w-8 h-8 flex items-center justify-center border-0 bg-transparent p-0 text-inherit font-[inherit]">✕</button>'
            : "";
    var rightHTML =
        countHTML || closeHTML
            ? '<span class="flex items-center shrink-0">' +
              countHTML +
              closeHTML +
              "</span>"
            : "";
    el.innerHTML =
        '<div class="' +
        titleBg +
        ' text-white font-semibold px-4 py-2 rounded-t-lg flex items-center justify-between text-[14px]">' +
        '<span class="truncate min-w-0 flex-1">' +
        title +
        "</span>" +
        rightHTML +
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
    var closeBtn = el.querySelector("[data-toast-close]");
    if (closeBtn) closeBtn.onclick = close;
    var barFill = null;
    function armAutoClose() {
        if (autoCloseSec <= 0) return;
        if (timer) clearTimeout(timer);
        if (barFill) {
            barFill.style.transition = "none";
            barFill.style.width = "100%";
            void barFill.offsetWidth;
            barFill.style.transition = reduceMotion
                ? "none"
                : "width " + autoCloseSec + "s linear";
            barFill.style.width = "0";
        }
        timer = setTimeout(close, autoCloseSec * 1000 + 100);
    }
    if (autoCloseSec > 0) {
        var bar = document.createElement("div");
        bar.className =
            "h-1 rounded-full bg-[rgba(128,128,128,.2)] overflow-hidden mt-1.5";
        bar.innerHTML =
            '<div class="h-full rounded-full" style="width:100%;background:var(--c-toast-success);' +
            (reduceMotion
                ? ""
                : "transition:width " + autoCloseSec + "s linear") +
            '"></div>';
        var bodyEl = el.querySelector("div:last-child");
        if (bodyEl) bodyEl.appendChild(bar);
        barFill = bar.querySelector("div");
        armAutoClose();
    }
    var count = 1;
    var countEl = merge ? el.querySelector(".toast-count") : null;
    function bump() {
        count++;
        if (countEl) {
            countEl.textContent = "×" + count;
            countEl.style.display = "";
        }
        if (autoCloseSec > 0) armAutoClose();
        if (!prefersReducedMotion()) {
            el.animate(
                [
                    { transform: "translateX(0)" },
                    { transform: "translateX(-4px)" },
                    { transform: "translateX(4px)" },
                    { transform: "translateX(-3px)" },
                    { transform: "translateX(3px)" },
                    { transform: "translateX(0)" }
                ],
                { duration: 350, easing: "ease" }
            );
        }
        _repositionToasts();
    }
    var toast = {
        el: el,
        key: mergeKey,
        bump: bump,
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
            if (!titleBar.querySelector("[data-toast-close]")) {
                var xBtn = document.createElement("button");
                xBtn.type = "button";
                xBtn.setAttribute("data-toast-close", "");
                xBtn.setAttribute("aria-label", "关闭");
                xBtn.className =
                    "cursor-pointer opacity-60 hover:opacity-100 text-lg leading-none shrink-0 ml-3 border-0 bg-transparent p-0 text-inherit font-[inherit]";
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
    _toastStack.push(toast);
    _currentToast = toast;
    _repositionToasts();

    return toast;
}

function showError(msg) {
    _makeToast(
        "错误",
        msg,
        "bg-toast-error",
        "bg-toast-error-bg",
        0,
        false,
        true,
        true
    );
}

function showWarn(msg) {
    _makeToast(
        "警告",
        msg,
        "bg-toast-warning",
        "bg-toast-warning-bg",
        8,
        false,
        true,
        true
    );
}

function showInfo(msg) {
    _makeToast(
        "提示",
        msg,
        "bg-toast-progress",
        "bg-toast-progress-bg text-text",
        5,
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
    document.removeEventListener("keydown", confirmKeydown, true);
    var target = confirmReturnFocus;
    confirmReturnFocus = null;
    if (
        target &&
        document.contains(target) &&
        typeof target.focus === "function"
    ) {
        target.focus();
    }
    setTimeout(function () {
        el.style.display = "none";
    }, 200);
    confirmCb = null;
}

async function doImportCollections() {
    var res = await fetch(API + "/v0/tracker/import-collections", {
        method: "POST"
    });
    var d = await readJSON(res);
    if (!res.ok) {
        showError(errText(d, res.status));
        return;
    }
    showSuccess(MSG.importCollDone(d.count));
    window.dispatchEvent(new Event("seshat:trackers-changed"));
}

async function doRebuildELO() {
    var res = await fetch(API + "/v0/elo/rebuild", { method: "POST" });
    var d = await readJSON(res);
    if (!res.ok || !d || d.status !== "ok") {
        showError("重建失败");
        return;
    }
    showSuccess("已重建 " + d.count + " 条 ELO 分数和计数");
}

async function doUserFetch() {
    var res = await fetch(API + "/v0/fetch/user", { method: "POST" });
    var d = await readJSON(res);
    if (!res.ok || !d) {
        showError(MSG.errUserFetchFailed + errText(d, res.status));
        return;
    }
    if (d.task_id) startProgress(d.task_id);
}

async function doDeepRebuild() {
    var res = await fetch(API + "/v0/fetch/deep", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ confirm_clean: true })
    });
    var d = await readJSON(res);
    if (!res.ok || !d) {
        showError(errText(d, res.status));
        return;
    }
    if (d.task_id) startProgress(d.task_id);
}

async function doFetchIndex() {
    var res = await fetch(API + "/v0/fetch/index", { method: "POST" });
    var d = await readJSON(res);
    if (!res.ok || !d) {
        showError(errText(d, res.status));
        return;
    }
    if (d.task_id) startProgress(d.task_id);
}
