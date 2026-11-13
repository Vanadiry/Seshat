<script>
    import SectionTitle from "../subject/SectionTitle.svelte";
    import { imgFallback } from "../lib/img.js";

    let { items = [], title = "", id = "", kind = "subjects" } = $props();

    let showing = $state(10);

    const order = { 2: 0, 4: 1, 1: 2, 3: 3, 6: 4 };
    const typeCls = {
        1: "bg-tier-gold-bg text-tier-gold",
        2: "bg-tier-blue-bg text-tier-blue",
        3: "bg-tier-purple-bg text-tier-purple",
        4: "bg-tier-green-bg text-tier-green",
        6: "bg-surface-alt text-sub"
    };

    function typeField() {
        return kind === "chars" ? "subject_type" : "type";
    }

    function subjectId(item) {
        return kind === "chars" ? item.subject_id : item.id;
    }

    function subjectName(item) {
        return kind === "chars" ? item.subject_name : item.name;
    }

    function subjectNameCn(item) {
        return kind === "chars"
            ? item.subject_name_cn || ""
            : item.name_cn || "";
    }

    function subjectType(item) {
        return kind === "chars" ? item.subject_type : item.type;
    }

    let sorted = $derived.by(() => {
        const field = typeField();
        return items.slice().sort(function (a, b) {
            const ta = a[field] != null ? a[field] : 0;
            const tb = b[field] != null ? b[field] : 0;
            return (
                (order[ta] != null ? order[ta] : 5) -
                (order[tb] != null ? order[tb] : 5)
            );
        });
    });

    let visible = $derived(sorted.slice(0, showing));

    let remoteExtra = $derived(
        window.isRemote(
            "/v0/persons/" +
                id +
                (kind === "chars" ? "/characters" : "/subjects")
        )
            ? window.globeIcon("w-4 h-4")
            : ""
    );

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
        data-href={"/subject?id=" + subjectId(item)}
        onkeydown={(e) => window.cardKeyNav(e)}
        onclick={() => (location.href = "/subject?id=" + subjectId(item))}
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
                    src={window.img("subject", subjectId(item), "grid")}
                    use:imgFallback={{
                        kind: "subject",
                        id: subjectId(item),
                        size: "grid"
                    }}
                    class="w-full h-full object-cover"
                />
                {#if window.MSG.subjectTypes[subjectType(item)]}
                    <span
                        class="absolute top-0.5 right-0.5 text-[9px] font-semibold {typeCls[
                            subjectType(item)
                        ] || ''} px-1 py-px rounded"
                        >{window.MSG.subjectTypes[subjectType(item)]}</span
                    >
                {/if}
            </div>
            <div class="flex-1 min-w-0">
                <div
                    class="font-semibold text-[14px] cursor-pointer hover:text-pink truncate"
                >
                    {window.primaryName(subjectName(item), subjectNameCn(item))}
                </div>
                {#if window.subName(subjectName(item), subjectNameCn(item))}
                    <div class="text-xs text-sub truncate mt-0.5">
                        {window.subName(subjectName(item), subjectNameCn(item))}
                    </div>
                {/if}
                <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 mt-1">
                    <span
                        class="bg-pink-solid text-white px-1.5 py-px rounded text-[11px] font-semibold"
                        >{item.staff || ""}</span
                    >
                </div>
            </div>
            {#if kind === "chars"}
                <div
                    tabindex="0"
                    role="link"
                    data-href={"/character?id=" + item.id}
                    onkeydown={(e) => window.cardKeyNav(e)}
                    class="flex items-start gap-2 shrink-0 cursor-pointer self-start mt-2"
                    onclick={(e) => {
                        e.stopPropagation();
                        location.href = "/character?id=" + item.id;
                    }}
                >
                    <span class="text-[14px] font-semibold hover:text-pink"
                        >{item.name}</span
                    >
                    <img
                        alt=""
                        src={window.img("character", item.id, "grid")}
                        use:imgFallback={{
                            kind: "character",
                            id: item.id,
                            size: "grid"
                        }}
                        class="w-10 h-10 rounded-lg object-cover bg-surface"
                    />
                </div>
            {/if}
        </div>
    </div>
{/snippet}

{#if !sorted.length}
    <div>
        <SectionTitle text={title} extra={remoteExtra} />
        <div class="text-center py-6 text-sub">暂无{title}</div>
    </div>
{:else}
    <div>
        <SectionTitle text={title} extra={remoteExtra} />
        {#each pairs(visible) as pair}
            <div class="flex flex-col">
                <div class="flex-1 min-w-0">
                    {@render card(pair[0])}
                </div>
                <div class="flex-1 min-w-0">
                    {#if pair[1]}{@render card(pair[1])}{/if}
                </div>
            </div>
        {/each}
        {#if sorted.length > 10}
            <div class="mt-3 text-center">
                <span class="block mb-2 md:inline md:mb-0 text-xs text-sub"
                    >共 {sorted.length} 个条目</span
                >
                {#if visible.length < sorted.length}
                    <button
                        type="button"
                        onclick={() => (showing += 10)}
                        class="md:ml-3 px-3 py-2.5 md:py-1 rounded-lg border border-bord bg-transparent text-sub text-xs cursor-pointer hover:bg-active hover:text-text"
                        >展开更多</button
                    >
                    {#if sorted.length - visible.length > 10}
                        <button
                            type="button"
                            onclick={() => (showing = sorted.length)}
                            class="md:ml-2 px-3 py-2.5 md:py-1 rounded-lg border border-bord bg-transparent text-sub text-xs cursor-pointer hover:bg-active hover:text-text"
                            >展开全部</button
                        >
                    {/if}
                {/if}
            </div>
        {/if}
    </div>
{/if}
