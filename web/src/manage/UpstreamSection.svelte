<script>
    import { get } from "svelte/store";
    import { API_BASE, fetchIds, fetchSource, splitIDs } from "./store.js";

    let visible = window.resolve("upstream") !== "hidden";

    let keyword = $state("");
    let items = $state([]);
    let total = $state(0);
    let loading = $state(false);
    let status = $state("");
    let inputEl = $state(null);
    let flashId = $state(null);
    let flashTimer = 0;
    let statusFlash = $state(false);
    let statusTimer = 0;

    async function search(append) {
        if (loading) return;
        const q = keyword.trim();
        if (!q) {
            window.showError("关键词为空");
            if (inputEl) inputEl.focus();
            return;
        }
        loading = true;
        status = "搜索中…";
        if (!append) {
            items = [];
            total = 0;
        }
        try {
            const res = await fetch(API_BASE + "/v0/upstream/search", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    keyword: q,
                    limit: 20,
                    offset: append ? items.length : 0
                })
            });
            const d = await window.readJSON(res);
            if (!res.ok || !d) {
                window.showError(window.errText(d, res.status));
                status = "搜索失败";
                return;
            }
            const page = Array.isArray(d.data) ? d.data : [];
            items = items.concat(page);
            total = typeof d.total === "number" ? d.total : items.length;
            if (append && !page.length) total = items.length;
            status =
                items.length === 0
                    ? "未找到结果"
                    : "已显示 " + items.length + "，共 " + total + " 条";
        } catch (_) {
            window.showError("搜索失败");
            status = "搜索失败";
        } finally {
            loading = false;
        }
    }

    function flash(id) {
        flashId = null;
        clearTimeout(flashTimer);
        requestAnimationFrame(function () {
            flashId = id;
            flashTimer = setTimeout(function () {
                flashId = null;
            }, 800);
        });
    }

    function flashStatus() {
        statusFlash = false;
        clearTimeout(statusTimer);
        requestAnimationFrame(function () {
            statusFlash = true;
            statusTimer = setTimeout(function () {
                statusFlash = false;
            }, 800);
        });
    }

    function addID(id) {
        const ids = splitIDs(get(fetchIds));
        const existed = ids.indexOf(id) !== -1;
        if (!existed) ids.push(id);
        fetchIds.set(ids.join(", "));
        fetchSource.set("ids");
        status = existed
            ? "#" + id + " 已在拉取列表中"
            : "已添加 #" + id + " 到拉取列表";
        flash(id);
        flashStatus();
    }

    function year(date) {
        return date ? String(date).slice(0, 4) : "";
    }
</script>

{#if visible}
    <section class="mb-8">
        <div class="flex items-center gap-2 mb-3">
            <span class="section-bar"></span>
            <h2 class="font-bold text-sm">上游 ID 搜索</h2>
        </div>
        <p class="text-xs text-sub mb-3">
            从 Bangumi 搜索动画，把 ID 添加到拉取列表。
        </p>
        <div class="flex gap-2 mb-2">
            <input
                bind:this={inputEl}
                bind:value={keyword}
                placeholder="关键词，例如 CLANNAD"
                class="h-10 flex-1 px-3 py-2 rounded-lg border border-bord bg-input text-sm"
                onkeydown={(e) => {
                    if (e.key === "Enter") {
                        e.preventDefault();
                        search(false);
                    }
                }}
            />
            <button
                type="button"
                onclick={() => search(false)}
                class="h-10 shrink-0 px-4 py-2 rounded-lg bg-pink-solid text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90"
            >
                搜索
            </button>
        </div>
        <div
            class="text-xs text-sub {statusFlash
                ? 'upstream-status-flash'
                : ''}"
        >
            {status}
        </div>
        <div
            class="thin-scroll flex flex-col gap-1.5 mt-2 max-h-80 overflow-y-auto pr-1"
        >
            {#each items as item}
                {@const primary =
                    window.primaryName(item.name, item.name_cn) ||
                    "ID " + item.id}
                {@const secondary = window.subName(item.name, item.name_cn)}
                {@const meta = ["ID " + item.id]
                    .concat(item.date ? [year(item.date)] : [])
                    .concat(item.platform ? [item.platform] : [])
                    .join("，")}
                <div
                    class="flex items-center gap-2 px-3 py-1.5 rounded-lg border border-bord {flashId ===
                    item.id
                        ? 'upstream-flash'
                        : ''}"
                >
                    <div class="flex-1 min-w-0">
                        <div class="flex items-baseline gap-1.5 min-w-0">
                            <span
                                class="text-sm leading-tight truncate min-w-0 max-w-[60%]"
                                >{primary}</span
                            >
                            {#if secondary}
                                <span
                                    class="text-xs leading-tight text-sub truncate min-w-0 flex-1"
                                    >{secondary}</span
                                >
                            {/if}
                        </div>
                        <div
                            class="text-[11px] leading-tight text-sub truncate mt-0.5"
                        >
                            {meta}
                        </div>
                    </div>
                    <button
                        type="button"
                        onclick={() => addID(item.id)}
                        class="h-9 w-9 md:h-8 md:w-8 shrink-0 rounded-lg border border-bord bg-transparent text-sub text-lg leading-none cursor-pointer hover:bg-active hover:text-text flex items-center justify-center"
                        title="添加到拉取列表"
                        aria-label="添加到拉取列表"
                    >
                        +
                    </button>
                </div>
            {/each}
            {#if items.length < total}
                <button
                    type="button"
                    onclick={() => search(true)}
                    class="h-8 w-full px-4 rounded-lg border border-bord bg-transparent text-sub text-xs cursor-pointer hover:bg-active hover:text-text"
                >
                    加载更多
                </button>
            {/if}
        </div>
    </section>
{/if}
