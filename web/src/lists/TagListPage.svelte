<script>
    let items = $state([]);
    let loading = $state(true);
    let blocked = $state(false);
    let error = $state("");
    let showing = $state(200);

    let visible = $derived(items.slice(0, showing));

    async function load() {
        try {
            if (!(await window.requireFeature("tags"))) {
                blocked = true;
                loading = false;
                return;
            }
            const res = await window.apiFor("tags", "/v0/tags");
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
{:else if blocked}
    <div></div>
{:else if error}
    <div class="text-center py-20 text-sub">渲染出错: {error}</div>
{:else if !items.length}
    <span class="text-sub text-sm">暂无标签</span>
{:else}
    <div id="tags" class="flex flex-wrap gap-2">
        {#each visible as t}
            <a
                href={"/tag?tag=" + encodeURIComponent(t.name)}
                class="no-underline bg-surface border border-bord rounded-lg px-3 py-1.5 text-sm hover:border-pink hover:-translate-y-0.5 transition-all"
                ><span class="text-text">{t.name}</span><small
                    class="text-sub ml-1.5">{t.count}</small
                ></a
            >
        {/each}
    </div>
    <div id="pager" class="mt-6 text-center">
        {#if items.length <= 200}
            <span class="text-xs text-sub">共 {items.length} 个标签</span>
        {:else}
            <span class="text-xs text-sub"
                >当前展示 {visible.length}，共 {items.length} 个标签</span
            >
            {#if visible.length < items.length}
                <button
                    type="button"
                    onclick={() => (showing += 200)}
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
