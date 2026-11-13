<script>
    import { tick } from "svelte";
    import { taskState, API_BASE } from "./store.js";

    let state = $derived($taskState);
    let abortBtn = $state(null);

    $effect(() => {
        if (state.visible && abortBtn) {
            tick().then(() => abortBtn.focus());
        }
    });

    function abortTask() {
        window.confirmAction("会中止当前任务，已完成的数据保留。", function () {
            var id = state.id || "";
            var url =
                API_BASE +
                "/v0/task/cancel" +
                (id ? "/" + encodeURIComponent(id) : "");
            fetch(url, { method: "POST" })
                .then(function (res) {
                    if (!res.ok) window.showError("中止请求失败");
                })
                .catch(function () {
                    window.showError("中止请求失败");
                });
        });
    }
</script>

{#if state.visible}
    <div
        class="fixed inset-x-0 bottom-0 z-20 flex items-center justify-center bg-bg/40 backdrop-blur-xl p-6"
        style="top: var(--topbar-h)"
    >
        <div
            class="w-full max-w-[420px] flex flex-col items-center text-center"
        >
            <p class="text-lg font-bold mb-1">{state.label}</p>
            <div class="text-xs text-sub mb-3">{state.phase}</div>
            <div
                class="w-full h-2 rounded-full bg-[rgba(128,128,128,.2)] overflow-hidden mb-2"
            >
                <div
                    class="h-full rounded-full"
                    style="width: {state.percent}%; background: {state.color}; transition: width 0.2s linear"
                ></div>
            </div>
            <div class="text-xs text-sub mb-5">{state.detail}</div>
            <button
                bind:this={abortBtn}
                type="button"
                onclick={abortTask}
                class="h-10 px-5 py-2 rounded-lg border border-[#dc2626] bg-transparent text-[#dc2626] text-sm cursor-pointer hover:bg-pink-bg"
            >
                中止任务
            </button>
        </div>
    </div>
{/if}
