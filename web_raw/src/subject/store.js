import { writable } from "svelte/store";

export const headerState = writable({
    subject: null,
    sid: "",
    tab: "overview",
    counts: {},
    showBundle: false
});
