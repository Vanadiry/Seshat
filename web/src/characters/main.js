import { mount } from "svelte";
import EntityListPage from "../lists/EntityListPage.svelte";

mount(EntityListPage, {
    target: document.getElementById("view"),
    props: { kind: "character", unit: "个角色" }
});
