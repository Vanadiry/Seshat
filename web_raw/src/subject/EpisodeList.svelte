<script>
    let { eps = [] } = $props();

    let main = $derived(
        eps
            .filter((e) => (e.type || 0) === 0)
            .sort((a, b) => (a.sort || 0) - (b.sort || 0))
    );
    let sp = $derived(
        eps
            .filter((e) => (e.type || 0) !== 0)
            .sort((a, b) => {
                var ta = a.type || 0,
                    tb = b.type || 0;
                if (ta !== tb) return ta - tb;
                return (a.sort || 0) - (b.sort || 0);
            })
    );

    function epLabel(e) {
        if ((e.type || 0) === 0) {
            return String(Math.floor(e.sort || e.ep || 0)).padStart(2, "0");
        }
        var types = (window.MSG && window.MSG.epTypes) || {};
        return types[e.type] || "";
    }

    function epDesc(e) {
        if (!e.desc) return "";
        return window
            .linkifyPerson(e.desc)
            .replace(/\r\n|\n|\\n|\\r\\n/g, "<br>");
    }

    function epPairs(list) {
        var out = [];
        for (var i = 0; i < list.length; i += 2) {
            out.push([list[i], list[i + 1] || null]);
        }
        return out;
    }
</script>

{#snippet card(e)}
    <div class="border-t border-bord-light py-3">
        <div class="flex items-baseline gap-2">
            <span class="text-pink text-sm font-bold w-8 shrink-0"
                >{epLabel(e)}</span
            >
            <span class="font-semibold text-[14px]"
                >{window.primaryName(e.name || "", e.name_cn || "")}</span
            >
            {#if window.subName(e.name || "", e.name_cn || "")}
                <span class="text-xs text-sub"
                    >{window.subName(e.name || "", e.name_cn || "")}</span
                >
            {/if}
        </div>
        <div class="text-xs text-sub mt-1 ml-10">
            {e.airdate ? "首播: " + e.airdate : ""}{e.airdate && e.duration
                ? " ｜ "
                : ""}{e.duration ? "时长: " + e.duration : ""}
        </div>
        {#if e.desc}
            <div class="text-[13px] text-sub mt-1 ml-10 leading-relaxed">
                {@html epDesc(e)}
            </div>
        {/if}
    </div>
{/snippet}

{#if main.length}
    <div class="mb-6">
        <div class="text-sm text-sub font-semibold mb-3">
            本篇 ({main.length})
        </div>
        {#each epPairs(main) as pair}
            <div class="flex gap-4">
                <div class="flex-1 min-w-0">{@render card(pair[0])}</div>
                <div class="flex-1 min-w-0">
                    {#if pair[1]}{@render card(pair[1])}{/if}
                </div>
            </div>
        {/each}
    </div>
{/if}

{#if sp.length}
    <div class="mb-6">
        <div class="text-sm text-sub font-semibold mb-3">
            SP / 其他 ({sp.length})
        </div>
        {#each epPairs(sp) as pair}
            <div class="flex gap-4">
                <div class="flex-1 min-w-0">{@render card(pair[0])}</div>
                <div class="flex-1 min-w-0">
                    {#if pair[1]}{@render card(pair[1])}{/if}
                </div>
            </div>
        {/each}
    </div>
{/if}
