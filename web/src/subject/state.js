import { writable, get } from "svelte/store";

const initialState = {
    sid: "",
    tab: "overview",
    subject: null,
    infoItems: [],
    myColl: null,
    eloInfo: null,
    eps: [],
    chars: [],
    persons: [],
    mainChars: [],
    rels: [],
    counts: { ep: 0, chars: 0, persons: 0 },
    showBundle: false,
    exporting: false,
    loading: true,
    error: "",
    notFound: false
};

export const subjectState = writable({ ...initialState });

const API_BASE = window.BACKEND_URL || "/api";

function patch(next) {
    subjectState.update((state) => ({ ...state, ...next }));
}

function list(value) {
    if (Array.isArray(value)) return value;
    if (value && Array.isArray(value.data)) return value.data;
    return [];
}

let collectionLoaded = false;
let collection = null;

async function loadMyCollection() {
    if (collectionLoaded) return collection;
    collectionLoaded = true;
    if (!window.USERNAME) return null;
    try {
        const d = await window.api(
            "/v0/users/" + window.USERNAME + "/collections?view=summary"
        );
        if (!d || !d.data) return null;
        for (let i = 0; i < d.data.length; i++) {
            if (
                String(d.data[i].subject_id) === String(get(subjectState).sid)
            ) {
                collection = {
                    type: d.data[i].type,
                    rate: d.data[i].rate
                };
                break;
            }
        }
    } catch (_) {}
    return collection;
}

async function loadSubject() {
    try {
        const subject = await window.api(
            "/v0/subjects/" + get(subjectState).sid
        );
        if (!subject) {
            patch({ subject: null, notFound: true, loading: false });
            return false;
        }
        await window.loadPersonMap();
        const infoItems = window.infoboxData(subject);
        const myColl = await loadMyCollection();
        patch({ subject, infoItems, myColl, notFound: false });
        return true;
    } catch (e) {
        patch({ error: e.message || "加载失败", loading: false });
        return false;
    }
}

async function loadOverview() {
    const sid = get(subjectState).sid;
    const [epsRaw, charsRaw, relsRaw, eloData, personsRaw] = await Promise.all([
        window.api("/v0/episodes?subject_id=" + sid),
        window.api("/v0/subjects/" + sid + "/characters"),
        window.api("/v0/subjects/" + sid + "/subjects"),
        window.apiFor("elo", "/v0/elo/ranking"),
        window.api("/v0/subjects/" + sid + "/persons")
    ]);
    const eps = list(epsRaw);
    const chars = list(charsRaw);
    const rels = list(relsRaw);
    const persons = list(personsRaw);
    let eloInfo = null;
    if (eloData && Array.isArray(eloData.data)) {
        for (let i = 0; i < eloData.data.length; i++) {
            if (eloData.data[i].id == sid) {
                eloInfo = eloData.data[i];
                break;
            }
        }
    }
    const mainChars = chars
        .filter((c) => c.relation === "主角" || c.relation === "配角")
        .slice(0, 12);
    patch({
        eps,
        chars,
        rels,
        persons,
        eloInfo,
        mainChars,
        counts: {
            ep: eps.length,
            chars: chars.length,
            persons: persons.length
        }
    });
}

async function loadEpisodes() {
    const eps = list(
        await window.api("/v0/episodes?subject_id=" + get(subjectState).sid)
    );
    subjectState.update((state) => ({
        ...state,
        eps,
        counts: { ...state.counts, ep: eps.length }
    }));
}

async function loadCharacters() {
    const chars = list(
        await window.api(
            "/v0/subjects/" + get(subjectState).sid + "/characters"
        )
    );
    subjectState.update((state) => ({
        ...state,
        chars,
        counts: { ...state.counts, chars: chars.length }
    }));
}

async function loadPersons() {
    const persons = list(
        await window.api("/v0/subjects/" + get(subjectState).sid + "/persons")
    );
    subjectState.update((state) => ({
        ...state,
        persons,
        counts: { ...state.counts, persons: persons.length }
    }));
}

async function loadTab() {
    const tab = get(subjectState).tab;
    patch({ error: "" });
    try {
        if (tab === "overview") await loadOverview();
        else if (tab === "ep") await loadEpisodes();
        else if (tab === "chars") await loadCharacters();
        else if (tab === "persons") await loadPersons();
    } catch (e) {
        patch({ error: e.message || "加载失败" });
    }
}

export function switchTab(tab) {
    const state = get(subjectState);
    patch({ tab, error: "" });
    history.replaceState({}, "", "?id=" + state.sid + "&tab=" + tab);
    loadTab();
}

export async function exportBundle() {
    const id = get(subjectState).sid;
    if (!id) return;
    window.confirmAction("导出 #" + id + " 的本地数据包？", async function () {
        patch({ exporting: true });
        window.showInfo("正在导出数据包");
        try {
            const res = await fetch(API_BASE + "/v0/bundle/export", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    source: "ids",
                    ids: [parseInt(id, 10)]
                })
            });
            const d = await window.readJSON(res);
            if (!res.ok || !d) {
                window.showError(window.errText(d, res.status));
                return;
            }
            window.showSuccess(
                "已导出 " +
                    window.escapeHtml(d.filename) +
                    "<br><span class='text-xs'>路径：" +
                    window.escapeHtml(d.path) +
                    "</span>"
            );
        } catch (_) {
            window.showError("导出失败");
        } finally {
            patch({ exporting: false });
        }
    });
}

export async function initSubject() {
    const params = new URLSearchParams(location.search);
    const sid = params.get("id") || "";
    let tab = params.get("tab") || "overview";
    if (!["overview", "ep", "chars", "persons"].includes(tab)) {
        tab = "overview";
    }
    patch({ sid, tab });
    await window.capabilitiesReady();
    patch({ showBundle: window.resolve("bundle") !== "hidden" });
    const ok = await loadSubject();
    if (!ok) return;
    await loadTab();
    patch({ loading: false });
}
