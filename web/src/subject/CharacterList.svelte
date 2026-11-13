<script>
    import { imgFallback } from "../lib/img.js";

    let { chars = [] } = $props();

    let filter = $state("all");

    let relCounts = $derived.by(() => {
        var counts = {};
        for (var i = 0; i < chars.length; i++) {
            var rel = chars[i].relation || "其他";
            counts[rel] = (counts[rel] || 0) + 1;
        }
        return counts;
    });

    let relKeys = $derived(Object.keys(relCounts));

    let visible = $derived(
        filter === "all"
            ? chars
            : chars.filter((c) => (c.relation || "其他") === filter)
    );

    function pairs(list) {
        var out = [];
        for (var i = 0; i < list.length; i += 2) {
            out.push([list[i], list[i + 1] || null]);
        }
        return out;
    }

    function charInfo(c) {
        var out = [];
        if (c.gender) out.push("性别:" + c.gender);
        if (c.infobox) {
            try {
                var info = JSON.parse(c.infobox);
                for (var i = 0; i < info.length; i++) {
                    if (info[i].key === "生日" || info[i].key === "身高") {
                        out.push(info[i].value);
                    }
                }
            } catch (_) {}
        }
        return out;
    }

    function selectFilter(rel) {
        filter = rel;
    }
</script>

{#snippet card(c)}
    <div
        tabindex="0"
        role="link"
        data-href={"/character?id=" + c.id}
        onkeydown={(e) => window.cardKeyNav(e)}
        onclick={() => (location.href = "/character?id=" + c.id)}
        class="border-t border-bord-light cursor-pointer"
        style="height:128px"
    >
        <div class="flex items-center gap-3 h-full">
            <img
                alt=""
                src={window.img("character", c.id, "small")}
                use:imgFallback={{ kind: "character", id: c.id, size: "small" }}
                class="shrink-0 rounded object-cover object-top bg-surface cursor-pointer"
                style="width:80px;aspect-ratio:1/1.414"
            />
            <div class="flex-1 min-w-0 py-2 ml-1">
                <div
                    class="font-semibold text-[14px] cursor-pointer hover:text-pink inline-block truncate"
                >
                    {c.name}
                </div>
                <div
                    class="flex flex-wrap items-center gap-x-2 gap-y-0.5 mt-0.5"
                >
                    <span
                        class="bg-pink-solid text-white px-1.5 py-px rounded text-[11px] font-semibold"
                        >{c.relation}</span
                    >
                    {#each charInfo(c) as text}
                        <span class="text-[12px] text-sub">{text}</span>
                    {/each}
                </div>
                {#if c.actors && c.actors.length}
                    <div
                        class="flex gap-3 overflow-x-auto mt-2 thin-scroll"
                        style="max-width:100%"
                    >
                        {#each c.actors as a}
                            <div
                                tabindex="0"
                                role="link"
                                data-href={"/person?id=" + a.id}
                                onkeydown={(e) => window.cardKeyNav(e)}
                                class="flex items-center gap-2 cursor-pointer hover:opacity-80 shrink-0"
                                onclick={(e) => {
                                    e.stopPropagation();
                                    location.href = "/person?id=" + a.id;
                                }}
                            >
                                <img
                                    alt=""
                                    src={window.img("person", a.id, "grid")}
                                    use:imgFallback={{
                                        kind: "person",
                                        id: a.id,
                                        size: "grid"
                                    }}
                                    class="w-8 h-8 rounded-lg object-cover bg-surface shrink-0"
                                />
                                <div class="shrink-0">
                                    <div
                                        class="text-[13px] font-semibold hover:text-pink"
                                    >
                                        {a.name}
                                    </div>
                                    <div class="text-[11px] text-sub">CV</div>
                                </div>
                            </div>
                        {/each}
                    </div>
                {/if}
            </div>
        </div>
    </div>
{/snippet}

<div class="select-text fade-in">
    {#if !chars.length}
        <div class="text-center py-10 text-sub">暂无角色</div>
    {:else}
        <div class="flex flex-wrap gap-1.5 mb-3">
            <span
                tabindex="0"
                role="button"
                onkeydown={(e) => window.clickKeyNav(e)}
                onclick={() => selectFilter("all")}
                class="px-3 py-2.5 md:py-1 rounded-full text-xs cursor-pointer {filter ===
                'all'
                    ? 'bg-pink-solid text-white'
                    : 'bg-surface-alt text-sub hover:text-text'}"
                >全部 ({chars.length})</span
            >
            {#each relKeys as rel}
                <span
                    tabindex="0"
                    role="button"
                    onkeydown={(e) => window.clickKeyNav(e)}
                    onclick={() => selectFilter(rel)}
                    class="px-3 py-2.5 md:py-1 rounded-full text-xs cursor-pointer {filter ===
                    rel
                        ? 'bg-pink-solid text-white'
                        : 'bg-surface-alt text-sub hover:text-text'}"
                    >{rel} ({relCounts[rel]})</span
                >
            {/each}
        </div>
        <div id="char-list">
            {#if !visible.length}
                <div class="text-center py-10 text-sub">没有符合条件的角色</div>
            {/if}
            {#each pairs(visible) as pair}
                <div class="flex flex-col md:flex-row md:gap-4">
                    <div class="flex-1 min-w-0">{@render card(pair[0])}</div>
                    <div class="flex-1 min-w-0">
                        {#if pair[1]}{@render card(pair[1])}{/if}
                    </div>
                </div>
            {/each}
        </div>
    {/if}
</div>
