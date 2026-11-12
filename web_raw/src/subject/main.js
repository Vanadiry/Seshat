import { mount, unmount } from "svelte";
import Overview from "./Overview.svelte";

let current = null;

window.addEventListener("seshat:subject-overview", function (e) {
    var target = document.getElementById("subject-overview");
    if (!target) return;
    if (current) unmount(current);
    window._epPinned = null;
    current = mount(Overview, {
        target: target,
        props: e.detail
    });
    if (typeof window.initEpTooltips === "function") {
        window.initEpTooltips();
    }
});
