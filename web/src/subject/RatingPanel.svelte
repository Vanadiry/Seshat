<script>
    let {
        rating = null,
        eloInfo = null,
        myRate = 0,
        myType = "",
        hasColl = false
    } = $props();

    let hasElo = $derived(!!(eloInfo && eloInfo.rating));
    let score = $derived(
        rating && rating.score ? rating.score.toFixed(1) : "-"
    );
    let total = $derived((rating && rating.total) || 0);
    let rank = $derived(rating && rating.rank ? "#" + rating.rank : "-");
    let count = $derived((rating && rating.count) || null);

    let totalVotes = $derived.by(() => {
        if (!count) return 0;
        var sum = 0;
        for (var i = 1; i <= 10; i++) sum += Number(count[String(i)] || 0);
        return sum;
    });

    let bars = $derived.by(() => {
        if (!count) return [];
        var max = 1;
        for (var k = 1; k <= 10; k++) {
            var value = Number(count[String(k)] || 0);
            if (value > max) max = value;
        }
        var out = [];
        for (var i = 10; i >= 1; i--) {
            var v = Number(count[String(i)] || 0);
            out.push({
                score: i,
                value: v,
                height: v > 0 ? Math.max((v / max) * 100, 4) : 0,
                pct:
                    totalVotes > 0
                        ? ((v / totalVotes) * 100).toFixed(1)
                        : "0.0",
                active: myRate === i
            });
        }
        return out;
    });

    let stddev = $derived.by(() => {
        if (!count || totalVotes <= 0) return "-";
        var mean = (rating && rating.score) || 0;
        var sumSq = 0;
        for (var i = 1; i <= 10; i++) {
            var c = Number(count[String(i)] || 0);
            sumSq += c * (i - mean) * (i - mean);
        }
        return Math.sqrt(sumSq / totalVotes).toFixed(4);
    });
</script>

<div
    class="w-full md:w-[230px] shrink-0 self-start bg-surface-alt border border-bord rounded-xl"
>
    <div class="px-4 pt-3 pb-2">
        {#if hasElo}
            <div class="text-xs text-sub text-center">ELO 评分</div>
            <div class="flex items-baseline justify-center">
                <span class="text-3xl font-bold"
                    >{eloInfo.rating.toFixed(1)}</span
                >
                <span class="text-xs text-sub ml-1"
                    >{(eloInfo.rank_pct || 0).toFixed(1)}%</span
                >
            </div>
            <div class="border-t border-bord mt-2 mb-2"></div>
        {/if}

        {#if hasColl && (myRate > 0 || myType)}
            <div class="flex items-center justify-center gap-2">
                <span class="text-xs text-sub">我的状态</span>
                {#if myType}
                    <span class="text-xs text-sub">{myType}</span>
                {/if}
            </div>
            <div class="flex items-baseline justify-center gap-2 mt-1">
                <span class="text-3xl font-bold"
                    >{myRate > 0 ? myRate : "-"}</span
                >
                <span class="text-xs text-sub">/ 10</span>
            </div>
            <div class="border-t border-bord mt-2 mb-2"></div>
        {/if}

        <div class="text-xs text-sub text-center">番组计划评分</div>
        <div class="flex items-baseline justify-center">
            <span class="text-3xl font-bold">{score}</span>
            <span class="text-xs text-sub ml-1">/ 10</span>
        </div>
        <div class="text-xs text-sub text-center mt-1">
            {total} 人评分 · 排名 {rank}
        </div>

        {#if count}
            <div class="flex items-end gap-px mt-2" style="height:80px">
                {#each bars as bar}
                    <div
                        class="flex-1 flex flex-col items-center justify-end group relative"
                        style="height:100%"
                    >
                        <span
                            data-score={bar.score}
                            class="block w-full rounded-t-sm transition-colors {bar.active
                                ? 'bg-pink'
                                : 'bg-pink/40 group-hover:bg-pink/70'}"
                            style="height:{bar.height}%"
                        ></span>
                        <div
                            class="absolute bottom-full mb-1 hidden group-hover:block z-50"
                        >
                            <div
                                class="bg-surface-raised border border-bord-strong rounded-lg px-2 py-1 text-xs whitespace-nowrap shadow-lg"
                            >
                                {bar.score}分 · {bar.pct}% · {bar.value} 人
                            </div>
                        </div>
                    </div>
                {/each}
            </div>
            <div class="flex gap-px mt-0.5">
                {#each [10, 9, 8, 7, 6, 5, 4, 3, 2, 1] as n}
                    <span class="flex-1 text-center text-[10px] text-sub"
                        >{n}</span
                    >
                {/each}
            </div>
            <div class="text-xs text-sub text-center mt-2">
                标准差: {stddev}
            </div>
        {/if}
    </div>
</div>
