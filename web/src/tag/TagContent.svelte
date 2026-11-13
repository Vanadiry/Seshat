<script>
    import { tagState } from "./state.js";
    import TagGrid from "./TagGrid.svelte";

    let state = $derived($tagState);
</script>

{#if state.loading}
    <div class="text-center py-20 text-sub"><span class="spin"></span></div>
{:else if state.missing}
    <div class="grid grid-cols-[repeat(auto-fill,minmax(175px,1fr))] gap-4">
        <div class="col-span-full text-center py-20 text-sub">缺少标签参数</div>
    </div>
{:else if state.error}
    <div class="grid grid-cols-[repeat(auto-fill,minmax(175px,1fr))] gap-4">
        <div class="col-span-full text-center py-20 text-sub">
            渲染出错：{state.error}
        </div>
    </div>
{:else if state.tag}
    <TagGrid items={state.items} />
{/if}
