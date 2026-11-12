<script>
    import { onMount } from "svelte";
    import { imgFallback } from "../lib/img.js";

    const API_BASE = window.BACKEND_URL || "/api";

    let pair = $state(null);
    let pkState = $state("idle");
    let loading = $state(true);
    let message = $state("");
    let blocked = $state(false);

    let history = $state([]);
    let historyLoading = $state(true);
    let histShowing = $state(10);
    let names = $state({});

    function sideID(side) {
        return pair[side === "left" ? 0 : 1].id;
    }

    async function loadPair() {
        pkState = "idle";
        loading = true;
        message = "";
        try {
            const data = await window.api("/v0/elo/pair");
            if (!data || data.length < 2) {
                pair = null;
                message = "需要至少 2 个已缓存条目";
            } else {
                pair = data;
            }
        } catch (_) {
            pair = null;
            message = "加载失败";
        }
        loading = false;
    }

    function setName(id, value) {
        names = { ...names, [id]: value };
    }

    async function ensureNames() {
        const visible = history.slice(0, histShowing);
        const missing = [];
        const seen = {};
        for (const row of visible) {
            for (const id of [row.winner, row.loser]) {
                if (!(id in names) && !seen[id]) {
                    seen[id] = true;
                    missing.push(id);
                }
            }
        }
        await Promise.all(
            missing.map(async function (id) {
                try {
                    const s = await window.api("/v0/subjects/" + id);
                    setName(
                        id,
                        s && s.id
                            ? { name: s.name || "", name_cn: s.name_cn || "" }
                            : null
                    );
                } catch (_) {
                    setName(id, null);
                }
            })
        );
    }

    async function loadHistory() {
        historyLoading = true;
        try {
            const data = await window.api("/v0/elo/history");
            history = data && data.data ? data.data.slice().reverse() : [];
            histShowing = Math.min(10, history.length);
        } catch (_) {
            history = [];
            histShowing = 0;
        }
        historyLoading = false;
        await ensureNames();
    }

    function showMoreHistory() {
        histShowing = Math.min(histShowing + 10, history.length);
        ensureNames();
    }

    async function handlePK(side) {
        if (!pair) return;
        if (pkState === "idle") {
            pkState = side;
            return;
        }
        if (pkState !== side) {
            pkState = "idle";
            return;
        }
        const winner = sideID(side);
        const loserSide = side === "left" ? "right" : "left";
        const loser = sideID(loserSide);
        try {
            await fetch(API_BASE + "/v0/elo/compare", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ winner: winner, loser: loser })
            });
        } catch (_) {}
        await Promise.all([loadPair(), loadHistory()]);
    }

    function dateText(t) {
        const d = new Date(t);
        return (
            d.getFullYear() +
            "-" +
            String(d.getMonth() + 1).padStart(2, "0") +
            "-" +
            String(d.getDate()).padStart(2, "0")
        );
    }

    function timeText(t) {
        const d = new Date(t);
        return (
            String(d.getHours()).padStart(2, "0") +
            ":" +
            String(d.getMinutes()).padStart(2, "0") +
            ":" +
            String(d.getSeconds()).padStart(2, "0")
        );
    }

    onMount(function () {
        (async function () {
            if (!(await window.requireFeature("elo"))) {
                blocked = true;
                loading = false;
                historyLoading = false;
                return;
            }
            loadPair();
            loadHistory();
        })();
    });
</script>

