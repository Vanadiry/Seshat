import { writable } from "svelte/store";

const initialState = {
    id: "",
    character: null,
    cn: "",
    infoItems: [],
    appearances: [],
    loading: true,
    missing: false,
    notFound: false,
    error: ""
};

export const characterState = writable({ ...initialState });

function patch(next) {
    characterState.update((state) => ({ ...state, ...next }));
}

async function loadAppearances(id) {
    try {
        const [subjectsRaw, personsRaw] = await Promise.all([
            window.api("/v0/characters/" + id + "/subjects"),
            window.api("/v0/characters/" + id + "/persons")
        ]);
        const subjects = Array.isArray(subjectsRaw) ? subjectsRaw : [];
        const persons = Array.isArray(personsRaw) ? personsRaw : [];
        const map = {};
        for (let i = 0; i < subjects.length; i++) {
            const s = subjects[i];
            map[s.id] = {
                id: s.id,
                name: s.name || "",
                name_cn: s.name_cn || "",
                staff: s.staff || "",
                type: s.type || 0,
                person_id: 0,
                person_name: ""
            };
        }
        for (let i = 0; i < persons.length; i++) {
            const p = persons[i];
            map[p.subject_id] = {
                id: p.subject_id,
                name: p.subject_name || "",
                name_cn: p.subject_name_cn || "",
                staff: p.staff || "",
                type: p.subject_type || 0,
                person_id: p.id || 0,
                person_name: p.name || ""
            };
        }
        const order = { 2: 0, 4: 1, 1: 2, 3: 3, 6: 4 };
        return Object.values(map).sort(function (a, b) {
            return (
                (order[a.type] != null ? order[a.type] : 5) -
                (order[b.type] != null ? order[b.type] : 5)
            );
        });
    } catch (_) {
        return [];
    }
}

export async function initCharacter() {
    const id = new URLSearchParams(location.search).get("id") || "";
    if (!id) {
        patch({ missing: true, loading: false });
        return;
    }
    patch({ id });
    try {
        const character = await window.api("/v0/characters/" + id);
        if (!character) {
            patch({ notFound: true, loading: false });
            return;
        }
        await window.loadPersonMap();
        const cn = window.extractCN(character);
        const infoItems = window.infoboxData(character);
        patch({ character, cn, infoItems });
        const appearances = await loadAppearances(id);
        patch({ appearances, loading: false });
    } catch (e) {
        patch({ error: e.message || "加载失败", loading: false });
    }
}
