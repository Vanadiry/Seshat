<script>
    import { onMount } from "svelte";
    import { get } from "svelte/store";
    import TrackerPicker from "./TrackerPicker.svelte";
    import {
        API_BASE,
        bundleSource,
        bundleIds,
        bundleTrackerIds,
        bundleTrackerText,
        bundleDescription,
        bundleExportStatus,
        bundleImportStatus,
        bundleTaskId,
        loadTrackers,
        splitIDs,
        splitList
    } from "./store.js";

    let visible = window.resolve("bundle") !== "hidden";

    let importing = $state(false);
    let trackerStatus = $state("");
    let fileInput = $state(null);

    let showTrackerPicker = $derived($bundleSource === "trackers");

    async function ensureTrackers() {
        trackerStatus = "加载中...";
        await loadTrackers(false);
        trackerStatus = "";
    }

    function setSource(value) {
        bundleSource.set(value);
        if (value === "trackers") ensureTrackers();
    }

    function onTracker(id, checked) {
        const next = { ...$bundleTrackerIds };
        if (checked) next[id] = true;
        else delete next[id];
        bundleTrackerIds.set(next);
    }

    let exporting = $state(false);
    let exportAbort = null;

    async function doExport(body) {
        bundleExportStatus.set("导出中...");
        exporting = true;
        exportAbort = new AbortController();
        try {
            const res = await fetch(API_BASE + "/v0/bundle/export", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(body),
                signal: exportAbort.signal
            });
            const d = await window.readJSON(res);
            if (!res.ok || !d) {
                window.showError(window.errText(d, res.status));
                bundleExportStatus.set("导出失败");
                return;
            }
            const lines = ["已导出 " + d.filename];
            if (d.counts) {
                lines.push(
                    "条目 " +
                        d.counts.subjects +
                        "，角色 " +
                        d.counts.characters +
                        "，人物 " +
                        d.counts.persons +
                        "，图片 " +
                        d.counts.images
                );
            }
            lines.push("路径：" + d.path);
            bundleExportStatus.set(lines.join("\n"));
            window.showSuccess("已导出 " + d.filename);
        } catch (e) {
            if (e && e.name === "AbortError") {
                bundleExportStatus.set("导出已取消");
                return;
            }
            window.showError("导出失败");
            bundleExportStatus.set("导出失败");
        } finally {
            exporting = false;
            exportAbort = null;
        }
    }

    function cancelExport() {
        if (exportAbort) exportAbort.abort();
    }

    function requestExport() {
        const source = $bundleSource;
        const body = {
            source: source,
            description: $bundleDescription.trim()
        };
        const lines = [
            "来源：" + (source === "ids" ? "指定 ID" : "指定 Tracker")
        ];
        if (source === "ids") {
            body.ids = splitIDs($bundleIds);
            if (!body.ids.length) {
                window.showError("请填写动画 ID");
                return;
            }
            lines.push("ID：" + body.ids.join("、"));
        } else {
            const trackers = [];
            const seen = {};
            const selected = Object.keys($bundleTrackerIds);
            for (let i = 0; i < selected.length; i++) {
                if (!seen[selected[i]]) {
                    seen[selected[i]] = true;
                    trackers.push(selected[i]);
                }
            }
            const manual = splitList($bundleTrackerText);
            for (let i = 0; i < manual.length; i++) {
                if (!seen[manual[i]]) {
                    seen[manual[i]] = true;
                    trackers.push(manual[i]);
                }
            }
            body.trackers = trackers;
            if (!trackers.length) {
                window.showError("请选择或填写 Tracker ID");
                return;
            }
            for (let i = 0; i < trackers.length; i++) {
                if (!/^[a-zA-Z0-9_-]+$/.test(trackers[i])) {
                    window.showError(window.MSG.errInvalidTrackerName);
                    return;
                }
            }
            lines.push("Tracker：" + trackers.join("、"));
        }
        if (body.description) lines.push("描述：" + body.description);
        lines.push("确定导出？");
        window.confirmAction(lines.join("\n"), function () {
            doExport(body);
        });
    }

    function uploadBundle(file) {
        return new Promise(function (resolve, reject) {
            const xhr = new XMLHttpRequest();
            xhr.open("POST", API_BASE + "/v0/bundle/import");
            xhr.setRequestHeader("Content-Type", "application/x-tar");
            xhr.upload.onprogress = function (e) {
                if (!e.lengthComputable || !e.total) return;
                bundleImportStatus.set(
                    "上传中 " + Math.round((e.loaded / e.total) * 100) + "%"
                );
            };
            xhr.onload = function () {
                let d = null;
                try {
                    d = JSON.parse(xhr.responseText);
                } catch (_) {
                    d = null;
                }
                resolve({
                    ok: xhr.status >= 200 && xhr.status < 300,
                    status: xhr.status,
                    data: d
                });
            };
            xhr.onerror = function () {
                reject(new Error("network"));
            };
            xhr.send(file);
        });
    }

    async function startImport(file) {
        importing = true;
        bundleImportStatus.set("上传中 0%");
        try {
            const res = await uploadBundle(file);
            if (!res.ok || !res.data) {
                window.showError(window.errText(res.data, res.status));
                bundleImportStatus.set("导入失败");
                importing = false;
                return;
            }
            if (res.data.task_id) {
                bundleTaskId.set(res.data.task_id);
                bundleImportStatus.set("导入中");
                window.startProgress(res.data.task_id);
            } else {
                bundleImportStatus.set("导入已开始");
                importing = false;
            }
        } catch (_) {
            window.showError("导入失败");
            bundleImportStatus.set("导入失败");
            importing = false;
        }
    }

    function chooseFile() {
        if (fileInput) fileInput.click();
    }

    function onFileChange(e) {
        const input = e.currentTarget;
        const file = input.files && input.files[0];
        if (!file) return;
        if (!/\.tar$/i.test(file.name)) {
            window.showError("请选择 .tar 文件");
            input.value = "";
            return;
        }
        if (file.size > 1024 * 1024 * 1024) {
            window.showError("数据包不能超过 1 GiB");
            input.value = "";
            return;
        }
        bundleImportStatus.set(
            file.name + " · " + Math.round(file.size / 1024) + " KB"
        );
        window.confirmAction(
            "将导入 " + file.name + "，覆盖同名文件并重建索引。",
            function () {
                startImport(file);
            }
        );
    }

    onMount(function () {
        function onProgress(e) {
            const taskId = get(bundleTaskId);
            if (!taskId || e.detail.id !== taskId) return;
            const d = e.detail.data || {};
            if (
                d.step === "complete" ||
                d.step === "done" ||
                d.step === "error" ||
                d.step === "cancelled"
            ) {
                bundleImportStatus.set(
                    d.step === "error"
                        ? "导入失败"
                        : d.step === "cancelled"
                          ? "导入已终止"
                          : "导入完成"
                );
                bundleTaskId.set("");
                importing = false;
            }
        }
        function onError(e) {
            const taskId = get(bundleTaskId);
            if (!taskId || e.detail.id !== taskId) return;
            bundleImportStatus.set("导入失败");
            bundleTaskId.set("");
            importing = false;
        }
        window.addEventListener("seshat:task-progress", onProgress);
        window.addEventListener("seshat:task-error", onError);
        return function () {
            window.removeEventListener("seshat:task-progress", onProgress);
            window.removeEventListener("seshat:task-error", onError);
        };
    });
