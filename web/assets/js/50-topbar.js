// 加载条和顶栏
(function () {
    var bar = document.createElement("div");
    bar.id = "load-bar";
    document.body.appendChild(bar);
})();

(function () {
    const tb = document.getElementById("topbar");
    if (!tb) return;

    const path = location.pathname;
    // 窄屏顶栏显示当前所处位置
    function sectionName(p) {
        if (p === "/" || p === "/subject") return "动画";
        if (p === "/characters" || p === "/character") return "角色";
        if (p === "/persons" || p === "/person") return "人物";
        if (p === "/tags" || p === "/tag") return "标签";
        if (p === "/stats") return "统计";
        if (p === "/rating") return "评分";
        if (p === "/search") return "搜索";
        if (p === "/settings") return "设置";
        if (p === "/manage") return "管理数据";
        return "";
    }
    function navCls(href) {
        var p = path;
        if (p === "/subject") p = "/";
        else if (p === "/character") p = "/characters";
        else if (p === "/person") p = "/persons";
        else if (p === "/tag") p = "/tags";
        if (href === "/" && p === "/") return "bg-active text-text";
        if (href !== "/" && p.startsWith(href)) return "bg-active text-text";
        return "hover:bg-active hover:text-text";
    }

    tb.style.cssText = "position:sticky;top:0;z-index:50";
    var navStyle = navCls;
    tb.innerHTML = `
    <div class="bg-surface-raised border-b border-bord">
      <div class="max-w-[1200px] mx-auto px-4 md:px-6 flex items-center gap-2 md:gap-3 h-12 relative overflow-x-auto">
        <h1 class="text-lg font-bold shrink-0"><a href="/" class="no-underline">Seshat</a></h1>
        <button id="nav-menu-btn" type="button" class="md:hidden flex items-center gap-1 min-w-0 px-2 py-1 rounded-lg border-0 bg-transparent text-text cursor-pointer focus-visible:outline focus-visible:outline-2 focus-visible:outline-pink" aria-expanded="false" aria-controls="nav-menu" aria-label="打开导航菜单">
          <span id="nav-menu-label" class="text-sm truncate">${sectionName(path) || "菜单"}</span>
          <svg id="nav-menu-chevron" viewBox="0 0 24 24" class="w-3.5 h-3.5 shrink-0 transition-transform" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="m6 9 6 6 6-6"></path>
          </svg>
        </button>
        <nav class="hidden md:flex gap-1 ml-2 items-center">
          <a href="/" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/")}">动画</a>
          <a href="/characters" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/characters")}">角色</a>
          <a href="/persons" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/persons")}">人物</a>
          <a href="/tags" data-feature="tags" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/tags")}">标签</a>
          <a href="/stats" data-feature="stats" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/stats")}">统计</a>
        </nav>
        <div class="flex-1"></div>
        <nav class="hidden md:flex gap-1 items-center">
          <a href="/rating" data-feature="elo" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/rating")}">评分</a>
          <a href="/search" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/search")}">搜索</a>
          <a href="/settings" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/settings")}">设置</a>
          <a id="btn-fetch" href="/manage" data-feature-all="fetch,task" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle("/manage")}">管理数据</a>
        </nav>
        <a href="/search" class="md:hidden shrink-0 px-2 py-1 rounded-lg text-sm text-text hover:bg-active no-underline" aria-label="搜索" title="搜索">搜索</a>
        <button type="button" class="relative group w-8 h-8 shrink-0 cursor-pointer rounded-lg border-0 bg-transparent p-0 focus-visible:outline focus-visible:outline-2 focus-visible:outline-pink" aria-label="刷新页面" onclick="location.reload()">
          <img alt="" id="user-avatar" src="" class="w-8 h-8 rounded-lg object-cover bg-surface shrink-0" style="display:none">
          <div id="user-placeholder" class="w-8 h-8 rounded-lg shrink-0" style="background:linear-gradient(135deg,#ff6b6b,#feca57,#48dbfb,#ff9ff3,#54a0ff);background-size:300% 300%;animation:rainbow 3s ease infinite"></div>
          <span class="pointer-events-none absolute inset-0 rounded-lg bg-black/45 opacity-0 group-hover:opacity-100 group-focus:opacity-100 transition-opacity flex items-center justify-center">
            <svg viewBox="0 0 24 24" class="w-4 h-4 text-white" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M21 12a9 9 0 1 1-2.64-6.36"></path>
              <path d="M21 3v6h-6"></path>
            </svg>
          </span>
        </button>
        <button type="button" id="header-collapse-btn" class="header-collapse-btn w-8 h-8 shrink-0 cursor-pointer rounded-lg border-0 bg-transparent p-0 text-sub hover:text-text hover:bg-active focus-visible:outline focus-visible:outline-2 focus-visible:outline-pink" aria-label="收起顶栏" title="收起顶栏">
          <svg viewBox="0 0 24 24" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="m18 15-6-6-6 6"></path>
          </svg>
        </button>
      </div>
    </div>
    <div id="nav-menu" class="hidden md:hidden absolute top-full left-0 right-0 z-50">
      <div class="max-w-[1200px] mx-auto px-3 pt-1">
        <div class="bg-surface-raised border border-bord rounded-xl shadow-lg p-2 flex flex-col gap-1">
          <div class="flex flex-wrap gap-1">
            <a href="/" class="no-underline px-3 py-2 rounded-lg text-sm ${navStyle("/")}">动画</a>
            <a href="/characters" class="no-underline px-3 py-2 rounded-lg text-sm ${navStyle("/characters")}">角色</a>
            <a href="/persons" class="no-underline px-3 py-2 rounded-lg text-sm ${navStyle("/persons")}">人物</a>
            <a href="/tags" data-feature="tags" class="no-underline px-3 py-2 rounded-lg text-sm ${navStyle("/tags")}">标签</a>
            <a href="/stats" data-feature="stats" class="no-underline px-3 py-2 rounded-lg text-sm ${navStyle("/stats")}">统计</a>
          </div>
          <div class="flex flex-wrap gap-1 pt-1 border-t border-bord-light">
            <a href="/rating" data-feature="elo" class="no-underline px-3 py-2 rounded-lg text-sm ${navStyle("/rating")}">评分</a>
            <a href="/settings" class="no-underline px-3 py-2 rounded-lg text-sm ${navStyle("/settings")}">设置</a>
            <a href="/manage" data-feature-all="fetch,task" class="no-underline px-3 py-2 rounded-lg text-sm ${navStyle("/manage")}">管理数据</a>
          </div>
        </div>
      </div>
    </div>
    <style>
      @keyframes rainbow { 0%{background-position:0% 50%} 50%{background-position:100% 50%} 100%{background-position:0% 50%} }
    </style>`;

    capabilitiesReady().then(function () {
        applyFeatureVisibility(tb);
    });

    // 高度不足时手动收起整块顶栏
    var shell = tb.closest(".sticky") || tb.parentElement;
    var collapseBtn = document.getElementById("header-collapse-btn");
    if (shell && collapseBtn) {
        shell.classList.add("header-shell");
        var restore = document.createElement("button");
        restore.type = "button";
        restore.id = "header-restore";
        restore.setAttribute("aria-label", "展开顶栏");
        restore.setAttribute("title", "展开顶栏");
        restore.innerHTML = "<span></span>";
        shell.insertBefore(restore, shell.firstChild);

        var collapsed = false;
        function setCollapsed(v) {
            collapsed = v;
            document.body.classList.toggle("header-collapsed", v);
        }
        collapseBtn.addEventListener("click", function () {
            setCollapsed(true);
        });
        restore.addEventListener("click", function () {
            setCollapsed(false);
        });
        document.addEventListener("keydown", function (e) {
            if (e.key === "Escape" && collapsed) setCollapsed(false);
        });
        window.addEventListener("resize", function () {
            if (collapsed && window.innerHeight >= 400) setCollapsed(false);
        });
    }

    // 窄屏导航菜单
    var menuBtn = document.getElementById("nav-menu-btn");
    var menu = document.getElementById("nav-menu");
    var menuChevron = document.getElementById("nav-menu-chevron");
    var menuOpen = false;
    var menuTimer = 0;
    function setMenuOpen(open) {
        if (!menu || !menuBtn) return;
        menuOpen = open;
        clearTimeout(menuTimer);
        menuBtn.setAttribute("aria-expanded", open ? "true" : "false");
        if (menuChevron) {
            menuChevron.style.transform = open ? "rotate(180deg)" : "";
        }
        if (open) {
            menu.classList.remove("hidden", "nav-menu-closing");
            // 重启动画，连续开关也能重新播放
            void menu.offsetWidth;
            menu.classList.add("nav-menu-open");
            window.dispatchEvent(
                new CustomEvent("seshat-menu", { detail: "topbar" })
            );
            return;
        }
        if (menu.classList.contains("hidden")) return;
        menu.classList.remove("nav-menu-open");
        menu.classList.add("nav-menu-closing");
        menuTimer = setTimeout(function () {
            menu.classList.add("hidden");
            menu.classList.remove("nav-menu-closing");
        }, 120);
    }
    if (menuBtn && menu) {
        menuBtn.addEventListener("click", function (e) {
            e.stopPropagation();
            setMenuOpen(!menuOpen);
        });
        document.addEventListener("click", function (e) {
            if (!menuOpen) return;
            if (tb.contains(e.target)) return;
            setMenuOpen(false);
        });
        document.addEventListener("keydown", function (e) {
            if (e.key === "Escape") setMenuOpen(false);
        });
        window.addEventListener("resize", function () {
            if (window.innerWidth >= 800) setMenuOpen(false);
        });
        window.addEventListener("seshat-menu", function (e) {
            if (e.detail !== "topbar") setMenuOpen(false);
        });
    }

    // 页面刷新后重新连接进行中的任务
    apiFor("task", "/v0/tasks").then(function (d) {
        if (d && d.data && d.data.length) {
            for (var i = 0; i < d.data.length; i++) {
                startProgress(d.data[i].id, d.data[i].label);
            }
        }
    });

    // 加载用户头像
    if (window.USERNAME) {
        api("/v0/users/" + window.USERNAME).then(function (u) {
            if (u && u.id) {
                var av = document.getElementById("user-avatar");
                av.onerror = function () {
                    av.style.display = "none";
                    document.getElementById("user-placeholder").style.display =
                        "";
                };
                av.src =
                    API + "/v0/users/" + window.USERNAME + "/avatar?type=large";
                av.style.display = "";
                document.getElementById("user-placeholder").style.display =
                    "none";
            }
        });
    }

    // 全局事件总线（SSE）
    initEventBus();
})();
