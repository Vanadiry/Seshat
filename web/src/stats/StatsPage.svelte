<script>
    import { onMount } from "svelte";
    import { imgFallback } from "../lib/img.js";

    const COL_ELO = "w-[120px] shrink-0";
    const COL_MY = "w-[90px] shrink-0";
    const COL_BGM = "w-[100px] shrink-0";
    const COL_ID = "w-[60px] shrink-0";

    const collator = new Intl.Collator("zh-Hans", {
        numeric: true,
        sensitivity: "base"
    });

    let stats = $state(null);
    let entries = $state([]);
    let noRating = $state([]);
    let orphans = $state([]);
    let myRates = $state({});
    let orphanNames = $state({});
    let sortKey = $state("elo");
    let sortDir = $state("desc");
    let loading = $state(true);
    let error = $state("");
    let blocked = $state(false);

    let cards = $derived.by(() => {
        if (!stats) return [];
        const out = [];
        if (stats.data_entries) {
            const total =
                stats.data_entries.subjects +
                stats.data_entries.characters +
                stats.data_entries.persons;
            out.push({
                label: "条目（共" + total + "项）",
                value:
                    stats.data_entries.subjects +
                    " / " +
                    stats.data_entries.characters +
                    " / " +
                    stats.data_entries.persons,
                sub: "动画 / 角色 / 人物"
            });
        }
        if (stats.images) {
            out.push({
                label: "图片（共" + (stats.images.total || 0) + "项）",
                value:
                    stats.images.unique_subjects +
                    " / " +
                    stats.images.unique_characters +
                    " / " +
                    stats.images.unique_persons,
                sub: "动画 / 角色 / 人物"
            });
        }
        if (stats.elo_scores != null) {
            out.push({
                label: "ELO 评分",
                value: stats.elo_scores + " / " + stats.elo_comparisons,
                sub: "有评分条目 / 评分次数"
            });
        }
        if (stats.trackers) {
            out.push({
                label: "Tracker",
                value: stats.trackers.length + " / " + stats.tracker_total,
                sub: "Tracker / 条目总数"
            });
        }
        return out;
    });

    let items = $derived.by(() => {
        const list = entries.concat(noRating);
        for (const o of orphans) {
            const cached = orphanNames[o.id];
            list.push({
                id: o.id,
                name: cached ? cached.name : undefined,
                name_cn: cached ? cached.name_cn : undefined,
                score: cached ? cached.score : undefined,
                rating: o.rating,
                rank_pct: o.rank_pct,
                _orphan: true
            });
        }
        return list;
    });

    function sortValue(it, key) {
        if (key === "elo") {
            if (it.rating == null || it.rating === 0 || it.rating === "")
                return null;
            return Number(it.rating);
        }
        if (key === "my") {
            const rate = myRates[it.id];
            if (rate == null || rate === 0 || rate === "") return null;
            return Number(rate);
        }
        if (key === "score") {
            if (it.score == null || it.score === 0 || it.score === "")
                return null;
            return Number(it.score);
        }
        return null;
    }

    let sorted = $derived.by(() => {
        const list = items.slice();
        const order = ["elo", "my", "score"];
        const keys = [sortKey].concat(
            order.filter(function (k) {
                return k !== sortKey;
            })
        );
        list.sort(function (a, b) {
            for (let k = 0; k < keys.length; k++) {
                const av = sortValue(a, keys[k]);
                const bv = sortValue(b, keys[k]);
                const am = av === null;
                const bm = bv === null;
                if (am && bm) continue;
                if (am) return 1;
                if (bm) return -1;
                if (av !== bv) {
                    return sortDir === "asc" ? av - bv : bv - av;
                }
            }
            const an = window.primaryName(a.name || "", a.name_cn || "");
            const bn = window.primaryName(b.name || "", b.name_cn || "");
            const d = collator.compare(an, bn);
            if (d !== 0) return d;
            return (a.id || 0) - (b.id || 0);
        });
        return list;
    });

    function sortBy(key) {
        if (sortKey === key) {
            sortDir = sortDir === "asc" ? "desc" : "asc";
        } else {
            sortKey = key;
            sortDir = "desc";
        }
    }

    function setOrphanName(id, value) {
        orphanNames = { ...orphanNames, [id]: value };
    }

    async function ensureOrphanNames() {
        const missing = orphans.filter((o) => !(o.id in orphanNames));
        await Promise.all(
            missing.map(async function (o) {
                try {
                    const s = await window.api("/v0/subjects/" + o.id);
                    setOrphanName(
                        o.id,
                        s && s.id
                            ? {
                                  name: s.name || "",
                                  name_cn: s.name_cn || "",
                                  score:
                                      s.rating && s.rating.score
                                          ? s.rating.score
                                          : null
                              }
                            : null
                    );
                } catch (_) {
                    setOrphanName(o.id, null);
                }
            })
        );
    }

    async function loadStats() {
        try {
            const s = await window.api("/v0/stats");
            if (s) stats = s;
        } catch (_) {}
    }

    async function load() {
        try {
            const data = (await window.apiFor("elo", "/v0/elo/ranking")) || {};
            entries = data && data.data ? data.data : [];
            noRating = data && data.no_rating ? data.no_rating : [];
            orphans = data && data.orphans ? data.orphans : [];
            myRates = {};
            if (window.USERNAME) {
                try {
                    const summary = await window.api(
                        "/v0/users/" +
                            window.USERNAME +
                            "/collections?view=summary"
                    );
                    if (summary && summary.data) {
                        const next = {};
                        for (let i = 0; i < summary.data.length; i++) {
                            const rate = Number(summary.data[i].rate);
                            if (rate > 0) {
                                next[summary.data[i].subject_id] = rate;
                            }
                        }
                        myRates = next;
                    }
                } catch (_) {}
            }
        } catch (_) {
            error = "加载失败";
        }
        loading = false;
        ensureOrphanNames();
    }

    onMount(function () {
        (async function () {
            if (!(await window.requireFeature("stats"))) {
                blocked = true;
                loading = false;
                return;
            }
            loadStats();
            load();
        })();
    });
