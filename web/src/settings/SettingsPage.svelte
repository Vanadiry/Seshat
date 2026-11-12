<script>
    import { tick } from "svelte";
    import SectionTitle from "../subject/SectionTitle.svelte";

    const PREFS = [
        {
            key: "theme",
            label: "界面主题",
            choices: [
                { k: "auto", l: "同步系统" },
                { k: "dark", l: "深色" },
                { k: "light", l: "浅色" }
            ],
            section: "外观"
        },
        {
            key: "prefer_lang",
            label: "标题展示方式",
            choices: [
                { k: "original", l: "原文优先" },
                { k: "chinese", l: "中文优先" }
            ],
            section: "外观"
        },
        {
            key: "subject_sort",
            label: "首页排序方式",
            choices: [
                { k: "elo", l: "ELO 评分降序" },
                { k: "my", l: "我的评分降序" },
                { k: "bgm_rank", l: "番组计划评分降序" },
                { k: "watching", l: "在看优先" },
                { k: "random", l: "随机" },
                { k: "none", l: "不排序" }
            ],
            section: "数据与浏览"
        },
        {
            key: "auto_link_names",
            label: "条目名称高亮",
            choices: [
                { k: "true", l: "开启" },
                { k: "false", l: "关闭" }
            ],
            section: "数据与浏览"
        },
        { key: "username", label: "Bangumi 用户 ID", section: "账号" }
    ];

    const CONFIG_SECTIONS = [
        {
            title: "本地服务器",
            keys: [
                "bind_addr",
                "port",
                "concurrency_info",
                "concurrency_image",
                "data_home",
                "log_level"
            ],
            labels: {
                bind_addr: "监听地址",
                port: "端口",
                concurrency_info: "信息拉取并发",
                concurrency_image: "图片下载并发",
                data_home: "数据目录",
                log_level: "日志级别"
            }
        },
        {
            title: "上游",
            keys: ["base_url", "user_agent"],
            labels: { base_url: "上游地址", user_agent: "User Agent" }
        },
        {
            title: "代理",
            keys: ["proxy_host", "proxy_port"],
            labels: { proxy_host: "代理地址", proxy_port: "代理端口" }
        },
        {
            title: "前端",
            keys: ["backend_url", "fallback_url"],
            labels: {
                backend_url: "后端地址",
                fallback_url: "回退地址"
            }
        },
        {
            title: "身份认证",
            keys: ["bangumi_access_token"],
            labels: { bangumi_access_token: "Bangumi 访问令牌" }
        }
    ];

    let prefs = $state({});
    let config = $state({});
    let loaded = $state(false);
    let about = $state(null);
    let capsText = $state("");
    let saveMsg = $state("");
    let saving = $state(false);

    let sections = $derived.by(() => {
        const map = {};
        for (const d of PREFS) {
            if (!map[d.section]) map[d.section] = [];
            map[d.section].push(d);
        }
        return Object.entries(map);
    });

    function setPref(key, value) {
        prefs = { ...prefs, [key]: value };
    }

    function scrollToHash() {
        if (!location.hash) return;
        const el = document.getElementById(location.hash.slice(1));
        if (el) el.scrollIntoView();
    }

    function computeCaps() {
        if (!window.BACKEND_URL) {
            return window.capLabels(window.CAPS_ALL).join("、");
        }
        if (typeof window._caps === "undefined" || !window._caps) {
            return "探测中";
        }
        if (!window._caps.seshat) {
            return "未识别为 Seshat";
        }
        const names = window.capLabels(window._caps.features);
        return names.length ? names.join("、") : "未提供能力清单";
    }

    function renderAbout(info) {
        about = info;
        capsText = computeCaps();
    }

    async function loadSettings() {
        const r = await fetch("/api/v0/settings");
        const data = await r.json();
        prefs = data.prefs || {};
        config = data.config || {};
        loaded = true;
        await tick();
        scrollToHash();
    }

    function loadInfo() {
        window.getInstanceInfo().then(function (info) {
            if (info) renderAbout(info);
        });
        window.capabilitiesReady().then(function () {
            if (about) renderAbout(about);
        });
    }

    async function saveSettings() {
        const updates = {};
        for (const d of PREFS) {
            updates[d.key] = prefs[d.key] !== undefined ? prefs[d.key] : "";
        }
        saving = true;
        try {
            const r = await fetch("/api/v0/settings", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(updates)
            });
            const d = await window.readJSON(r);
            if (!r.ok) {
                saveMsg = window.errText(d, r.status);
                return;
            }
            if (d.error && !d.success) {
                saveMsg = d.error;
                return;
            }
            let text = "";
            if (d.success) text += "✓ " + d.success;
            if (d.error) text += (text ? "  " : "") + "✗ " + d.error;
            saveMsg = text || "保存成功";
            if (updates.theme) {
                localStorage.setItem("theme", updates.theme);
                const isLight =
                    updates.theme === "light" ||
                    (updates.theme === "auto" &&
                        window.matchMedia("(prefers-color-scheme: light)")
                            .matches);
                document.documentElement.classList.toggle("light", isLight);
            }
            if (!d.error) await loadSettings();
        } finally {
            saving = false;
        }
    }

    loadSettings();
    loadInfo();
</script>

