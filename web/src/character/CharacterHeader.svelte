<script>
    import { characterState } from "./state.js";

    let state = $derived($characterState);

    function sub() {
        return window.subName(state.character.name, state.cn);
    }
</script>

{#if state.character}
    <div class="max-w-[1200px] mx-auto px-6 h-12 py-0 flex items-center gap-3">
        <a
            href="/characters"
            onclick={(e) => window.backNav(e)}
            class="w-7 h-7 rounded-full border border-bord-strong flex items-center justify-center text-sub hover:text-text hover:border-bord-xstrong shrink-0 no-underline text-xs"
            title="返回">←</a
        >
        {#if window.isRemote("/v0/characters/" + state.id)}
            {@html window.globeIcon("w-6 h-6")}
        {/if}
        {#if state.character.nsfw}
            <span
                class="text-[10px] font-semibold px-1.5 py-0.5 rounded bg-warn-bg text-warn shrink-0"
                >NSFW</span
            >
        {/if}
        <span class="select-text text-xl font-bold truncate"
            >{window.primaryName(state.character.name, state.cn)}</span
        >
        {#if sub()}
            <span class="select-text text-sm text-sub truncate hidden md:inline"
                >{sub()}</span
            >
        {/if}
        <span class="select-text text-xs text-sub ml-auto shrink-0"
            >ID: {state.character.id}</span
        >
    </div>
{/if}
