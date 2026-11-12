import { mount } from "svelte";
import TagHeader from "./TagHeader.svelte";
import TagContent from "./TagContent.svelte";
import { initTag } from "./state.js";

var headerTarget = document.getElementById("detail-header");
var viewTarget = document.getElementById("view");

if (headerTarget) {
    mount(TagHeader, { target: headerTarget });
}
if (viewTarget) {
    mount(TagContent, { target: viewTarget });
}

initTag();
