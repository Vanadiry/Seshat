<script>
    let {
        rating = null,
        eloInfo = null,
        myRate = 0,
        myType = "",
        hasColl = false
    } = $props();

    let hasElo = $derived(!!(eloInfo && eloInfo.rating));
    let hasMy = $derived(hasColl && (myRate > 0 || !!myType));
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

    // 窄屏没有悬停，点击柱子固定提示
    let activeBar = $state(null);

    $effect(() => {
        if (activeBar === null) return;
        var close = function (e) {
            var el =
                e.target instanceof Element
                    ? e.target.closest("[data-bar]")
                    : null;
            if (!el) activeBar = null;
        };
        document.addEventListener("click", close);
        return function () {
            document.removeEventListener("click", close);
        };
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
        {#if hasElo || hasMy}
            <div
                class="flex flex-col min-[450px]:flex-row min-[450px]:items-stretch md:flex-col"
            >
                {#if hasElo}
                    <div class="min-[450px]:flex-1 md:flex-none">
                        <div class="text-xs text-sub text-center">ELO 评分</div>
                        <div class="flex items-baseline justify-center">
                            <span class="text-3xl font-bold"
                                >{eloInfo.rating.toFixed(1)}</span
                            >
                            <span class="text-xs text-sub ml-1"
                                >{(eloInfo.rank_pct || 0).toFixed(1)}%</span
                            >
                        </div>
                    </div>
                {/if}

                {#if hasElo && hasMy}
                    <div
                        class="border-t border-bord mt-2 mb-2 min-[450px]:hidden md:block"
                    ></div>
                    <div
                        class="hidden min-[450px]:block md:hidden w-px bg-bord shrink-0 mx-3 self-stretch"
                    ></div>
                {/if}

                {#if hasMy}
                    <div class="min-[450px]:flex-1 md:flex-none">
                        <div class="flex items-center justify-center gap-2">
                            <span class="text-xs text-sub">我的状态</span>
                            {#if myType}
                                <span class="text-xs text-sub">{myType}</span>
                            {/if}
                        </div>
                        <div
                            class="flex items-baseline justify-center gap-2 mt-1"
                        >
                            <span class="text-3xl font-bold"
                                >{myRate > 0 ? myRate : "-"}</span
                            >
                            <span class="text-xs text-sub">/ 10</span>
                        </div>
                    </div>
                {/if}
            </div>
            <div class="border-t border-bord mt-2 mb-2"></div>
        {/if}

        <div
            class="flex flex-col min-[450px]:grid min-[450px]:grid-cols-2 min-[450px]:items-center min-[450px]:gap-3 md:flex md:flex-col md:items-stretch md:gap-0"
        >
            <div>
                <div class="text-xs text-sub text-center">番组计划评分</div>
                <div class="flex items-baseline justify-center">
                    <span class="text-3xl font-bold">{score}</span>
                    <span class="text-xs text-sub ml-1">/ 10</span>
                </div>
                <div class="text-xs text-sub text-center mt-1">
                    {total} 人评分，排名 {rank}
                </div>
                {#if count}
                    <div
                        class="hidden min-[450px]:block md:hidden text-xs text-sub text-center mt-1"
                    >
                        标准差：{stddev}
                    </div>
                {/if}
            </div>

            {#if count}
                <div class="min-w-0 mt-2 min-[450px]:mt-0 md:mt-2">
                    <div
                        class="flex items-end gap-px h-20 min-[450px]:h-16 md:h-20"
                    >
                        {#each bars as bar}
                            <div
                                class="flex-1 flex flex-col items-center justify-end group relative cursor-pointer"
                                style="height:100%"
                                data-bar={bar.score}
                                role="button"
                                tabindex="0"
                                aria-label="{bar.score} 分，{bar.pct}%，{bar.value} 人"
                                onclick={() =>
                                    (activeBar =
                                        activeBar === bar.score
                                            ? null
                                            : bar.score)}
                                onkeydown={(e) => {
                                    if (e.key === "Enter" || e.key === " ") {
                                        e.preventDefault();
                                        activeBar =
                                            activeBar === bar.score
                                                ? null
                                                : bar.score;
                                    }
                                }}
                            >
                                <span
                                    data-score={bar.score}
                                    class="block w-full rounded-t-sm transition-colors {bar.active
                                        ? 'bg-pink'
                                        : 'bg-pink/40 group-hover:bg-pink/70'}"
                                    style="height:{bar.height}%"
                                ></span>
                                <div
                                    class="absolute bottom-full mb-1 z-50 {bar.score >=
                                    6
                                        ? 'left-0'
                                        : 'right-0'} {activeBar === bar.score
                                        ? ''
                                        : 'hidden'} group-hover:block"
                                >
                                    <div
                                        class="bg-surface-raised border border-bord-strong rounded-lg px-2 py-1 text-xs whitespace-nowrap shadow-lg"
                                    >
                                        {bar.score} 分，{bar.pct}%，{bar.value}
                                        人
                                    </div>
                                </div>
                            </div>
                        {/each}
                    </div>
                    <div class="flex gap-px mt-0.5">
                        {#each [10, 9, 8, 7, 6, 5, 4, 3, 2, 1] as n}
                            <span
                                class="flex-1 text-center text-[10px] text-sub"
                                >{n}</span
                            >
                        {/each}
                    </div>
                    <div
                        class="text-xs text-sub text-center mt-2 min-[450px]:hidden md:block"
                    >
                        标准差：{stddev}
                    </div>
                </div>
            {/if}
        </div>
    </div>
</div>
