import { mount, unmount } from "svelte";
import { headerState } from "./store.js";
import SubjectHeader from "./SubjectHeader.svelte";
import Overview from "./Overview.svelte";
import EpisodeList from "./EpisodeList.svelte";
import CharacterList from "./CharacterList.svelte";
import PersonList from "./PersonList.svelte";

var headerTarget = document.getElementById("subject-header");
if (headerTarget) {
    mount(SubjectHeader, { target: headerTarget });
}

window.addEventListener("seshat:subject-header", function (e) {
    headerState.set(e.detail);
});

var current = null;

function mountContent(Component, id, detail) {
    var target = document.getElementById(id);
    if (!target) return;
    if (current) unmount(current);
    window._epPinned = null;
    current = mount(Component, {
        target: target,
        props: detail || {}
    });
    if (typeof window.initEpTooltips === "function") {
        window.initEpTooltips();
    }
}

window.addEventListener("seshat:subject-overview", function (e) {
    mountContent(Overview, "subject-overview", e.detail);
});

window.addEventListener("seshat:subject-episodes", function (e) {
    mountContent(EpisodeList, "subject-episodes", e.detail);
});

window.addEventListener("seshat:subject-characters", function (e) {
    mountContent(CharacterList, "subject-characters", e.detail);
});

window.addEventListener("seshat:subject-persons", function (e) {
    mountContent(PersonList, "subject-persons", e.detail);
});
