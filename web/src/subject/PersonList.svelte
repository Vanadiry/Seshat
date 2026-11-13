<script>
    import SectionTitle from "./SectionTitle.svelte";

    let { persons = [] } = $props();

    let groups = $derived.by(() => {
        var map = {};
        for (var i = 0; i < persons.length; i++) {
            var key = persons[i].relation || "其他";
            if (!map[key]) map[key] = [];
            map[key].push(persons[i]);
        }
        return Object.entries(map);
    });
</script>

<div class="select-text fade-in">
    {#if !persons.length}
        <div class="text-center py-10 text-sub">暂无制作人员</div>
    {:else}
        <div class="flex gap-6">
            <div class="flex-1 columns-1 md:columns-2 gap-x-6">
                {#each groups as [key, list], i}
                    <div
                        class="mb-4 bg-surface-alt rounded-lg p-3 scroll-mt-28"
                        id={"g-" + i}
                        style="break-inside:avoid"
                    >
                        <SectionTitle
                            text={key}
                            outer="mb-2"
                            extra={'<span class="text-xs text-sub">(' +
                                list.length +
                                ")</span>"}
                        />
                        <div class="grid grid-cols-2 gap-x-4 gap-y-0.5">
                            {#each list as p}
                                <a
                                    href={"/person?id=" + p.id}
                                    class="flex items-center gap-2 py-2 md:py-1 border-b border-bord-light cursor-pointer hover:text-pink text-sm truncate no-underline"
                                    >{p.name}{#if p.eps}<span
                                            class="text-xs text-sub ml-1 shrink-0"
                                            >({p.eps})</span
                                        >{/if}</a
                                >
                            {/each}
                        </div>
                    </div>
                {/each}
            </div>
            <div
                class="w-[140px] shrink-0 hidden md:block sticky sticky-below-detail-header self-start"
            >
                <div class="bg-surface-alt rounded-lg overflow-hidden">
                    <div
                        class="p-3 max-h-[calc(100vh-8rem)] overflow-y-auto thin-scroll"
                    >
                        {#each groups as [key, list], i}
                            <a
                                href={"#g-" + i}
                                class="text-sub text-xs no-underline block py-0.5 hover:text-text"
                                >{key} ({list.length})</a
                            >
                        {/each}
                    </div>
                </div>
            </div>
        </div>
    {/if}
</div>
