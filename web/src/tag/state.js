import { writable } from "svelte/store";

const initialState = {
    tag: "",
    items: [],
    loading: true,
    missing: false,
    blocked: false,
    error: ""
};

export const tagState = writable({ ...initialState });

function patch(next) {
    tagState.update((state) => ({ ...state, ...next }));
}

export async function initTag() {
    const tag = new URLSearchParams(location.search).get("tag") || "";
    if (!tag) {
        patch({ missing: true, loading: false });
        return;
    }
    try {
        if (!(await window.requireFeature("tags"))) {
            patch({ blocked: true, loading: false });
            return;
        }
        patch({ tag });
        document.title = tag + " - 标签";
        const res = await window.apiFor(
            "tags",
            "/v0/tags/" + encodeURIComponent(tag) + "/subjects"
        );
        const items = res && Array.isArray(res.data) ? res.data : [];
        patch({ items, loading: false });
    } catch (e) {
        patch({ error: e.message || "加载失败", loading: false });
    }
}
