import { mount, unmount } from "svelte";
import CharacterHeader from "./CharacterHeader.svelte";
import CharacterDetail from "./CharacterDetail.svelte";
import AppearanceList from "./AppearanceList.svelte";

let currentHeader = null;
let currentDetail = null;

window.addEventListener("seshat:character-header", function (e) {
    var target = document.getElementById("character-header");
    if (!target) return;
    if (currentHeader) unmount(currentHeader);
    currentHeader = mount(CharacterHeader, {
        target: target,
        props: e.detail
    });
});

window.addEventListener("seshat:character-detail", function (e) {
    var target = document.getElementById("character-detail");
    if (!target) return;
    if (currentDetail) unmount(currentDetail);
    currentDetail = mount(CharacterDetail, {
        target: target,
        props: e.detail
    });
});

let currentAppearances = null;

window.addEventListener("seshat:character-appearances", function (e) {
    var target = document.getElementById("character-appearances");
    if (!target) return;
    if (currentAppearances) unmount(currentAppearances);
    currentAppearances = mount(AppearanceList, {
        target: target,
        props: e.detail
    });
});
