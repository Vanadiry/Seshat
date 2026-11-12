<script>
    import { imgFallback } from "../lib/img.js";

    let { items = [] } = $props();

    let showing = $state(100);
    let visible = $derived(items.slice(0, showing));
</script>

<div
    id="grid"
    class="grid grid-cols-[repeat(auto-fill,minmax(175px,1fr))] gap-4"
>
    {#if !items.length}
        <div class="col-span-full text-center py-20 text-sub">暂无数据</div>
    {:else}
        {#each visible as s}
            <a
                href={"/subject?id=" + s.id}
                class="fade-in block no-underline bg-surface rounded-xl overflow-hidden cursor-pointer border border-bord hover:border-pink hover:-translate-y-0.5 hover:shadow-[0_4px_20px_rgba(254,138,149,.1)] transition-all"
            >
                <img
                    alt=""
                    src={window.img("subject", s.id, "small")}
                    loading="lazy"
                    use:imgFallback={{
                        kind: "subject",
                        id: s.id,
                        size: "small"
                    }}
                    class="w-full object-cover bg-surface"
                    style="aspect-ratio:1/1.414"
                />
                <div class="p-2.5">
                    <div class="text-sm font-semibold truncate">
                        {window.primaryName(s.name, s.name_cn)}
                    </div>
                    <div
                        class="text-xs text-sub mt-1 flex gap-1.5 items-center"
                    >
                        {#if s.rating && s.rating.score}
                            <span
                                class="bg-pink-solid text-white px-1.5 py-px rounded-md text-[12px] font-bold"
                                >★ {s.rating.score.toFixed(1)}</span
                            >
                        {/if}
                        {#if s.platform}
                            <span
                                class="bg-pink-bg text-pink px-1.5 py-px rounded-md text-[12px]"
                                >{s.platform}</span
                            >
                        {/if}
                    </div>
                </div>
            </a>
        {/each}
    {/if}
</div>

<div id="pager" class="mt-6 text-center">
    {#if items.length <= 100}
        <span class="text-xs text-sub">共 {items.length} 个动画</span>
    {:else}
        <span class="text-xs text-sub"
            >当前展示 {visible.length}，共 {items.length} 个动画</span
        >
        {#if visible.length < items.length}
            <button
                type="button"
                onclick={() => (showing += 100)}
                class="ml-3 px-3 py-1 rounded-lg border border-bord bg-transparent text-sub text-xs cursor-pointer hover:bg-active hover:text-text"
                >展开更多</button
            >
            <button
                type="button"
                onclick={() => (showing = items.length)}
                class="ml-2 px-3 py-1 rounded-lg border border-bord bg-transparent text-sub text-xs cursor-pointer hover:bg-active hover:text-text"
                >展开全部</button
            >
        {/if}
    {/if}
</div>
