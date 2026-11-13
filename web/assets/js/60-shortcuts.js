// Cmd+R / Ctrl+R 刷新页面
document.addEventListener("keydown", function (e) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "r") {
        e.preventDefault();
        location.reload();
    }
});

// "/" 打开搜索页；已在搜索页时聚焦输入框
document.addEventListener("keydown", function (e) {
    if (e.key !== "/" || e.metaKey || e.ctrlKey || e.altKey) return;
    var t = e.target;
    if (t) {
        var tag = t.tagName;
        if (
            tag === "INPUT" ||
            tag === "TEXTAREA" ||
            tag === "SELECT" ||
            t.isContentEditable
        )
            return;
    }
    var p = location.pathname;
    if (p === "/search" || p === "/search.html") {
        var q = document.getElementById("q");
        if (q && document.activeElement !== q) {
            e.preventDefault();
            q.focus();
        }
        return;
    }
    e.preventDefault();
    location.href = "/search";
});
