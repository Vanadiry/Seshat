<script>
    import SectionTitle from "./SectionTitle.svelte";
    import EpisodeGrid from "./EpisodeGrid.svelte";
    import RatingPanel from "./RatingPanel.svelte";
    import { imgFallback } from "../lib/img.js";

    let {
        subject = {},
        infoItems = [],
        myRate = 0,
        myType = "",
        hasColl = false,
        eloInfo = null,
        eps = [],
        mainChars = [],
        rels = []
    } = $props();

    let posterSrc = $derived(window.img("subject", subject.id, "large"));

    let summaryHtml = $derived(
        window.formatSummary(window.escapeHtml(subject.summary) || "暂无简介")
    );

    let epExtra = $derived(
        (window.isRemote("/v0/episodes?subject_id=" + subject.id)
            ? window.globeIcon("w-4 h-4")
            : "") +
            '<span class="text-xs text-sub">(' +
            eps.length +
            ")</span>"
    );

    let charExtra = $derived(
        (window.isRemote("/v0/subjects/" + subject.id + "/characters")
            ? window.globeIcon("w-4 h-4")
            : "") +
            '<a href="javascript:switchTab(\'chars\')" class="text-pink text-xs ml-auto no-underline">更多角色 &raquo;</a>'
    );

    let relExtra = $derived(
        window.isRemote("/v0/subjects/" + subject.id + "/subjects")
            ? window.globeIcon("w-4 h-4")
            : ""
    );

    let sortedRels = $derived.by(() => {
        var list = (rels || []).slice();
        function priority(relation) {
            if (relation.indexOf("前传") !== -1) return 0;
            if (relation.indexOf("续") !== -1) return 1;
            if (relation.indexOf("番外") !== -1) return 2;
            if (relation.indexOf("书") !== -1) return 3;
            if (relation.indexOf("游戏") !== -1) return 4;
            return 99;
        }
        list.sort(function (a, b) {
            var pa = priority(a.relation || ""),
                pb = priority(b.relation || "");
            if (pa !== pb) return pa - pb;
            var ra = a.relation || "",
                rb = b.relation || "";
            if (ra < rb) return -1;
            if (ra > rb) return 1;
            return 0;
        });
        return list;
    });
</script>

