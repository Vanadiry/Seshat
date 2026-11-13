<script>
    import { personState } from "./state.js";

    let state = $derived($personState);
    let infoOpen = $state(false);

    function sub() {
        return window.subName(state.person.name, state.cn);
    }

    function toggleInfo(e) {
        e.stopPropagation();
        infoOpen = !infoOpen;
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

{#if state.person}
    <div
        class="max-w-[1200px] mx-auto px-4 md:px-6 h-12 py-0 flex items-center gap-3 relative"
    >
        <a
            href="/persons"
            onclick={(e) => window.backNav(e)}
            class="w-7 h-7 rounded-full border border-bord-strong flex items-center justify-center text-sub hover:text-text hover:border-bord-xstrong shrink-0 no-underline text-xs"
            title="返回">←</a
        >
        {#if window.isRemote("/v0/persons/" + state.id)}
            {@html window.globeIcon("w-6 h-6")}
        {/if}
        {#if state.person.nsfw}
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
                >{window.primaryName(state.person.name, state.cn)}</span
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

        <span class="hidden md:inline select-text text-xl font-bold truncate"
            >{window.primaryName(state.person.name, state.cn)}</span
        >
        {#if sub()}
            <span class="select-text text-sm text-sub truncate hidden md:inline"
                >{sub()}</span
            >
        {/if}
        <span
            class="hidden md:inline select-text text-xs text-sub ml-auto shrink-0"
            >ID: {state.person.id}</span
        >

        {#if infoOpen}
            <div
                id="detail-info"
                class="md:hidden absolute left-0 right-0 top-full z-50 px-4 pt-1"
            >
                <div
                    class="bg-surface-raised border border-bord rounded-xl shadow-lg px-3 py-2 text-xs"
                >
                    {#if sub()}
                        <div class="flex gap-2">
                            <span class="text-sub shrink-0"
                                >{window.PREFER_LANG === "chinese"
                                    ? "原名"
                                    : "中文名"}</span
                            >
                            <span class="select-text break-all">{sub()}</span>
                        </div>
                    {/if}
                    <div class="flex gap-2 mt-1">
                        <span class="text-sub shrink-0">ID</span>
                        <span class="select-text">{state.person.id}</span>
                    </div>
                </div>
            </div>
        {/if}
    </div>
{/if}
