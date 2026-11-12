<script>
    import SectionTitle from "../subject/SectionTitle.svelte";
    import { imgFallback } from "../lib/img.js";

    let { items = [], id = "" } = $props();

    let showing = $state(10);

    let visible = $derived(items.slice(0, showing));

    let remoteExtra = $derived(
        window.isRemote("/v0/characters/" + id + "/subjects") ||
            window.isRemote("/v0/characters/" + id + "/persons")
            ? window.globeIcon("w-4 h-4")
            : ""
    );

    const typeCls = {
        1: "bg-tier-gold-bg text-tier-gold",
        2: "bg-tier-blue-bg text-tier-blue",
        2: "bg-tier-blue-bg text-tier-blue",
        3: "bg-tier-purple-bg text-tier-purple",
        4: "bg-tier-green-bg text-tier-green",
        6: "bg-surface-alt text-sub"
    };

    function pairs(list) {
        var out = [];
        for (var i = 0; i < list.length; i += 2) {
            out.push([list[i], list[i + 1] || null]);
        }
        return out;
    }
</script>

{#snippet card(item)}
    <div
        tabindex="0"
        role="link"
        data-href={"/subject?id=" + item.id}
        onkeydown={(e) => window.cardKeyNav(e)}
        onclick={() => (location.href = "/subject?id=" + item.id)}
        class="border-t border-bord-light overflow-hidden cursor-pointer"
        style="height:80px"
    >
        <div class="flex items-center gap-3 h-full">
            <div
                class="relative shrink-0 rounded overflow-hidden bg-surface cursor-pointer"
                style="width:64px;aspect-ratio:1"
            >
                <img
                    alt=""
                    src={window.img("subject", item.id, "grid")}
                    use:imgFallback={{
                        kind: "subject",
                        id: item.id,
                        size: "grid"
                    }}
                    class="w-full h-full object-cover"
                />
                {#if window.MSG.subjectTypes[item.type]}
                    <span
                        class="absolute top-0.5 right-0.5 text-[9px] font-semibold {typeCls[
                            item.type
                        ]} px-1 py-px rounded"
                        >{window.MSG.subjectTypes[item.type]}</span
                    >
                {/if}
            </div>
            <div class="flex-1 min-w-0">
                <div
                    class="font-semibold text-[14px] cursor-pointer hover:text-pink truncate"
                >
                    {window.primaryName(item.name, item.name_cn)}
                </div>
                {#if window.subName(item.name, item.name_cn)}
                    <div class="text-xs text-sub truncate mt-0.5">
                        {window.subName(item.name, item.name_cn)}
                    </div>
                {/if}
                <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 mt-1">
                    <span
                        class="bg-pink-solid text-white px-1.5 py-px rounded text-[11px] font-semibold"
                        >{item.staff || ""}</span
                    >
                </div>
            </div>
            {#if item.person_id}
                <div
                    tabindex="0"
                    role="link"
                    data-href={"/person?id=" + item.person_id}
                    onkeydown={(e) => window.cardKeyNav(e)}
                    class="flex items-start gap-2 shrink-0 cursor-pointer self-start mt-2"
                    onclick={(e) => {
                        e.stopPropagation();
                        location.href = "/person?id=" + item.person_id;
                    }}
                >
                    <span class="text-[14px] font-semibold hover:text-pink"
                        >{item.person_name}</span
                    >
                    <img
                        alt=""
                        src={window.img("person", item.person_id, "grid")}
                        use:imgFallback={{
                            kind: "person",
                            id: item.person_id,
                            size: "grid"
                        }}
                        class="w-10 h-10 rounded-lg object-cover bg-surface"
                    />
                </div>
            {/if}
        </div>
    </div>
{/snippet}

{#if items.length}
    <div>
        <SectionTitle text="出演" extra={remoteExtra} />
        {#each pairs(visible) as pair}
            <div class="flex gap-4">
                <div class="flex-1 min-w-0">
                    {@render card(pair[0])}
                </div>
                <div class="flex-1 min-w-0">
                    {#if pair[1]}{@render card(pair[1])}{/if}
                </div>
            </div>
        {/each}
        {#if items.length > 10}
            <div class="mt-3 text-center">
                <span class="text-xs text-sub">共 {items.length} 个条目</span>
                {#if visible.length < items.length}
                    <button
                        type="button"
                        onclick={() => (showing += 10)}
                        class="ml-3 px-3 py-1 rounded-lg border border-bord bg-transparent text-sub text-xs cursor-pointer hover:bg-active hover:text-text"
                        >展开更多</button
                    >
                    {#if items.length - visible.length > 10}
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
    </div>
{/if}