<div class="fade-in flex flex-col md:flex-row gap-6">
    <div class="w-full md:w-[260px] shrink-0">
        <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
        <img
            alt=""
            src={posterSrc}
            use:imgFallback={{ kind: "subject", id: subject.id, size: "large" }}
            class="w-full rounded-xl cursor-pointer"
            tabindex="0"
            role="button"
            aria-label="查看大图"
            onkeydown={(e) => window.clickKeyNav(e)}
            onclick={() => window.showLightbox(posterSrc)}
        />
        {#if infoItems.length}
            <div class="mt-3 flex flex-col gap-0.5">
                {#each infoItems as item}
                    <div
                        class="select-text py-1 border-b border-bord-light text-[14px]"
                        class:ml-4={item[2]}
                    >
                        <span class="text-sub"
                            >{item[0] ? item[0] + ": " : ""}</span
                        ><span>{@html String(item[1])}</span>
                    </div>
                {/each}
            </div>
        {/if}
    </div>

    <div class="flex-1 min-w-0 flex flex-col gap-5">
        <div class="flex flex-col md:flex-row gap-5">
            <div class="flex-1 min-w-0">
                {#if eps.length}
                    <div class="mb-4">
                        <SectionTitle
                            text="剧集"
                            outer="mb-2"
                            extra={epExtra}
                        />
                        <EpisodeGrid {eps} />
                    </div>
                {/if}
                <SectionTitle text="简介" outer="mb-2" />
                <p
                    class="select-text text-[14px] leading-relaxed text-sub mb-4"
                >
                    {@html summaryHtml}
                </p>
                {#if subject.tags && subject.tags.length}
                    <div class="mb-2">
                        <SectionTitle text="标签" outer="mb-2" />
                        <div class="flex flex-wrap gap-x-2 gap-y-1">
                            {#each subject.tags.slice(0, 20) as tag}
                                <a
                                    href={"/tag?tag=" +
                                        encodeURIComponent(tag.name)}
                                    class="no-underline bg-surface-alt rounded-full px-2.5 py-0.5 text-[14px] text-sub border border-transparent hover:border-pink hover:bg-elo-bg transition-all"
                                    >{tag.name}<small
                                        class="text-xs text-sub ml-1"
                                        >{tag.count}</small
                                    ></a
                                >
                            {/each}
                        </div>
                    </div>
                {/if}
            </div>

            <RatingPanel
                rating={subject.rating}
                {eloInfo}
                {myRate}
                {myType}
                {hasColl}
            />
        </div>

        {#if mainChars.length}
            <div>
                <SectionTitle text="主要角色" extra={charExtra} />
                <div
                    class="flex gap-3 overflow-x-auto pb-2 thin-scroll"
                    style="padding-top:4px"
                >
                    {#each mainChars as c}
                        <div
                            tabindex="0"
                            role="link"
                            data-href={"/character?id=" + c.id}
                            onkeydown={(e) => window.cardKeyNav(e)}
                            onclick={() =>
                                (location.href = "/character?id=" + c.id)}
                            class="shrink-0 w-[96px] cursor-pointer group rounded-lg hover:-translate-y-1 transition-transform"
                        >
                            <div
                                class="relative rounded-lg overflow-hidden bg-surface"
                                style="aspect-ratio:1/1.414"
                            >
                                <img
                                    alt=""
                                    src={window.img("character", c.id, "small")}
                                    use:imgFallback={{
                                        kind: "character",
                                        id: c.id,
                                        size: "small"
                                    }}
                                    class="w-full h-full object-cover object-top"
                                />
                                <span
                                    class="absolute top-1 right-1 bg-black/60 text-white px-1.5 py-0.5 rounded text-[10px] font-semibold"
                                    >{c.relation}</span
                                >
                            </div>
                            <div class="mt-1.5 px-0.5">
                                <div class="text-[13px] font-semibold truncate">
                                    {window.primaryName(c.name || "", "")}
                                </div>
                                {#if c.actors && c.actors.length}
                                    <div class="text-[11px] text-sub truncate">
                                        CV: {c.actors[0].name}
                                    </div>
                                {/if}
                            </div>
                        </div>
                    {/each}
                </div>
            </div>
        {/if}

        {#if sortedRels.length}
            <div>
                <SectionTitle text="关联条目" extra={relExtra} />
                <div
                    class="flex items-start gap-3 overflow-x-auto pb-2 thin-scroll"
                    style="padding-top:4px"
                >
                    {#each sortedRels as r, i}
                        {#if i > 0 && r.relation !== sortedRels[i - 1].relation}
                            <div
                                class="border-l border-bord self-stretch shrink-0 -mx-1"
                            ></div>
                        {/if}
                        <div
                            tabindex="0"
                            role="link"
                            data-href={"/subject?id=" + r.id}
                            onkeydown={(e) => window.cardKeyNav(e)}
                            onclick={() =>
                                (location.href = "/subject?id=" + r.id)}
                            class="shrink-0 w-[96px] cursor-pointer group rounded-lg hover:-translate-y-1 transition-transform"
                        >
                            <div
                                class="relative rounded-lg overflow-hidden bg-surface aspect-square"
                            >
                                <img
                                    alt=""
                                    src={window.img("subject", r.id, "small")}
                                    use:imgFallback={{
                                        kind: "subject",
                                        id: r.id,
                                        size: "small"
                                    }}
                                    class="w-full h-full object-cover"
                                />
                                {#if r.relation}
                                    <span
                                        class="absolute top-1 right-1 bg-black/60 text-white px-1.5 py-0.5 rounded text-[10px] font-semibold"
                                        >{r.relation}</span
                                    >
                                {/if}
                            </div>
                            <div class="mt-1.5 px-0.5">
                                <div
                                    class="text-[13px] font-semibold line-clamp-3"
                                >
                                    {r.name_cn || r.name || "#" + r.id}
                                </div>
                            </div>
                        </div>
                    {/each}
                </div>
            </div>
        {/if}
    </div>
</div>
