<script>
    let { eps = [] } = $props();

    const epCls = {
        0: "bg-surface-alt text-sub",
        1: "bg-tier-gold-bg text-tier-gold",
        2: "bg-tier-blue-bg text-tier-blue",
        3: "bg-tier-purple-bg text-tier-purple",
        4: "bg-tier-green-bg text-tier-green",
        5: "bg-pink-bg text-pink",
        6: "bg-surface-alt text-sub"
    };
    const sepCls = {
        1: "text-tier-gold",
        2: "text-tier-blue",
        3: "text-tier-purple",
        4: "text-tier-green",
        5: "text-pink",
        6: "text-sub"
    };

    function sepStyle(type) {
        var gap = "gap-1",
            text = "text-[9px]";
        if (type >= 1 && type <= 3) {
            gap = "gap-1.5";
            text = "text-[14px]";
        } else if (type === 4 || type === 6) {
            gap = "gap-1";
            text = "text-[12px]";
        } else if (type === 5) {
            gap = "gap-0.5";
            text = "text-[10px]";
        }
        return { gap: gap, text: text };
    }

    function epTypeLabel(type) {
        var types = (window.MSG && window.MSG.epTypes) || {};
        return types[type] || "";
    }

    let rows = $derived.by(() => {
        var sorted = eps.slice().sort(function (a, b) {
            var ta = a.type || 0,
                tb = b.type || 0;
            if (ta !== tb) return ta - tb;
            return (a.sort || 0) - (b.sort || 0);
        });
        var out = [];
        var lastType = -1;
        for (var i = 0; i < sorted.length; i++) {
            var e = sorted[i];
            var t = e.type || 0;
            if (t !== lastType && t !== 0) {
                out.push({
                    kind: "sep",
                    type: t,
                    label: epTypeLabel(t)
                });
            }
            lastType = t;
            var epNum = String(Math.floor(e.sort || e.ep || 0));
            out.push({
                kind: "ep",
                num: epNum.padStart(2, "0"),
                epNum: epNum,
                cls: epCls[t] || "bg-surface-alt text-sub",
                name: window.primaryName(e.name || "", e.name_cn || ""),
                sub: window.subName(e.name || "", e.name_cn || ""),
                typeLabel: epTypeLabel(t),
                airdate: e.airdate,
                duration: e.duration
            });
        }
        return out;
    });
</script>

{#if rows.length}
    <div class="flex flex-wrap gap-1.5">
        {#each rows as row}
            {#if row.kind === "sep"}
                {@const st = sepStyle(row.type)}
                <span
                    class="w-8 h-8 flex items-center pl-0 font-semibold cursor-default {st.gap} {st.text} {sepCls[
                        row.type
                    ] || 'text-sub'}"
                    ><span class="border-l border-current h-7"
                    ></span>{row.label}</span
                >
            {:else}
                <div class="relative ep-cell">
                    <span
                        tabindex="0"
                        role="button"
                        class="ep-num w-8 h-8 flex items-center justify-center rounded text-xs font-semibold cursor-pointer {row.cls}"
                        >{row.num}</span
                    >
                    <div
                        class="absolute bottom-full left-1/2 -translate-x-1/2 mb-1.5 ep-tip z-50"
                        style="display:none"
                    >
                        <div
                            class="select-text bg-surface-raised border border-bord-strong rounded-lg px-3 py-2 text-xs whitespace-nowrap shadow-lg relative"
                        >
                            <div class="text-sub">
                                第 {row.epNum} 话 · {row.typeLabel}
                            </div>
                            <div class="text-sm font-semibold mt-1">
                                {row.name}
                            </div>
                            {#if row.sub}
                                <div class="text-sub mt-0.5">{row.sub}</div>
                            {/if}
                            {#if row.airdate}
                                <div class="text-sub mt-1">
                                    首播: {row.airdate}
                                </div>
                            {/if}
                            {#if row.duration}
                                <div class="text-sub">时长: {row.duration}</div>
                            {/if}
                        </div>
                    </div>
                </div>
            {/if}
        {/each}
    </div>
{/if}
