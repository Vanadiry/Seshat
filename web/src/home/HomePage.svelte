<script>
    import { onMount } from "svelte";
    import { imgFallback } from "../lib/img.js";

    let items = $state([]);
    let loading = $state(true);
    let error = $state("");
    let searchMode = $state(false);
    let searchEmpty = $state(false);
    let showing = $state(100);

    let visible = $derived(items.slice(0, showing));

    function sortValue(it, key) {
        if (key === "elo") return it._elo ? Number(it._elo) : null;
        if (key === "my") return it._my ? Number(it._my) : null;
        if (key === "bgm") return it.score ? Number(it.score) : null;
        return null;
    }

    function sortByOrder(list, order, pinWatching) {
        list.sort(function (a, b) {
            if (pinWatching) {
                var aw = Number(a._coll) === 3 ? 1 : 0;
                var bw = Number(b._coll) === 3 ? 1 : 0;
                if (aw !== bw) return bw - aw;
            }
            for (var i = 0; i < order.length; i++) {
                var av = sortValue(a, order[i]);
                var bv = sortValue(b, order[i]);
                var am = av === null;
                var bm = bv === null;
                if (am && bm) continue;
                if (am) return 1;
                if (bm) return -1;
                if (av !== bv) return bv - av;
            }
            var an = window.primaryName(a.name || "", a.name_cn || "");
            var bn = window.primaryName(b.name || "", b.name_cn || "");
            var d = String(an).localeCompare(String(bn), "zh-Hans", {
                numeric: true,
                sensitivity: "base"
            });
            if (d !== 0) return d;
            return (a.id || 0) - (b.id || 0);
        });
    }

    function sortByPrimary(list, primary) {
        sortByOrder(
            list,
            [primary].concat(
                ["elo", "my", "bgm"].filter(function (k) {
                    return k !== primary;
                })
            ),
            false
        );
    }

    function finishLoad(subjects) {
        var sort = window.SUBJECT_SORT || "elo";
        var list = subjects.slice();
        if (sort === "elo") {
            sortByPrimary(list, "elo");
        } else if (sort === "my") {
            sortByPrimary(list, "my");
        } else if (sort === "bgm_rank") {
            sortByPrimary(list, "bgm");
        } else if (sort === "watching") {
            sortByOrder(list, ["elo", "my", "bgm"], true);
        } else if (sort === "random") {
            list.sort(function () {
                return Math.random() - 0.5;
            });
        }
        items = list;
        searchMode = false;
        searchEmpty = false;
        loading = false;
    }

    async function load() {
        loading = true;
        error = "";
        var q = new URLSearchParams(location.search).get("q");
        if (q) {
            await searchLoad(q);
            return;
        }
        try {
            var resp = await window.apiFor("lists", "/v0/subjects/list");
            var subjects = (resp && resp.data) || [];
            try {
                var results = await Promise.all([
                    window.apiFor("elo", "/v0/elo/ranking"),
                    window.USERNAME
                        ? window.api(
                              "/v0/users/" + window.USERNAME + "/collections"
                          )
                        : Promise.resolve(null)
                ]);
                var eloData = results[0],
                    collData = results[1];
                if (eloData && eloData.data) {
                    var eloMap = {};
                    for (var i = 0; i < eloData.data.length; i++) {
                        var e = eloData.data[i];
                        eloMap[e.id] = e.rating
                            ? Number(e.rating).toFixed(1)
                            : null;
                    }
                    for (var i = 0; i < subjects.length; i++) {
                        if (eloMap[subjects[i].id]) {
                            subjects[i]._elo = eloMap[subjects[i].id];
                        }
                    }
                }
                if (collData && collData.data) {
                    var collMap = {};
                    for (var i = 0; i < collData.data.length; i++) {
                        var item = collData.data[i];
                        collMap[item.subject_id] = {
                            type: item.type,
                            rate: Number(item.rate) || 0
                        };
                    }
                    for (var i = 0; i < subjects.length; i++) {
                        var coll = collMap[subjects[i].id];
                        if (!coll) continue;
                        subjects[i]._coll = coll.type;
                        if (coll.rate > 0) {
                            subjects[i]._my = coll.rate;
                        }
                    }
                }
            } catch (_) {}
            finishLoad(subjects);
        } catch (e) {
            error = "无法连接";
            loading = false;
        }
    }

    async function searchLoad(q) {
        searchMode = true;
        searchEmpty = false;
        try {
            var d = await window.api(
                "/v0/search?q=" + encodeURIComponent(q) + "&type=subjects"
            );
            if (!d) {
                error = "搜索出错";
                loading = false;
                return;
            }
            items = d.subjects || [];
            searchEmpty = !items.length;
        } catch (_) {
            error = "无法连接";
        }
        loading = false;
    }

    onMount(function () {
        window.onFetchDone = load;
        load();
    });
</script>

