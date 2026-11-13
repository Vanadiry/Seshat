<script>
    import { onMount } from "svelte";
    import { get } from "svelte/store";
    import TrackerPicker from "./TrackerPicker.svelte";
    import {
        API_BASE,
        trackers,
        trackersLoaded,
        fetchIds,
        fetchSource,
        fetchJson,
        fetchImages,
        fetchTrackerIds,
        loadTrackers,
        splitIDs,
        splitList
    } from "./store.js";

    let visible =
        window.resolve("fetch") !== "hidden" &&
        window.resolve("task") !== "hidden";

    let manualTrackers = $state("");
    let trackerStatus = $state("");

    let showNames = $derived($fetchSource === "tracker_names");
    let showIds = $derived($fetchSource === "ids");

    async function ensureTrackers() {
        trackerStatus = "加载中...";
        await loadTrackers(false);
        trackerStatus = "";
    }

    function setSource(value) {
        fetchSource.set(value);
        if (value === "tracker_names") ensureTrackers();
    }

    function onTracker(id, checked) {
        const next = { ...$fetchTrackerIds };
        if (checked) next[id] = true;
        else delete next[id];
        fetchTrackerIds.set(next);
    }

    function buildBody() {
        const source = $fetchSource;
        const body = {
            source: source,
            json: $fetchJson,
            images: $fetchImages
        };
        if (source === "tracker_names") {
            const names = [];
            const seen = {};
            const selected = Object.keys($fetchTrackerIds);
            for (let i = 0; i < selected.length; i++) {
                if (!seen[selected[i]]) {
                    seen[selected[i]] = true;
                    names.push(selected[i]);
                }
            }
            const manual = splitList(manualTrackers);
            for (let i = 0; i < manual.length; i++) {
                if (!seen[manual[i]]) {
                    seen[manual[i]] = true;
                    names.push(manual[i]);
                }
            }
            if (!names.length) {
                window.showError("请选择或填写 Tracker");
                return null;
            }
            for (let i = 0; i < names.length; i++) {
                if (!/^[a-zA-Z0-9_-]+$/.test(names[i])) {
                    window.showError(window.MSG.errInvalidTrackerName);
                    return null;
                }
            }
            const index = {};
            for (const t of $trackers) index[t.id] = t;
            if ($trackersLoaded) {
                const missing = names.filter((n) => !index[n]);
                if (missing.length) {
                    window.showError("未找到 Tracker：" + missing.join("、"));
                    return null;
                }
                const empty = names.filter(
                    (n) => index[n] && index[n].count === 0
                );
                if (empty.length) {
                    window.showError("Tracker 为空：" + empty.join("、"));
                    return null;
                }
            }
            body.names = names;
        }
        if (source === "ids") {
            body.ids = splitIDs($fetchIds);
            if (!body.ids.length) {
                window.showError("请填写动画 ID");
                return null;
            }
        }
        return body;
    }

    function summary(body) {
        const lines = [
            "来源：" +
                {
                    tracker_new: "所有 Tracker 中的新增 ID",
                    tracker_all: "所有 Tracker",
                    tracker_names: "指定 Tracker",
                    ids: "指定 ID",
                    local: "本地存在的条目"
                }[body.source]
        ];
        if (body.names && body.names.length) {
            lines.push("Tracker：" + body.names.join("、"));
        }
        if (body.ids && body.ids.length) {
            lines.push("ID：" + body.ids.join("、"));
        }
        lines.push(
            "详情数据：" +
                (body.json === "fill" ? "补充缺失" : "全量下载并覆盖")
        );
        lines.push(
            "图片：" +
                {
                    none: "跳过",
                    fill: "补充缺失",
                    overwrite: "全量下载并覆盖"
                }[body.images]
        );
        lines.push("");
        lines.push("确定开始拉取？");
        return lines.join("\n");
    }

    async function startFetch(body) {
        const res = await fetch(API_BASE + "/v0/fetch", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(body)
        });
        const d = await window.readJSON(res);
        if (!res.ok || !d) {
            window.showError(window.errText(d, res.status));
            return;
        }
        if (d.task_id) window.startProgress(d.task_id);
        else window.showInfo("没有需要处理的条目");
    }

    function requestFetch() {
        const body = buildBody();
        if (!body) return;
        window.confirmAction(summary(body), function () {
            startFetch(body);
        });
    }

    async function addTrackers() {
        const names = splitList(manualTrackers);
        if (!names.length) {
            window.showError("请填写 Tracker ID");
            return;
        }
        for (let i = 0; i < names.length; i++) {
            if (!/^[a-zA-Z0-9_-]+$/.test(names[i])) {
                window.showError(window.MSG.errInvalidTrackerName);
                return;
            }
        }
        window.confirmAction(
            "将创建以下 Tracker：\n" + names.join("、") + "\n已存在的会跳过",
            async function () {
                let created = 0,
                    existed = 0,
                    failed = 0;
                for (let i = 0; i < names.length; i++) {
                    const r = await fetch(API_BASE + "/v0/tracker/create", {
                        method: "POST",
                        headers: { "Content-Type": "application/json" },
                        body: JSON.stringify({ id: names[i] })
                    });
                    if (r.ok) created++;
                    else if (r.status === 409) existed++;
                    else failed++;
                }
                if (failed > 0) {
                    window.showWarn(
                        "已添加 " +
                            created +
                            " 个，已存在 " +
                            existed +
                            " 个，失败 " +
                            failed +
                            " 个"
                    );
                } else {
                    window.showSuccess(
                        "已添加 " +
                            created +
                            " 个 Tracker" +
                            (existed > 0 ? "，已存在 " + existed + " 个" : "")
                    );
                }
                manualTrackers = "";
                await loadTrackers(true);
            }
        );
    }

    onMount(function () {
        if (showNames) ensureTrackers();
    });
