<script>
    import { onDestroy, onMount } from "svelte";
    import { registerLock, resetOtherLocks } from "./dangerLocks.js";

    let { label = "", onConfirm } = $props();

    let mode = $state("idle");
    let fill = $state(0);
    let highlight = $state(false);

    let armTimer = 0;
    let highlightTimer = 0;
    let completeTimer = 0;
    let raf = 0;
    let suppressClick = false;

    let labelText = $derived(mode === "idle" ? label : "按住来应用此项");

    function cancelTimers() {
        cancelAnimationFrame(raf);
        clearTimeout(armTimer);
        clearTimeout(highlightTimer);
        clearTimeout(completeTimer);
    }

    function reset() {
        cancelTimers();
        mode = "idle";
        fill = 0;
        highlight = false;
    }

    function arm() {
        if (mode !== "idle") return;
        resetOtherLocks(reset);
        mode = "armed";
        highlight = true;
        highlightTimer = setTimeout(function () {
            highlight = false;
        }, 900);
        armTimer = setTimeout(function () {
            if (mode === "armed") reset();
        }, 5000);
    }

    function cancelHold() {
        if (mode !== "holding") return;
        cancelAnimationFrame(raf);
        fill = 0;
        mode = "armed";
        highlight = true;
        clearTimeout(highlightTimer);
        highlightTimer = setTimeout(function () {
            highlight = false;
        }, 900);
        clearTimeout(armTimer);
        armTimer = setTimeout(function () {
            if (mode === "armed") reset();
        }, 5000);
    }

    function startHold(e) {
        if (mode !== "idle" && mode !== "armed") return;
        if (e) e.preventDefault();
        if (mode === "idle") resetOtherLocks(reset);
        mode = "holding";
        clearTimeout(armTimer);
        clearTimeout(highlightTimer);
        highlight = false;
        const start = performance.now();
        fill = 0;

        function frame(now) {
            if (mode !== "holding") return;
            const pct = Math.min(100, ((now - start) / 1500) * 100);
            fill = pct;
            if (pct >= 100) {
                mode = "idle";
                suppressClick = true;
                fill = 100;
                clearTimeout(armTimer);
                clearTimeout(highlightTimer);
                completeTimer = setTimeout(function () {
                    reset();
                    if (onConfirm) onConfirm();
                    setTimeout(function () {
                        suppressClick = false;
                    }, 500);
                }, 120);
                return;
            }
            raf = requestAnimationFrame(frame);
        }
        raf = requestAnimationFrame(frame);
    }

    let unregister = function () {};

    onMount(function () {
        unregister = registerLock(reset);
    });

    onDestroy(function () {
        unregister();
        cancelTimers();
    });
</script>

<button
    type="button"
    class="h-10 danger-lock w-full px-4 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text select-none"
    onclick={() => {
        if (suppressClick) {
            suppressClick = false;
            return;
        }
        if (mode === "idle") arm();
    }}
    onpointerdown={startHold}
    onpointerup={cancelHold}
    onpointercancel={cancelHold}
    onpointerleave={() => {
        if (mode === "holding") cancelHold();
    }}
    onkeydown={(e) => {
        if (e.key !== " " && e.key !== "Enter") return;
        if (mode === "idle" || mode === "armed") {
            e.preventDefault();
            startHold(e);
        }
    }}
    onkeyup={(e) => {
        if (e.key !== " " && e.key !== "Enter") return;
        if (mode === "holding") {
            e.preventDefault();
            cancelHold();
        }
    }}
    onblur={() => {
        if (mode === "armed" || mode === "holding") reset();
    }}
>
    <span class="lock-fill" style="width:{fill}%"></span>
    <span class="lock-label {highlight ? 'text-pink' : ''}">{labelText}</span>
</button>
