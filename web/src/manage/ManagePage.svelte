<script>
    import { onMount } from "svelte";
    import FetchSection from "./FetchSection.svelte";
    import UpstreamSection from "./UpstreamSection.svelte";
    import UserSection from "./UserSection.svelte";
    import BundleSection from "./BundleSection.svelte";
    import DangerSection from "./DangerSection.svelte";
    import TaskOverlay from "./TaskOverlay.svelte";
    import { taskState, loadTrackers } from "./store.js";

    let activeId = "";
    let hideTimer = 0;
    let capsReady = $state(false);

    function setState(next) {
        taskState.update((state) => ({ ...state, ...next }));
    }

    function showTask(id, label) {
        if (!id) return;
        clearTimeout(hideTimer);
        const same = activeId === id;
        activeId = id;
        setState({
            visible: true,
            id: id,
            label: label || "任务进行中",
            ...(same
                ? {}
                : {
                      phase: "",
                      detail: "",
                      percent: 0,
                      color: "var(--c-toast-progress)"
                  })
        });
    }

    function hideTask() {
        clearTimeout(hideTimer);
        activeId = "";
        setState({ visible: false, id: "" });
    }

    function updateTask(detail) {
        if (!activeId || detail.id !== activeId) return;
        const d = detail.data || {};
        if (d.label) setState({ label: d.label });

        if (d.step === "complete" || d.step === "done") {
            setState({
                phase: "完成",
                detail: "",
                percent: 100,
                color: "#22c55e"
            });
            const id = activeId;
            hideTimer = setTimeout(function () {
                if (activeId === id) hideTask();
            }, 800);
            return;
        }
        if (d.step === "error") {
            setState({
                phase: "任务失败",
                detail: d.status || "",
                percent: 100,
                color: "var(--c-toast-error)"
            });
            const id = activeId;
            hideTimer = setTimeout(function () {
                if (activeId === id) hideTask();
            }, 2500);
            return;
        }
        if (d.step === "cancelled") {
            setState({
                phase: "任务已终止",
                detail: "",
                percent: 100,
                color: "var(--c-toast-warning)"
            });
            const id = activeId;
            hideTimer = setTimeout(function () {
                if (activeId === id) hideTask();
            }, 1200);
            return;
        }
        if (d.step === "start") return;

        let phaseText = d.phase_name || "";
        if (d.phase && d.phases) {
            phaseText += (phaseText ? " " : "") + d.phase + "/" + d.phases;
        }
        setState({ phase: phaseText });

        if (typeof d.percent === "number") {
            const percent = Math.max(0, Math.min(100, d.percent));
            const parts = [];
            if (typeof d.done === "number" && d.total) {
                let count = d.done + "/" + d.total;
                if (d.speed) count += "，" + d.speed;
                parts.push(count);
            }
            if (d.status) parts.push(d.status);
            parts.push(d.percent + "%");
            setState({ percent: percent, detail: parts.join("，") });
        } else if (typeof d.done === "number" && d.total) {
            const parts = [d.done + "/" + d.total];
            if (d.speed) parts.push(d.speed);
            setState({
                percent: Math.round((d.done / d.total) * 100),
                detail: parts.join("，")
            });
        } else if (d.status) {
            setState({ detail: d.status });
        }
    }

    $effect(() => {
        capsReady;
        const content = document.getElementById("manage-content");
        if ($taskState.visible) {
            document.body.style.overflow = "hidden";
            if (content) content.inert = true;
        } else {
            document.body.style.overflow = "";
            if (content) content.inert = false;
        }
        return function () {
            document.body.style.overflow = "";
            if (content) content.inert = false;
        };
    });

    onMount(function () {
        function onStart(e) {
            showTask(e.detail.id, e.detail.label);
        }
        function onProgress(e) {
            updateTask(e.detail);
        }
        function onError(e) {
            if (!activeId || e.detail.id !== activeId) return;
            setState({
                phase: "进度连接中断",
                detail: "请刷新页面重试"
            });
        }
        function onTrackers() {
            loadTrackers(true);
        }

        window.addEventListener("seshat:task-start", onStart);
        window.addEventListener("seshat:task-progress", onProgress);
        window.addEventListener("seshat:task-error", onError);
        window.addEventListener("seshat:trackers-changed", onTrackers);

        (async function () {
            await window.capabilitiesReady();
            capsReady = true;
            const d = await window.apiFor("task", "/v0/tasks");
            if (!d || !d.data || !d.data.length) return;
            const task = d.data[0];
            showTask(task.id, task.label);
            updateTask({
                id: task.id,
                data: {
                    step: "snapshot",
                    label: task.label,
                    done: task.done,
                    total: task.total,
                    phase: task.phase,
                    phases: task.phases,
                    phase_name: task.phase_name
                }
            });
        })();

        return function () {
            window.removeEventListener("seshat:task-start", onStart);
            window.removeEventListener("seshat:task-progress", onProgress);
            window.removeEventListener("seshat:task-error", onError);
            window.removeEventListener("seshat:trackers-changed", onTrackers);
            clearTimeout(hideTimer);
        };
    });
</script>

{#if capsReady}
    <div id="manage-content" class="fade-in flex flex-col md:flex-row gap-8">
        <div class="flex-1 min-w-0">
            <FetchSection />
        </div>
        <div class="hidden md:block w-px bg-bord-light shrink-0"></div>
        <div class="flex-1 min-w-0">
            <UpstreamSection />
            <UserSection />
            <BundleSection />
            <DangerSection />
        </div>
    </div>
{/if}

<TaskOverlay />
