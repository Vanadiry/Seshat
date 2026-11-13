<script>
    import { tagState } from "./state.js";

    let state = $derived($tagState);
</script>

{#if state.tag}
    <div class="max-w-[1200px] mx-auto px-6 py-2 flex items-center gap-3">
        <a
            href="/tags"
            onclick={(e) => window.backNav(e)}
            class="w-9 h-9 md:w-7 md:h-7 rounded-full border border-bord-strong flex items-center justify-center text-sub hover:text-text hover:border-bord-xstrong shrink-0 no-underline text-xs"
            title="返回">←</a
        >
        <span class="text-xl font-bold truncate">标签：{state.tag}</span>
    </div>
{/if}