{#if loading}
    <div
        class="grid grid-cols-1 min-[300px]:grid-cols-2 min-[600px]:grid-cols-[repeat(auto-fill,minmax(min(175px,calc((100%_-_2rem)/3)),1fr))] gap-4"
    >
        <div class="col-span-full text-center py-20 text-sub">
            <span class="spin"></span>{searchMode ? "搜索中…" : "加载中…"}
        </div>
    </div>
{:else if error}
    <div
        class="grid grid-cols-1 min-[300px]:grid-cols-2 min-[600px]:grid-cols-[repeat(auto-fill,minmax(min(175px,calc((100%_-_2rem)/3)),1fr))] gap-4"
    >
        <div class="col-span-full text-center py-20 text-sub">{error}</div>
    </div>
{:else if searchEmpty}
    <div
        class="grid grid-cols-1 min-[300px]:grid-cols-2 min-[600px]:grid-cols-[repeat(auto-fill,minmax(min(175px,calc((100%_-_2rem)/3)),1fr))] gap-4"
    >
        <div class="col-span-full text-center py-20 text-sub">未找到结果</div>
    </div>
{:else if !items.length}
    <div
        class="grid grid-cols-1 min-[300px]:grid-cols-2 min-[600px]:grid-cols-[repeat(auto-fill,minmax(min(175px,calc((100%_-_2rem)/3)),1fr))] gap-4"
    >
        <div class="col-span-full text-center py-20 text-sub">
            暂无数据。点击「管理数据」添加。<br /><br />首次使用请阅读<a
                href="https://github.com/Vanadiry/Seshat/blob/main/docs/guide.md"
                onclick={(e) => {
                    e.preventDefault();
                    window.openUrl(
                        "https://github.com/Vanadiry/Seshat/blob/main/docs/guide.md"
                    );
                }}
                class="text-pink hover:underline ml-1 select-text">用户指南</a
            >
        </div>
    </div>
{:else}
    <div
        id="grid"
        class="grid grid-cols-1 min-[300px]:grid-cols-2 min-[600px]:grid-cols-[repeat(auto-fill,minmax(min(175px,calc((100%_-_2rem)/3)),1fr))] gap-4"
    >
        {#each visible as s}
            <a
                href={"/subject?id=" + s.id}
                class="fade-in block no-underline bg-surface rounded-xl overflow-hidden cursor-pointer border border-bord hover:border-pink hover:-translate-y-0.5 hover:shadow-[0_4px_20px_rgba(254,138,149,.1)] transition-all"
            >
                <div class="relative">
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
                    {#if s._elo || s._coll}
                        <div
                            class="absolute top-1 right-1 flex flex-col items-end gap-0.5"
                        >
                            {#if s._elo}
                                <span
                                    class="bg-elo-bg text-elo-text px-1.5 py-px rounded text-[10px] font-semibold"
                                    >ELO {s._elo}</span
                                >
                            {/if}
                            {#if s._coll}
                                {@html window.collBadge(s._coll)}
                            {/if}
                        </div>
                    {/if}
                </div>
                <div class="p-2.5">
                    <div class="text-sm font-semibold truncate">
                        {window.primaryName(s.name, s.name_cn)}
                    </div>
                    <div
                        class="text-xs text-sub truncate mt-0.5 {window.subName(
                            s.name,
                            s.name_cn
                        )
                            ? ''
                            : 'invisible'}"
                    >
                        {window.subName(s.name, s.name_cn) || " "}
                    </div>
                    <div
                        class="text-xs text-sub mt-1 flex gap-1.5 items-center truncate"
                    >
                        {#if s.score}
                            <span
                                class="bg-pink-solid text-white px-1.5 py-px rounded-md text-[12px] font-bold shrink-0"
                                >★ {s.score.toFixed(1)}</span
                            >
                        {/if}
                        {#if s.platform}
                            <span
                                class="bg-pink-bg text-pink px-1.5 py-px rounded-md text-[12px] shrink-0"
                                >{s.platform}</span
                            >
                        {/if}
                        {#if s.date}
                            <span class="text-sub text-[12px] shrink-0"
                                >{s.date.slice(0, 4)}</span
                            >
                        {/if}
                    </div>
                </div>
            </a>
        {/each}
    </div>
    <div id="pager" class="mt-6 text-center">
        {#if items.length <= 100}
            <span class="block mb-2 md:inline md:mb-0 text-xs text-sub"
                >共 {items.length} 个项目</span
            >
        {:else}
            <span class="block mb-2 md:inline md:mb-0 text-xs text-sub"
                >当前展示 {visible.length}，共 {items.length} 个项目</span
            >
            {#if visible.length < items.length}
                <button
                    type="button"
                    onclick={() => (showing += 100)}
                    class="md:ml-3 px-3 py-1 rounded-lg border border-bord bg-transparent text-sub text-xs cursor-pointer hover:bg-active hover:text-text"
                    >展开更多</button
                >
                <button
                    type="button"
                    onclick={() => (showing = items.length)}
                    class="md:ml-2 px-3 py-1 rounded-lg border border-bord bg-transparent text-sub text-xs cursor-pointer hover:bg-active hover:text-text"
                    >展开全部</button
                >
            {/if}
        {/if}
    </div>
{/if}
