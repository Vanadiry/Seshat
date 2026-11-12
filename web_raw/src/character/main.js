import { mount } from "svelte";
import CharacterHeader from "./CharacterHeader.svelte";
import CharacterContent from "./CharacterContent.svelte";
import { initCharacter } from "./state.js";

var headerTarget = document.getElementById("detail-header");
var viewTarget = document.getElementById("view");

if (headerTarget) {
    mount(CharacterHeader, { target: headerTarget });
}
if (viewTarget) {
    mount(CharacterContent, { target: viewTarget });
}

initCharacter();
