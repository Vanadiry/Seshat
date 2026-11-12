import { mount } from "svelte";
import SubjectHeader from "./SubjectHeader.svelte";
import SubjectContent from "./SubjectContent.svelte";
import { initSubject } from "./state.js";

var headerTarget = document.getElementById("subject-header");
var viewTarget = document.getElementById("view");

if (headerTarget) {
    mount(SubjectHeader, { target: headerTarget });
}
if (viewTarget) {
    mount(SubjectContent, { target: viewTarget });
}

initSubject();
