<script>
    import { headerState } from "./store.js";

    const tabs = [
        ["overview", "概览"],
        ["ep", "剧集"],
        ["chars", "角色"],
        ["persons", "制作人员"]
    ];

    function goTab(key, e) {
        e.preventDefault();
        window.switchTab(key);
    }
</script>

{#if $headerState.subject}
    {@const subject = $headerState.subject}
    {@const sub = window.subName(subject.name, subject.name_cn)}
    <div
        class="max-w-[1200px] mx-auto px-6 h-12 py-0 flex items-center gap-3 overflow-x-auto"
    >
        <a
            href="/"
            onclick={(e) => window.backNav(e)}
            class="w-7 h-7 rounded-full border border-bord-strong flex items-center justify-center text-sub hover:text-text hover:border-bord-xstrong shrink-0 no-underline text-xs"
            title="返回">←</a
        >
        {#if window.isRemote("/v0/subjects/" + $headerState.sid)}
            {@html window.globeIcon("w-6 h-6")}
        {/if}
        {#if subject.nsfw}
            <span
                class="text-[10px] font-semibold px-1.5 py-0.5 rounded bg-warn-bg text-warn shrink-0"
                >NSFW</span
            >
        {/if}
        <span class="select-text text-xl font-bold truncate"
            >{window.primaryName(subject.name, subject.name_cn)}</span
        >
        {#if sub}
            <span class="select-text text-sm text-sub truncate hidden sm:inline"
                >{sub}</span
            >
        {/if}
        {#if subject.platform}
            <span class="text-sub text-xs shrink-0">｜</span><span
                class="text-xs text-sub shrink-0">{subject.platform}</span
            >
        {/if}
        <span class="text-sub text-xs shrink-0">｜</span><span
            class="select-text text-xs text-sub shrink-0"
            >ID: {$headerState.sid}</span
        >
        <span class="flex-1"></span>
        {#if $headerState.showBundle}
            <button
                type="button"
                onclick={() => window.exportSubjectBundle()}
                class="w-7 h-7 rounded-full border border-bord-strong flex items-center justify-center text-sub hover:text-text hover:border-bord-xstrong shrink-0 cursor-pointer bg-transparent"
                title="导出数据包"
                aria-label="导出数据包"
            >
                <svg
                    viewBox="0 0 24 24"
                    class="w-4 h-4"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    aria-hidden="true"
                    ><path d="M12 3v12"></path><path d="m7 10 5 5 5-5"
                    ></path><path d="M5 21h14"></path></svg
                >
            </button>
        {/if}
        <span class="flex gap-0 shrink-0 ml-2">
            {#each tabs as [key, label]}
                {@const count = $headerState.counts[key] || 0}
                <a
                    href={"?id=" + $headerState.sid + "&tab=" + key}
                    id={"tab-" + key}
                    onclick={(e) => goTab(key, e)}
                    class="{$headerState.tab === key
                        ? 'text-text border-b-2 border-pink'
                        : 'text-sub hover:text-text'} no-underline px-3 py-1 text-sm inline-block whitespace-nowrap"
                    >{label}{#if count && $headerState.tab === key}
                        ({count}){/if}</a
                >
            {/each}
        </span>
    </div>
{/if}