</script>

{#if visible}
    <section class="mb-8 md:mb-0">
        <div class="flex items-center gap-2 mb-3">
            <span class="section-bar"></span>
            <h2 class="font-bold text-sm">拉取</h2>
        </div>

        <div class="text-xs text-sub mb-2">来源</div>
        <div class="flex flex-col gap-2 mb-3">
            <label class="flex items-center gap-2 cursor-pointer">
                <input
                    type="radio"
                    name="fetch-source"
                    value="tracker_new"
                    checked={$fetchSource === "tracker_new"}
                    onchange={() => setSource("tracker_new")}
                    class="radio-pink"
                />
                所有 Tracker 中的新增 ID
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
                <input
                    type="radio"
                    name="fetch-source"
                    value="tracker_all"
                    checked={$fetchSource === "tracker_all"}
                    onchange={() => setSource("tracker_all")}
                    class="radio-pink"
                />
                所有 Tracker
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
                <input
                    type="radio"
                    name="fetch-source"
                    value="tracker_names"
                    checked={$fetchSource === "tracker_names"}
                    onchange={() => setSource("tracker_names")}
                    class="radio-pink"
                />
                指定 Tracker
            </label>
            {#if showNames}
                <div class="ml-6 flex flex-col gap-2">
                    <TrackerPicker
                        selected={$fetchTrackerIds}
                        onchange={onTracker}
                        emptyHint="可以直接填写 id 或用加号创建"
                        status={trackerStatus}
                    />
                    <div class="flex items-stretch gap-2">
                        <input
                            value={manualTrackers}
                            oninput={(e) =>
                                (manualTrackers = e.currentTarget.value)}
                            placeholder="也可以填写 Tracker ID，逗号分隔"
                            class="h-10 flex-1 px-3 py-2 rounded-lg border border-bord bg-input text-sm"
                        />
                        <button
                            type="button"
                            onclick={addTrackers}
                            title="添加 Tracker"
                            aria-label="添加 Tracker"
                            class="h-10 w-10 shrink-0 rounded-lg border border-bord bg-transparent text-sub text-lg leading-none cursor-pointer hover:bg-active hover:text-text flex items-center justify-center"
                        >
                            +
                        </button>
                    </div>
                </div>
            {/if}
            <label class="flex items-center gap-2 cursor-pointer">
                <input
                    type="radio"
                    name="fetch-source"
                    value="ids"
                    checked={$fetchSource === "ids"}
                    onchange={() => setSource("ids")}
                    class="radio-pink"
                />
                指定 ID
            </label>
            {#if showIds}
                <div class="ml-6">
                    <input
                        value={$fetchIds}
                        oninput={(e) => fetchIds.set(e.currentTarget.value)}
                        placeholder="动画 ID，如 51 或 51, 288"
                        class="h-10 w-full px-3 py-2 rounded-lg border border-bord bg-input text-sm"
                    />
                </div>
            {/if}
            <label class="flex items-center gap-2 cursor-pointer">
                <input
                    type="radio"
                    name="fetch-source"
                    value="local"
                    checked={$fetchSource === "local"}
                    onchange={() => setSource("local")}
                    class="radio-pink"
                />
                本地存在的条目
            </label>
        </div>

        <div class="text-xs text-sub mb-2">详情数据</div>
        <div class="flex flex-col gap-2 mb-3">
            <label class="flex items-center gap-2 cursor-pointer">
                <input
                    type="radio"
                    name="fetch-json"
                    value="fill"
                    checked={$fetchJson === "fill"}
                    onchange={() => fetchJson.set("fill")}
                    class="radio-pink"
                />
                补充缺失
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
                <input
                    type="radio"
                    name="fetch-json"
                    value="overwrite"
                    checked={$fetchJson === "overwrite"}
                    onchange={() => fetchJson.set("overwrite")}
                    class="radio-pink"
                />
                全量下载并覆盖
            </label>
        </div>

        <div class="text-xs text-sub mb-2">图片</div>
        <div class="flex flex-col gap-2 mb-3">
            <label class="flex items-center gap-2 cursor-pointer">
                <input
                    type="radio"
                    name="fetch-images"
                    value="none"
                    checked={$fetchImages === "none"}
                    onchange={() => fetchImages.set("none")}
                    class="radio-pink"
                />
                跳过
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
                <input
                    type="radio"
                    name="fetch-images"
                    value="fill"
                    checked={$fetchImages === "fill"}
                    onchange={() => fetchImages.set("fill")}
                    class="radio-pink"
                />
                补充缺失
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
                <input
                    type="radio"
                    name="fetch-images"
                    value="overwrite"
                    checked={$fetchImages === "overwrite"}
                    onchange={() => fetchImages.set("overwrite")}
                    class="radio-pink"
                />
                全量下载并覆盖
            </label>
        </div>

        <button
            type="button"
            onclick={requestFetch}
            class="h-10 w-full px-4 py-2 rounded-lg bg-pink-solid text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90"
        >
            开始拉取
        </button>
    </section>
{/if}
