<script>
    import { trackers, trackerLabel } from "./store.js";

    let { selected = {}, onchange, emptyHint = "", status = "" } = $props();

    let sorted = $derived(
        $trackers.slice().sort(function (a, b) {
            if (a.type !== b.type) return a.type === "manual" ? -1 : 1;
            const an = trackerLabel(a);
            const bn = trackerLabel(b);
            if (an !== bn) return an.localeCompare(bn, "zh-Hans-CN");
            return a.id.localeCompare(b.id);
        })
    );

    let count = $derived(Object.keys(selected).length);
    let available = $derived(sorted.some((t) => t.count > 0));

    let statusText = $derived.by(() => {
        if (status) return status;
        if (!sorted.length) {
            return "暂无 Tracker，" + emptyHint;
        }
        if (!available) {
            return "暂无可用 Tracker，" + emptyHint;
        }
        return "已选择 " + count + " 个";
    });
</script>

<div class="text-xs text-sub mb-1">{statusText}</div>
<div class="thin-scroll flex flex-col gap-1.5 max-h-60 overflow-y-auto pr-1">
    {#each sorted as t}
        <label
            class="h-10 w-full flex items-center gap-1.5 md:gap-2 px-3 md:px-4 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm min-w-0 {t.count >
            0
                ? 'cursor-pointer hover:bg-active hover:text-text focus-within:bg-active focus-within:text-text'
                : 'opacity-50'}"
        >
            <input
                type="checkbox"
                value={t.id}
                checked={!!selected[t.id]}
                disabled={t.count <= 0}
                onchange={(e) => onchange(t.id, e.currentTarget.checked)}
                class="w-4 h-4 accent-pink shrink-0"
            />
            <span class="flex items-center gap-2 flex-1 min-w-0">
                <span class="truncate min-w-0">{trackerLabel(t)}</span>
                {#if t.name && t.name !== t.id}
                    <span class="text-xs text-sub font-mono shrink-0"
                        >{t.id}</span
                    >
                {/if}
            </span>
            <span class="ml-auto text-xs text-right shrink-0"
                >{t.count > 0 ? t.count + " 条" : "空"}</span
            >
            <span class="w-7 md:w-8 text-right text-xs shrink-0"
                >{t.type === "auto" ? "自动" : "手动"}</span
            >
        </label>
    {/each}
</div>
