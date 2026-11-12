import { mount } from "svelte";
import PersonHeader from "./PersonHeader.svelte";
import PersonContent from "./PersonContent.svelte";
import { initPerson } from "./state.js";

var headerTarget = document.getElementById("detail-header");
var viewTarget = document.getElementById("view");

if (headerTarget) {
    mount(PersonHeader, { target: headerTarget });
}
if (viewTarget) {
    mount(PersonContent, { target: viewTarget });
}

initPerson();
