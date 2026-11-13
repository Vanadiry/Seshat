<script>
    import { subjectState, switchTab, exportBundle } from "./state.js";

    const tabs = [
        ["overview", "概览"],
        ["ep", "剧集"],
        ["chars", "角色"],
        ["persons", "制作人员"]
    ];

    // tab 和导出按钮共用同一套盒子样式，保证高度一致
    const TAB_CLS =
        "h-7 px-3 text-sm inline-flex items-center whitespace-nowrap";

    let infoOpen = $state(false);

    function goTab(key, e) {
        e.preventDefault();
        infoOpen = false;
        switchTab(key);
    }

    function toggleInfo(e) {
        e.stopPropagation();
        infoOpen = !infoOpen;
    }

    async function onExport() {
        await exportBundle();
    }

    $effect(() => {
        if (!infoOpen) return;
        var close = function (e) {
            var t = e.target;
            if (t instanceof Element && t.closest("#detail-name-btn")) return;
            if (t instanceof Element && t.closest("#detail-info")) return;
            infoOpen = false;
        };
        var onResize = function () {
            if (window.innerWidth >= 800) infoOpen = false;
        };
        var onKey = function (e) {
            if (e.key === "Escape") infoOpen = false;
        };
        document.addEventListener("click", close);
        window.addEventListener("resize", onResize);
        document.addEventListener("keydown", onKey);
        return function () {
            document.removeEventListener("click", close);
            window.removeEventListener("resize", onResize);
            document.removeEventListener("keydown", onKey);
        };
    });
</script>

{#snippet exportButton(cls)}
    <button
        type="button"
        onclick={onExport}
        disabled={$subjectState.exporting}
        class="{TAB_CLS} shrink-0 border-0 bg-transparent text-sub hover:text-text cursor-pointer disabled:opacity-50 disabled:cursor-default {cls}"
        title="导出数据包">导出</button
    >
{/snippet}

{#snippet tabItems(withId)}
    {#each tabs as [key, label]}
        {@const count = $subjectState.counts[key] || 0}
        <a
            href={"?id=" + $subjectState.sid + "&tab=" + key}
            id={withId ? "tab-" + key : undefined}
            onclick={(e) => goTab(key, e)}
            class="{TAB_CLS} {$subjectState.tab === key
                ? 'text-text border-b-2 border-pink'
                : 'text-sub hover:text-text'} no-underline"
            >{label}{#if count && $subjectState.tab === key}
                ({count}){/if}</a
        >
    {/each}
{/snippet}

{#if $subjectState.subject}
    {@const subject = $subjectState.subject}
    {@const sub = window.subName(subject.name, subject.name_cn)}
    <div class="max-w-[1200px] mx-auto px-4 md:px-6 relative">
        <div class="h-12 py-0 flex items-center gap-2 md:gap-3 overflow-x-auto">
            <a
                href="/"
                onclick={(e) => window.backNav(e)}
                class="w-7 h-7 rounded-full border border-bord-strong flex items-center justify-center text-sub hover:text-text hover:border-bord-xstrong shrink-0 no-underline text-xs"
                title="返回">←</a
            >
            {#if window.isRemote("/v0/subjects/" + $subjectState.sid)}
                {@html window.globeIcon("w-6 h-6")}
            {/if}
            {#if subject.nsfw}
                <span
                    class="text-[10px] font-semibold px-1.5 py-0.5 rounded bg-warn-bg text-warn shrink-0"
                    >NSFW</span
                >
            {/if}

            <!-- 窄屏：名称按钮，点击显示次要名称和 ID -->
            <button
                id="detail-name-btn"
                type="button"
                onclick={toggleInfo}
                class="md:hidden flex items-center gap-1 min-w-0 max-w-full px-0 py-0 border-0 bg-transparent text-text cursor-pointer"
                aria-expanded={infoOpen}
                aria-controls="detail-info"
            >
                <span class="truncate text-xl font-bold"
                    >{window.primaryName(subject.name, subject.name_cn)}</span
                >
                <svg
                    viewBox="0 0 24 24"
                    class="w-3.5 h-3.5 shrink-0 text-sub transition-transform"
                    style="transform:{infoOpen ? 'rotate(180deg)' : ''}"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    aria-hidden="true"><path d="m6 9 6 6 6-6"></path></svg
                >
            </button>

            {#if subject.platform}
                <span class="md:hidden text-xs text-sub shrink-0"
                    >{subject.platform}</span
                >
            {/if}

            <!-- 桌面：名称、次要名称、平台和 ID 保持一行 -->
            <span
                class="hidden md:inline select-text text-xl font-bold truncate"
                >{window.primaryName(subject.name, subject.name_cn)}</span
            >
            {#if sub}
                <span
                    class="select-text text-sm text-sub truncate hidden md:inline"
                    >{sub}</span
                >
            {/if}
            {#if subject.platform}
                <span class="text-sub text-xs shrink-0 hidden md:inline"
                    >｜</span
                ><span class="text-xs text-sub shrink-0 hidden md:inline"
                    >{subject.platform}</span
                >
            {/if}
            <span class="text-sub text-xs shrink-0 hidden md:inline">｜</span
            ><span
                class="select-text text-xs text-sub shrink-0 hidden md:inline"
                >ID: {$subjectState.sid}</span
            >
            <span class="flex-1"></span>
            <span class="hidden md:flex gap-0 shrink-0 ml-2"
                >{@render tabItems(
                    true
                )}{#if $subjectState.showBundle}{@render exportButton(
                        ""
                    )}{/if}</span
            >
        </div>

        <!-- 窄屏：导出按钮和 tab -->
        <div
            class="md:hidden h-10 flex items-center gap-2 border-t border-bord-light"
        >
            <span class="flex-1 flex gap-0 min-w-0 overflow-x-auto"
                >{@render tabItems(
                    false
                )}{#if $subjectState.showBundle}{@render exportButton(
                        ""
                    )}{/if}</span
            >
        </div>

        <!-- 名称浮层 -->
        {#if infoOpen}
            <div
                id="detail-info"
                class="md:hidden absolute left-0 right-0 top-full z-50 px-4 pt-1"
            >
                <div
                    class="bg-surface-raised border border-bord rounded-xl shadow-lg px-3 py-2 text-xs"
                >
                    {#if sub}
                        <div class="flex gap-2">
                            <span class="text-sub shrink-0"
                                >{window.PREFER_LANG === "chinese"
                                    ? "原名"
                                    : "中文名"}</span
                            >
                            <span class="select-text break-all">{sub}</span>
                        </div>
                    {/if}
                    <div class="flex gap-2 mt-1">
                        <span class="text-sub shrink-0">ID</span>
                        <span class="select-text">{$subjectState.sid}</span>
                    </div>
                </div>
            </div>
        {/if}
    </div>
{/if}