{#snippet prefRow(d)}
    <div
        class="flex items-center gap-3 py-1.5 px-2 -mx-2 rounded-lg transition-colors duration-200 hover:bg-active focus-within:bg-active"
    >
        <div class="flex-1 min-w-0">
            <div class="text-sm text-text truncate" title={d.label}>
                {d.label}
            </div>
            <div class="text-[11px] text-sub truncate mt-0.5" title={d.key}>
                {d.key}
            </div>
        </div>
        {#if d.choices}
            <select
                id={"f-" + d.key}
                value={prefs[d.key] !== undefined ? prefs[d.key] : ""}
                onchange={(e) => setPref(d.key, e.currentTarget.value)}
                class="custom-select w-[140px] shrink-0 px-3 py-2 rounded-lg border border-bord bg-input text-sm"
            >
                {#each d.choices as c}
                    <option value={c.k}>{c.l}</option>
                {/each}
            </select>
        {:else}
            <input
                id={"f-" + d.key}
                value={prefs[d.key] !== undefined ? prefs[d.key] : ""}
                oninput={(e) => setPref(d.key, e.currentTarget.value)}
                class="w-[200px] shrink-0 px-3 py-2 rounded-lg border border-bord bg-input text-sm"
            />
        {/if}
    </div>
{/snippet}

<div class="fade-in">
    <div class="flex flex-col min-[720px]:flex-row gap-8">
        <div class="flex-1 min-w-0">
            <div id="settings-form">
                {#if !loaded}
                    <div class="text-center py-20 text-sub">
                        <span class="spin"></span>加载中…
                    </div>
                {:else}
                    <h2 class="text-lg font-bold text-text mb-1">前端设置</h2>
                    <p class="text-xs text-sub mb-4">
                        修改后刷新页面即可生效，无需重启后端
                    </p>
                    {#each sections as [section, defs]}
                        <SectionTitle
                            text={section}
                            outer="mt-6 mb-3 first:mt-0"
                            label="text-sm"
                        />
                        {#each defs as d}
                            {@render prefRow(d)}
                        {/each}
                    {/each}
                    <button
                        type="button"
                        onclick={saveSettings}
                        disabled={saving}
                        class="w-full mt-6 px-4 py-2.5 rounded-xl bg-pink-solid text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90 transition-all active:scale-[0.98]"
                    >
                        保存设置
                    </button>
                    <p id="save-msg" class="text-xs text-sub mt-3 text-center">
                        {saveMsg}
                    </p>
                {/if}
            </div>
        </div>

        <div class="hidden min-[720px]:block w-px bg-bord-light shrink-0"></div>

        <div
            class="flex-1 min-w-0 border-t border-bord-light pt-8 min-[720px]:border-0 min-[720px]:pt-0"
        >
            <div id="settings-sidebar" class="sticky sticky-below-topbar">
                <h2 class="text-lg font-bold text-text mb-1">后端设置</h2>
                <p class="text-xs text-sub mb-4">
                    此处内容需修改 config.toml 并重启应用
                </p>
                {#each CONFIG_SECTIONS as sec}
                    <SectionTitle
                        text={sec.title}
                        outer="mt-6 mb-3 first:mt-0"
                        label="text-sm"
                    />
                    {#each sec.keys as k}
                        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
                        <div
                            class="flex items-center gap-3 py-1.5 px-2 -mx-2 rounded-lg transition-colors duration-200 hover:bg-active focus-visible:bg-active"
                            tabindex="0"
                        >
                            <div class="flex-1 min-w-0">
                                <div
                                    class="text-sm text-text truncate"
                                    title={sec.labels[k] || k}
                                >
                                    {sec.labels[k] || k}
                                </div>
                                <div
                                    class="text-[11px] text-sub truncate mt-0.5"
                                    title={k}
                                >
                                    {k}
                                </div>
                            </div>
                            <div
                                class="text-sm text-text font-mono shrink-0 text-right max-w-[50%] truncate"
                                title={config[k] ? String(config[k]) : ""}
                            >
                                {#if config[k]}
                                    {String(config[k])}
                                {:else}
                                    <span class="italic text-sub">未设置</span>
                                {/if}
                            </div>
                        </div>
                    {/each}
                {/each}
            </div>
        </div>
    </div>

    <div id="about" class="mt-10 scroll-mt-20">
        {#if about}
            <div class="border-t border-bord pt-6 text-center">
                <div class="text-sm text-text">{about.name || "Seshat"}</div>
                {#if about.version || about.license || about.repository}
                    <div class="text-xs text-sub mt-1">
                        {#if about.version}版本：{about.version}{/if}
                        {#if about.version && (about.license || about.repository)}
                            ｜
                        {/if}
                        {#if about.license}许可证：{about.license}{/if}
                        {#if about.license && about.repository}｜{/if}
                        {#if about.repository}
                            <a
                                href={about.repository}
                                onclick={(e) => {
                                    e.preventDefault();
                                    window.openUrl(about.repository);
                                }}
                                class="text-pink hover:underline">项目储存库</a
                            >
                        {/if}
                    </div>
                {/if}
                <div class="text-xs text-sub mt-1 leading-relaxed">
                    {window.BACKEND_URL ? "远端能力：" : "能力："}{capsText}
                </div>
                {#if about.copyright}
                    <div class="text-xs text-sub mt-1">{about.copyright}</div>
                {/if}
            </div>
        {/if}
    </div>
</div>