</script>

{#snippet sortHeader(label, key, cls)}
    <div
        class="{cls} text-center cursor-pointer select-none hover:text-text {sortKey ===
        key
            ? 'text-pink font-semibold'
            : ''}"
        data-sort-key={key}
        title="点击排序"
        role="button"
        tabindex="0"
        onkeydown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                sortBy(key);
            }
        }}
        onclick={() => sortBy(key)}
    >
        {label}{sortKey === key ? (sortDir === "asc" ? " ↑" : " ↓") : ""}
    </div>
{/snippet}

{#snippet eloCell(rating, pct, cls)}
    <span class="{cls} {COL_ELO} flex justify-center gap-1 font-mono">
        {#if rating && pct}
            <span class="flex-1 text-right font-mono">{rating}</span><span
                class="text-sub flex-1 text-right font-mono">{pct}</span
            >
        {:else if pct}
            <span class="flex-1 text-right font-mono text-sub">{pct}</span>
        {:else}
            <span class="flex-1 text-right font-mono">{rating || "-"}</span>
        {/if}
    </span>
{/snippet}

{#if !blocked}
    <div
        id="stats-dash"
        class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6 text-xs text-sub"
    >
        {#each cards as c}
            <div
                class="bg-surface border border-bord-light rounded-xl p-3 flex items-center"
                style="max-width:260px;max-height:100px"
            >
                <div class="min-w-0 flex-1">
                    <div class="text-sub truncate">{c.label}</div>
                    <div class="text-lg font-bold mt-1 truncate">
                        {c.value}
                    </div>
                    {#if c.sub}
                        <div class="text-xs text-sub mt-0.5 truncate">
                            {c.sub}
                        </div>
                    {/if}
                </div>
            </div>
        {/each}
    </div>

    <div id="stats-list" class="thin-scroll overflow-x-auto">
        {#if loading}
            <div class="text-center py-20 text-sub">
                <span class="spin"></span>加载中…
            </div>
        {:else if error}
            <div class="text-center py-20 text-sub">{error}</div>
        {:else}
            <div
                class="flex items-center gap-4 px-3 py-1.5 text-xs text-sub border-b border-bord-light"
            >
                <div class="w-10 shrink-0"></div>
                <div class="flex-1 min-w-0">动画</div>
                {@render sortHeader("ELO 评分", "elo", COL_ELO)}
                {@render sortHeader("我的评分", "my", COL_MY)}
                {@render sortHeader("番组计划评分", "score", COL_BGM)}
                <div class="{COL_ID} text-center">ID</div>
            </div>
            {#each sorted as it, i (it.id + (it._orphan ? "-o" : ""))}
                {@const bg = i % 2 === 0 ? "bg-surface" : "bg-surface-raised"}
                {@const eloCls =
                    sortKey === "elo"
                        ? "text-sm font-semibold"
                        : "text-sm text-sub"}
                {@const myCls =
                    sortKey === "my"
                        ? "text-sm font-semibold"
                        : "text-sm text-sub"}
                {@const bkgCls =
                    sortKey === "score"
                        ? "text-sm font-semibold"
                        : "text-sm text-sub"}
                {#if it._orphan}
                    {@const entry = orphanNames[it.id]}
                    {@const ratingVal = it.rating
                        ? it.rating.toFixed(1)
                        : "1500"}
                    {@const pctVal =
                        it.rank_pct != null ? it.rank_pct.toFixed(1) + "%" : ""}
                    <div
                        id={"orphan-" + it.id}
                        class="flex items-center gap-4 px-3 py-1.5 border border-transparent {bg}"
                    >
                        <img
                            alt=""
                            src={window.img("subject", it.id, "grid")}
                            use:imgFallback={{
                                kind: "subject",
                                id: it.id,
                                size: "grid"
                            }}
                            class="w-10 h-10 rounded object-cover bg-input shrink-0"
                        />
                        <div class="flex-1 min-w-0">
                            <div
                                class="text-sm font-semibold truncate {entry &&
                                entry.name
                                    ? ''
                                    : 'text-sub'}"
                            >
                                {#if entry === undefined}
                                    加载中…
                                {:else if entry && entry.name}
                                    {window.primaryName(
                                        entry.name,
                                        entry.name_cn
                                    )}
                                {:else}
                                    无标题
                                {/if}
                            </div>
                            {#if entry !== undefined && entry && entry.name}
                                {#if window.subName(entry.name, entry.name_cn)}
                                    <div class="text-xs text-sub truncate">
                                        {window.subName(
                                            entry.name,
                                            entry.name_cn
                                        )}
                                    </div>
                                {/if}
                            {:else if entry !== undefined}
                                <div class="text-xs text-sub truncate">
                                    未能获取此条目的数据，可能是未储存此条目，或数据端点异常
                                </div>
                            {/if}
                        </div>
                        {@render eloCell(ratingVal, pctVal, eloCls)}
                        <span class="{myCls} {COL_MY} text-center font-mono"
                            >{myRates[it.id] || "-"}</span
                        >
                        <span class="{bkgCls} {COL_BGM} text-center font-mono"
                            >{entry && entry.score != null
                                ? entry.score.toFixed(1)
                                : "-"}</span
                        >
                        <span
                            class="text-xs text-sub {COL_ID} text-center font-mono"
                            >#{it.id}</span
                        >
                    </div>
                {:else}
                    {@const ratingVal = it.rating
                        ? typeof it.rating === "number"
                            ? it.rating.toFixed(1)
                            : it.rating
                        : ""}
                    {@const pctVal =
                        it.rank_pct != null ? it.rank_pct.toFixed(1) + "%" : ""}
                    <a
                        href={"/subject?id=" + it.id}
                        class="flex items-center gap-4 px-3 py-1.5 cursor-pointer hover:border-pink border border-transparent no-underline {bg}"
                    >
                        <img
                            alt=""
                            src={window.img("subject", it.id, "grid")}
                            use:imgFallback={{
                                kind: "subject",
                                id: it.id,
                                size: "grid"
                            }}
                            class="w-10 h-10 rounded object-cover bg-input shrink-0"
                        />
                        <div class="flex-1 min-w-0">
                            <div class="text-sm font-semibold truncate">
                                {window.primaryName(it.name, it.name_cn)}
                            </div>
                            {#if window.subName(it.name, it.name_cn)}
                                <div class="text-xs text-sub truncate">
                                    {window.subName(it.name, it.name_cn)}
                                </div>
                            {/if}
                        </div>
                        {@render eloCell(ratingVal, pctVal, eloCls)}
                        <span class="{myCls} {COL_MY} text-center font-mono"
                            >{myRates[it.id] || "-"}</span
                        >
                        <span class="{bkgCls} {COL_BGM} text-center font-mono"
                            >{it.score ? it.score.toFixed(1) : "-"}</span
                        >
                        <span
                            class="text-xs text-sub {COL_ID} text-center font-mono"
                            >#{it.id}</span
                        >
                    </a>
                {/if}
            {/each}
        {/if}
    </div>
{/if}
