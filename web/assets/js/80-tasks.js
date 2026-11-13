// 数据类任务完成后的默认行为：刷新当前页面。
function onFetchDone() {
    location.reload();
}

// 进度通知横幅（右下角浮动）
function _dispatchTaskEvent(name, detail) {
    window.dispatchEvent(new CustomEvent(name, { detail: detail }));
}

function startProgress(taskId, label) {
    var taskLabel = label || "";
    _dispatchTaskEvent("seshat:task-start", { id: taskId, label: taskLabel });
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
    var cancelBtn = document.createElement("button");
    cancelBtn.type = "button";
    cancelBtn.className =
        "cursor-pointer opacity-60 hover:opacity-100 text-[12px] font-normal shrink-0 ml-3 border-0 bg-transparent p-0 text-inherit font-[inherit]";
    cancelBtn.textContent = "终止";
    cancelBtn.onclick = function (e) {
        e.stopPropagation();
        fetch(API + "/v0/task/cancel/" + encodeURIComponent(taskId), {
            method: "POST"
        });
        _dispatchTaskEvent("seshat:task-progress", {
            id: taskId,
            data: { step: "cancelled" }
        });
        fill.style.transition = "none";
        fill.style.width = "100%";
        fill.style.background = "var(--c-toast-warning)";
        if (detail) detail.textContent = "任务已终止";
        t.update(taskLabel, null, "bg-toast-warning", "bg-toast-warning-bg");
        t.addClose();
        cancelBtn.remove();
        evt.close();
    };
    // 本地和远端后端都允许中止任务
    titleBar.appendChild(cancelBtn);

    var fill = t.el.querySelector("#pb-fill");
    var detail = t.el.querySelector("#pb-detail");

    var evt = new EventSource(API + "/v0/task/" + taskId);
    evt.onmessage = function (e) {
        var d = JSON.parse(e.data);
        _dispatchTaskEvent("seshat:task-progress", { id: taskId, data: d });
        if (d.label && !taskLabel) {
            taskLabel = d.label;
        }
        if (d.step === "error") {
            fill.style.transition = "none";
            fill.style.width = "100%";
            fill.style.background = "var(--c-toast-error)";
            if (detail) detail.textContent = d.status || "任务失败";
            t.update(taskLabel, null, "bg-toast-error", "bg-toast-error-bg");
            t.addClose();
            cancelBtn.remove();
            evt.close();
            setTimeout(function () {
                t.close();
            }, 5100);
            return;
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
        // start 事件只用于标记任务开始，进度以连接时的 snapshot 为准
        if (d.step === "start") return;
        var title = taskLabel;
        if (d.phase && d.phases) title += " " + d.phase + "/" + d.phases;
        if (title !== taskLabel || (d.label && !label)) t.update(title);

        if (typeof d.percent === "number") {
            fill.style.width = Math.max(0, Math.min(100, d.percent)) + "%";
            var percentParts = [];
            if (d.phase_name) percentParts.push(d.phase_name);
            if (typeof d.done === "number" && d.total) {
                var count = d.done + "/" + d.total;
                if (d.speed) count += "，" + d.speed;
                percentParts.push(count);
            }
            if (d.status) percentParts.push(d.status);
            percentParts.push(d.percent + "%");
            if (detail) detail.textContent = percentParts.join("，");
        } else if (
            typeof d.done === "number" &&
            d.total &&
            d.step !== "subject"
        ) {
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
        _dispatchTaskEvent("seshat:task-error", { id: taskId });
        evt.close();
    };
}
