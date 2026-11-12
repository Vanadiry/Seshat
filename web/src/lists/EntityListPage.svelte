<script>
    import { imgFallback } from "../lib/img.js";

    let { kind = "character", unit = "个角色" } = $props();

    let items = $state([]);
    let loading = $state(true);
    let error = $state("");
    let showing = $state(100);

    let visible = $derived(items.slice(0, showing));
    let path = $derived(kind === "person" ? "/person?id=" : "/character?id=");

    async function load() {
        try {
            const res = await window.apiFor("lists", "/v0/" + kind + "s/list");
            items = res && Array.isArray(res.data) ? res.data : [];
        } catch (e) {
            error = e.message || "加载失败";
        }
        loading = false;
    }

    load();
</script>

{#if loading}
    <div class="text-center py-20 text-sub"><span class="spin"></span></div>
{:else if error}
    <div class="text-center py-20 text-sub">渲染出错: {error}</div>
{:else if !items.length}
    <div class="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-2">
        <div class="col-span-full text-center py-20 text-sub">暂无数据</div>
    </div>
{:else}
    <div
        id="grid"
        class="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-2"
    >
        {#each visible as c}
            <a
                href={path + c.id}
                class="fade-in flex items-center gap-3 bg-surface rounded-lg overflow-hidden cursor-pointer border border-bord hover:border-pink hover:-translate-y-0.5 transition-all h-[72px] no-underline"
            >
                <img
                    alt=""
                    src={window.img(kind, c.id, "grid")}
                    loading="lazy"
                    use:imgFallback={{
                        kind: kind,
                        id: c.id,
                        size: "grid"
                    }}
                    class="w-[72px] h-[72px] object-cover bg-surface shrink-0"
                />
                <div class="flex-1 min-w-0 pr-3 py-1">
                    <div class="text-sm font-semibold truncate">
                        {window.primaryName(c.name, c.name_cn)}
                    </div>
                    {#if window.subName(c.name, c.name_cn)}
                        <div class="text-xs text-sub truncate mt-0.5">
                            {window.subName(c.name, c.name_cn)}
                        </div>
                    {/if}
                </div>
            </a>
        {/each}
    </div>
    <div id="pager" class="mt-6 text-center">
        {#if items.length <= 100}
            <span class="text-xs text-sub">共 {items.length} {unit}</span>
        {:else}
            <span class="text-xs text-sub"
                >当前展示 {visible.length}，共 {items.length} {unit}</span
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
{/if}
