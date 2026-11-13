<script>
    let { eps = [] } = $props();

    let pinned = $state(null);
    let hovered = $state(null);

    let cells = [];
    let tipX = $state(0);
    let tipW = $state(260);

    function isOpen(index) {
        return pinned === index || (pinned === null && hovered === index);
    }

    function toggle(index) {
        pinned = pinned === index ? null : index;
        hovered = null;
    }

    // 提示框宽度受限，并水平夹在视口内，避免窄屏左右超出
    function place(index, target) {
        var el = target || cells[index];
        if (!el) return;
        var r = el.getBoundingClientRect();
        var vw = document.documentElement.clientWidth;
        var w = Math.min(260, vw - 32);
        var left = r.left + r.width / 2 - w / 2;
        left = Math.max(16, Math.min(left, vw - w - 16));
        tipW = w;
        tipX = left - r.left;
    }

    let active = $derived(pinned !== null ? pinned : hovered);

    $effect(() => {
        if (active === null) return;
        place(active);
    });

    $effect(() => {
        if (active === null) return;
        var onResize = function () {
            place(active);
        };
        window.addEventListener("resize", onResize);
        return function () {
            window.removeEventListener("resize", onResize);
        };
    });

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
        {#each rows as row, index}
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
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <div
                    class="relative ep-cell"
                    bind:this={cells[index]}
                    onmouseenter={(e) => {
                        place(index, e.currentTarget);
                        hovered = index;
                    }}
                    onmouseleave={() => {
                        if (hovered === index) hovered = null;
                    }}
                >
                    <span
                        tabindex="0"
                        role="button"
                        class="ep-num w-8 h-8 flex items-center justify-center rounded text-xs font-semibold cursor-pointer {row.cls} {pinned ===
                        index
                            ? 'ring-1 ring-pink'
                            : ''}"
                        onclick={(e) => {
                            e.stopPropagation();
                            place(index);
                            toggle(index);
                        }}
                        onkeydown={(e) => {
                            if (e.key === "Enter" || e.key === " ") {
                                e.preventDefault();
                                place(index);
                                toggle(index);
                            }
                        }}>{row.num}</span
                    >
                    <div
                        class="absolute bottom-full mb-1.5 ep-tip z-50 {isOpen(
                            index
                        )
                            ? ''
                            : 'hidden'}"
                        aria-hidden={!isOpen(index)}
                        style="left:{tipX}px;width:{tipW}px"
                    >
                        <div
                            class="select-text bg-surface-raised border border-bord-strong rounded-lg px-3 py-2 text-xs shadow-lg relative"
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
                            {#if pinned === index}
                                <button
                                    class="ep-close"
                                    onclick={(e) => {
                                        e.stopPropagation();
                                        pinned = null;
                                    }}>×</button
                                >
                            {/if}
                        </div>
                    </div>
                </div>
            {/if}
        {/each}
    </div>
{/if}
