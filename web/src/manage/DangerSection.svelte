<script>
    import DangerButton from "./DangerButton.svelte";

    let eloVisible = window.resolve("elo") !== "hidden";

    function rebuildIndex() {
        window.confirmAction(
            window.MSG.confirmRebuildIndex,
            window.doFetchIndex
        );
    }

    function rebuildElo() {
        window.confirmAction(
            "将清空当前 ELO 分数并从对战历史完整重建",
            window.doRebuildELO
        );
    }

    async function rebuildAll() {
        try {
            const d = await window.api("/v0/tracker");
            let msg = window.MSG.confirmRebuildAll().replace(/。$/, "");
            if (d && typeof d.total === "number") {
                msg += "，Tracker 数量：" + d.total;
            }
            msg += "，此操作不可恢复";
            window.confirmAction(msg, window.doDeepRebuild);
        } catch (_) {
            window.confirmAction(
                window.MSG.confirmRebuildAll().replace(/。$/, "") +
                    "，此操作不可恢复",
                window.doDeepRebuild
            );
        }
    }
</script>

<section class="mb-8">
    <div class="flex items-center gap-2 mb-3">
        <span class="w-[3px] h-[18px] bg-[#dc2626] rounded-sm"></span>
        <h2 class="font-bold text-sm text-[#dc2626]">危险区</h2>
    </div>
    <p class="text-xs text-sub mb-3">这些操作会影响大量数据，请先确认。</p>
    <div class="flex flex-col gap-2">
        <DangerButton label="重建索引" onConfirm={rebuildIndex} />
        {#if eloVisible}
            <DangerButton label="重建 ELO" onConfirm={rebuildElo} />
        {/if}
        <DangerButton label="重建全部" onConfirm={rebuildAll} />
    </div>
</section>
