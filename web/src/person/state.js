import { writable } from "svelte/store";

const initialState = {
    id: "",
    person: null,
    cn: "",
    infoItems: [],
    characters: [],
    subjects: [],
    loading: true,
    missing: false,
    notFound: false,
    error: ""
};

export const personState = writable({ ...initialState });

function patch(next) {
    personState.update((state) => ({ ...state, ...next }));
}

async function loadList(url) {
    try {
        const data = await window.api(url);
        return Array.isArray(data) ? data : [];
    } catch (_) {
        return [];
    }
}

export async function initPerson() {
    const id = new URLSearchParams(location.search).get("id") || "";
    if (!id) {
        patch({ missing: true, loading: false });
        return;
    }
    patch({ id });
    try {
        const person = await window.api("/v0/persons/" + id);
        if (!person) {
            patch({ notFound: true, loading: false });
            return;
        }
        await window.loadPersonMap();
        const cn = window.extractCN(person);
        const infoItems = window.infoboxData(person);
        patch({ person, cn, infoItems });
        const [characters, subjects] = await Promise.all([
            loadList("/v0/persons/" + id + "/characters"),
            loadList("/v0/persons/" + id + "/subjects")
        ]);
        patch({ characters, subjects, loading: false });
    } catch (e) {
        patch({ error: e.message || "加载失败", loading: false });
    }
}
