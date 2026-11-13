// 确认弹窗内循环 Tab，Esc 关闭
function confirmKeydown(e) {
    if (e.key === "Escape") {
        e.preventDefault();
        closeConfirm();
        return;
    }
    if (e.key !== "Tab") return;
    var dialog = document.querySelector('#confirm-overlay [role="dialog"]');
    if (!dialog) return;
    var nodes = Array.prototype.slice
        .call(
            dialog.querySelectorAll(
                'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
            )
        )
        .filter(function (node) {
            return !node.disabled && node.offsetParent !== null;
        });
    if (!nodes.length) {
        e.preventDefault();
        return;
    }
    var first = nodes[0];
    var last = nodes[nodes.length - 1];
    if (!dialog.contains(document.activeElement)) {
        e.preventDefault();
        first.focus();
        return;
    }
    if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
    }
}

function confirmAction(msg, cb) {
    confirmReturnFocus = document.activeElement;
    document.getElementById("confirm-msg").textContent = msg;
    var el = document.getElementById("confirm-overlay");
    el.style.display = "flex";
    requestAnimationFrame(function () {
        el.style.opacity = "1";
    });
    confirmCb = cb;
    document.getElementById("btn-confirm-exec").style.display = "block";
    document.getElementById("btn-confirm-cancel").textContent = MSG.btnCancel;
    document.getElementById("btn-confirm-cancel").onclick = closeConfirm;
    document.getElementById("btn-confirm-exec").onclick = function () {
        closeConfirm();
        cb();
    };
    document.removeEventListener("keydown", confirmKeydown, true);
    document.addEventListener("keydown", confirmKeydown, true);
    document.getElementById("btn-confirm-cancel").focus();
}
