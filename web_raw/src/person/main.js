import { mount, unmount } from "svelte";
import PersonHeader from "./PersonHeader.svelte";
import PersonDetail from "./PersonDetail.svelte";
import CreditList from "./CreditList.svelte";

let currentHeader = null;
let currentDetail = null;

window.addEventListener("seshat:person-header", function (e) {
    var target = document.getElementById("person-header");
    if (!target) return;
    if (currentHeader) unmount(currentHeader);
    currentHeader = mount(PersonHeader, {
        target: target,
        props: e.detail
    });
});

window.addEventListener("seshat:person-detail", function (e) {
    var target = document.getElementById("person-detail");
    if (!target) return;
    if (currentDetail) unmount(currentDetail);
    currentDetail = mount(PersonDetail, {
        target: target,
        props: e.detail
    });
});

let currentCharacters = null;
let currentSubjects = null;

window.addEventListener("seshat:person-characters", function (e) {
    var target = document.getElementById("person-characters");
    if (!target) return;
    if (currentCharacters) unmount(currentCharacters);
    currentCharacters = mount(CreditList, {
        target: target,
        props: e.detail
    });
});

window.addEventListener("seshat:person-subjects", function (e) {
    var target = document.getElementById("person-subjects");
    if (!target) return;
    if (currentSubjects) unmount(currentSubjects);
    currentSubjects = mount(CreditList, {
        target: target,
        props: e.detail
    });
});
