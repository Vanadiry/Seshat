!(function () {
    var t = localStorage.getItem("theme") || "auto";
    if (
        t === "light" ||
        (t === "auto" &&
            window.matchMedia("(prefers-color-scheme: light)").matches)
    )
        document.documentElement.classList.add("light");
})();