{#snippet sideCard(id)}
    {@const entry = names[id]}
    {@const name =
        entry === undefined ? "" : entry && entry.name ? entry.name : ""}
    {@const nameCN = entry && entry.name_cn ? entry.name_cn : ""}
    <a
        href={"/subject?id=" + id}
        style="flex:1 1 50%;max-width:50%;overflow:hidden"
        class="flex items-center gap-3 py-1.5 pl-2 pr-2 no-underline hover:bg-active transition-colors"
    >
        <img
            alt=""
            src={window.img("subject", id, "grid")}
            use:imgFallback={{ kind: "subject", id: id, size: "grid" }}
            class="w-8 h-8 rounded object-cover bg-input shrink-0"
        />
        <div class="min-w-0 flex-1">
            <div class="text-sm font-semibold truncate">
                {entry === undefined
                    ? "加载中…"
                    : name
                      ? window.primaryName(name, nameCN)
                      : "无标题"}
            </div>
            {#if entry !== undefined && name && window.subName(name, nameCN)}
                <div class="text-xs text-sub truncate">
                    {window.subName(name, nameCN)}
                </div>
            {/if}
        </div>
        <span class="text-xs text-sub shrink-0">#{id}</span>
    </a>
{/snippet}

{#if !blocked}
    <div class="flex items-center gap-3 mb-4">
        <span class="section-bar"></span>
        <span class="font-bold">评分</span>
        <button
            type="button"
            onclick={loadPair}
            class="ml-2 px-3 py-1 rounded-lg text-xs cursor-pointer border border-bord bg-transparent text-sub hover:text-text"
        >
            刷新
        </button>
    </div>

    {#if loading}
        <div id="pk-loading" class="text-center py-20 text-sub">加载中…</div>
    {:else if !pair}
        <div id="pk-loading" class="text-center py-20 text-sub">
            {message}
        </div>
    {:else}
        <div
            id="pk-area"
            class="flex gap-2 mb-6"
            class:pk-picked={pkState !== "idle"}
            style="min-height: 240px"
        >
            {#each ["left", "right"] as side, i}
                {@const item = pair[i]}
                {@const winner = pkState === side}
                {@const loser = pkState !== "idle" && pkState !== side}
                <div
                    id={"pk-" + side}
                    class="flex-1 bg-surface border border-bord-mid rounded-xl p-6 flex flex-col items-center justify-center cursor-pointer relative"
                    class:shrunk={loser}
                    style="height: 298px; flex: {winner
                        ? '1.2'
                        : loser
                          ? '0 0 200px'
                          : '1'}"
                    tabindex="0"
                    role="button"
                    aria-label={"选择" +
                        (side === "left" ? "左" : "右") +
                        "侧条目"}
                    onkeydown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            handlePK(side);
                        }
                    }}
                    onclick={() => handlePK(side)}
                >
                    <div
                        id={"pk-" + side + "-img"}
                        class="relative shrink-0 rounded-lg overflow-hidden bg-input"
                        style="width: 140px; aspect-ratio: 1/1.414"
                    >
                        <img
                            alt=""
                            id={"pk-" + side + "-img-el"}
                            src={window.img("subject", item.id, "medium")}
                            use:imgFallback={{
                                kind: "subject",
                                id: item.id,
                                size: "grid"
                            }}
                            class="w-full h-full object-cover"
                        />
                        <span
                            id={"pk-" + side + "-id"}
                            class="absolute top-0.5 right-0.5 text-[9px] font-semibold bg-black/60 text-white px-1 py-px rounded"
                            >#{item.id}</span
                        >
                    </div>
                    <div
                        id={"pk-" + side + "-name"}
                        class="text-sm font-semibold mt-3 text-center truncate w-full"
                    >
                        {window.primaryName(item.name, item.name_cn)}
                    </div>
                    {#if window.subName(item.name, item.name_cn)}
                        <div
                            id={"pk-" + side + "-sub"}
                            class="text-xs text-sub mt-0.5 text-center truncate w-full"
                        >
                            {window.subName(item.name, item.name_cn)}
                        </div>
                    {/if}
                    <div
                        id={"pk-" + side + "-confirm"}
                        class="confirm-bar absolute top-0 left-0 right-0 h-[40%] flex items-start justify-center"
                        class:show={winner}
                    >
                        <span class="text-pink font-semibold text-sm"
                            >再次点击以确认该条目胜出</span
                        >
                    </div>
                    <div class="cancel-hint">点击此处取消</div>
                </div>
            {/each}
        </div>
    {/if}

    <div class="flex items-center gap-2 mb-3">
        <span class="section-bar"></span><span class="font-bold">历史</span>
    </div>

    {#if historyLoading}
        <div class="text-center py-10 text-sub">
            <span class="spin"></span>加载中…
        </div>
    {:else if history.length}
        <div id="history-area">
            <div
                class="flex gap-7 text-xs text-sub font-semibold mb-1 border-b border-bord-light pb-1"
            >
                <div class="flex-1 text-center">胜</div>
                <div class="flex-1 text-center">负</div>
                <div class="w-[100px] shrink-0 text-center">时间</div>
            </div>
            {#each history.slice(0, histShowing) as row, i}
                <div
                    class="flex gap-7 border border-transparent {i % 2 === 0
                        ? 'bg-surface'
                        : 'bg-surface-raised'}"
                >
                    {@render sideCard(row.winner)}
                    {@render sideCard(row.loser)}
                    {#if row.time}
                        <div
                            class="w-[100px] shrink-0 flex flex-col items-center justify-center py-1.5"
                        >
                            <div class="text-sm text-sub">
                                {dateText(row.time)}
                            </div>
                            <div class="text-xs text-sub">
                                {timeText(row.time)}
                            </div>
                        </div>
                    {/if}
                </div>
            {/each}
        </div>
        <div id="history-pager" class="mt-3 text-center">
            {#if history.length > 10}
                <span class="text-xs text-sub"
                    >当前展示 {Math.min(histShowing, history.length)}，共 {history.length}
                    条记录</span
                >
                {#if histShowing < history.length}
                    <button
                        type="button"
                        onclick={showMoreHistory}
                        class="ml-3 px-3 py-1 rounded-lg border border-bord bg-transparent text-sub text-xs cursor-pointer hover:bg-active hover:text-text"
                        >展开更多 10 条</button
                    >
                {/if}
            {:else}
                <span class="text-xs text-sub">共 {history.length} 条记录</span>
            {/if}
        </div>
    {:else}
        <div id="history-area"></div>
        <div id="history-pager"></div>
    {/if}
{/if}
