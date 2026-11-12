import { writable } from "svelte/store";

export const API_BASE = window.BACKEND_URL || "/api";

export const trackers = writable([]);
export const trackersLoaded = writable(false);
export const fetchIds = writable("");
export const fetchSource = writable("ids");
export const fetchJson = writable("overwrite");
export const fetchImages = writable("overwrite");
export const fetchTrackerIds = writable({});

export const bundleSource = writable("ids");
export const bundleIds = writable("");
export const bundleTrackerIds = writable({});
export const bundleTrackerText = writable("");
export const bundleDescription = writable("");
export const bundleExportStatus = writable("");
export const bundleImportStatus = writable("");
export const bundleTaskId = writable("");

export const taskState = writable({
    visible: false,
    id: "",
    label: "",
    phase: "",
    detail: "",
    percent: 0,
    color: "var(--c-toast-progress)"
});

let trackerLoading = false;
let trackerLoaded = false;

export function trackerLabel(t) {
    return t.name && t.name !== t.id ? t.name : t.id;
}

export async function loadTrackers(force) {
    if (trackerLoading) return;
    if (trackerLoaded && !force) return;
    trackerLoading = true;
    try {
        const res = await fetch(API_BASE + "/v0/tracker");
        const d = await window.readJSON(res);
        if (!res.ok || !d) {
            trackers.set([]);
            return;
        }
        trackers.set(Array.isArray(d.data) ? d.data : []);
        trackersLoaded.set(true);
        trackerLoaded = true;
    } catch (_) {
        trackers.set([]);
    } finally {
        trackerLoading = false;
    }
}

export function splitList(value) {
    return String(value || "")
        .split(",")
        .map(function (s) {
            return s.trim();
        })
        .filter(Boolean);
}

export function splitIDs(value) {
    return splitList(value)
        .map(function (s) {
            return parseInt(s, 10);
        })
        .filter(function (n) {
            return n > 0;
        });
}