</script>

{#if visible}
    <section class="mb-8">
        <div class="flex items-center gap-2 mb-3">
            <span class="section-bar"></span>
            <h2 class="font-bold text-sm">数据包</h2>
        </div>
        <p class="text-xs text-sub mb-3">导出本地数据包，或导入 tar 数据包。</p>

        <div class="text-xs text-sub mb-2">导出</div>
        <div class="flex flex-col gap-2 mb-2">
            <label class="flex items-center gap-2 cursor-pointer">
                <input
                    type="radio"
                    name="bundle-source"
                    value="ids"
                    checked={$bundleSource === "ids"}
                    onchange={() => setSource("ids")}
                    class="radio-pink"
                />
                指定 ID
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
                <input
                    type="radio"
                    name="bundle-source"
                    value="trackers"
                    checked={$bundleSource === "trackers"}
                    onchange={() => setSource("trackers")}
                    class="radio-pink"
                />
                指定 Tracker
            </label>
        </div>

        {#if showTrackerPicker}
            <div class="mb-2">
                <TrackerPicker
                    selected={$bundleTrackerIds}
                    onchange={onTracker}
                    emptyHint="可以直接填写 Tracker ID"
                    status={trackerStatus}
                />
            </div>
        {/if}

        <div class="flex flex-col gap-2 mb-2">
            {#if $bundleSource === "ids"}
                <input
                    value={$bundleIds}
                    oninput={(e) => bundleIds.set(e.currentTarget.value)}
                    placeholder="动画 ID，逗号分隔"
                    class="h-10 w-full px-3 py-2 rounded-lg border border-bord bg-input text-sm"
                />
            {:else}
                <input
                    value={$bundleTrackerText}
                    oninput={(e) =>
                        bundleTrackerText.set(e.currentTarget.value)}
                    placeholder="也可以填写 Tracker ID，逗号分隔"
                    class="h-10 w-full px-3 py-2 rounded-lg border border-bord bg-input text-sm"
                />
            {/if}
            <input
                value={$bundleDescription}
                oninput={(e) => bundleDescription.set(e.currentTarget.value)}
                placeholder="描述，可选"
                class="h-10 w-full px-3 py-2 rounded-lg border border-bord bg-input text-sm"
            />
            <div class="flex gap-2">
                <button
                    type="button"
                    onclick={requestExport}
                    disabled={exporting}
                    class="h-10 flex-1 px-4 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text disabled:opacity-50 disabled:cursor-default"
                >
                    导出数据包
                </button>
                {#if exporting}
                    <button
                        type="button"
                        onclick={cancelExport}
                        class="h-10 shrink-0 px-4 py-2 rounded-lg border border-[#dc2626] bg-transparent text-[#dc2626] text-sm cursor-pointer hover:bg-pink-bg"
                    >
                        取消
                    </button>
                {/if}
            </div>
            <div class="text-xs text-sub whitespace-pre-line break-all">
                {$bundleExportStatus}
            </div>
        </div>

        <div class="text-xs text-sub mb-2">导入</div>
        <input
            bind:this={fileInput}
            type="file"
            accept=".tar,application/x-tar"
            class="hidden"
            onchange={onFileChange}
        />
        <button
            type="button"
            onclick={chooseFile}
            disabled={importing}
            class="h-10 w-full px-4 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text"
        >
            导入数据包
        </button>
        <div class="text-xs text-sub mt-2">{$bundleImportStatus}</div>
    </section>
{/if}
