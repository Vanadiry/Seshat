<script>
    import { onMount } from "svelte";
    import SectionTitle from "../subject/SectionTitle.svelte";
    import { imgFallback } from "../lib/img.js";

    const API_BASE = window.BACKEND_URL || "/api";

    const TYPE_LABELS = {
        subjects: "条目",
        characters: "角色",
        persons: "人物",
        tags: "标签"
    };
    const TYPE_LINKS = {
        subjects: "/subject?id=",
        characters: "/character?id=",
        persons: "/person?id="
    };
    const TYPE_IMAGES = {
        subjects: "subject",
        characters: "character",
        persons: "person"
    };

    let keyword = $state("");
    let started = $state(false);
    let searching = $state(false);
    let noResults = $state(false);
    let sections = $state([]);
    let currentKeyword = $state("");
    let tagsVisible = $state(true);
    let typeChecks = $state({
        subjects: true,
        characters: true,
        persons: false,
        tags: false
    });

    let abort = null;
    let seq = 0;

    let selectedTypes = $derived(
        ["subjects", "characters", "persons", "tags"].filter(function (t) {
            if (t === "tags" && !tagsVisible) return false;
            return typeChecks[t];
        })
    );

    function highlightText(text, q) {
        var s = String(text == null ? "" : text);
        var query = String(q || "");
        if (!query) return window.escapeHtml(s);
        var rs = Array.from(s);
        var rq = Array.from(query.toLowerCase());
        var out = "";
        var buf = "";
        for (var i = 0; i < rs.length; ) {
            var matched = true;
            for (var j = 0; j < rq.length; j++) {
                if (i + j >= rs.length || rs[i + j].toLowerCase() !== rq[j]) {
                    matched = false;
                    break;
                }
            }
            if (matched) {
                out += window.escapeHtml(buf);
                out +=
                    '<mark class="bg-pink-bg text-pink rounded px-0.5">' +
                    window.escapeHtml(rs.slice(i, i + rq.length).join("")) +
                    "</mark>";
                buf = "";
                i += rq.length;
            } else {
                buf += rs[i];
                i++;
            }
        }
        return out + window.escapeHtml(buf);
    }

    // 截取关键词附近的文本，保证高亮部分能显示出来
    function snippetAround(text, q, before, after) {
        var s = String(text == null ? "" : text);
        var query = String(q || "");
        if (!query) return s;
        var idx = s.toLowerCase().indexOf(query.toLowerCase());
        if (idx < 0) return s;
        var start = Math.max(0, idx - before);
        var end = Math.min(s.length, idx + query.length + after);
        return (
            (start > 0 ? "…" : "") +
            s.slice(start, end) +
            (end < s.length ? "…" : "")
        );
    }

    function toggleType(type, event) {
        if (!typeChecks[type] && selectedTypes.length <= 1) {
            if (event && event.currentTarget) {
                event.currentTarget.checked = true;
            }
            return;
        }
        typeChecks = { ...typeChecks, [type]: !typeChecks[type] };
        seq++;
        if (abort) {
            abort.abort();
            abort = null;
        }
    }

    async function searchType(type, q, signal) {
        var body = { keyword: q };
        if (type === "subjects") body.filter = { type: [2] };
        var resp = await fetch(API_BASE + "/v0/search/" + type, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(body),
            signal: signal
        });
        if (!resp.ok) throw new Error("search failed");
        var d = await resp.json();
        if (!d || !Array.isArray(d.data)) throw new Error("bad response");
        return {
            data: d.data,
            total: typeof d.total === "number" ? d.total : d.data.length
        };
    }

    async function doSearch() {
        var q = keyword.trim();
        if (!q) return;
        var types = selectedTypes;
        if (!types.length) return;
        started = true;
        searching = true;
        noResults = false;
        sections = [];
        if (abort) abort.abort();
        var ctl = new AbortController();
        abort = ctl;
        var mySeq = ++seq;
        var settled = await Promise.allSettled(
            types.map(function (type) {
                return searchType(type, q, ctl.signal);
            })
        );
        if (mySeq !== seq) return;
        var next = [];
        for (var i = 0; i < settled.length; i++) {
            var type = types[i];
            var r = settled[i];
            if (r.status === "fulfilled") {
                if (r.value.data.length) {
                    next.push({
                        type: type,
                        data: r.value.data,
                        total: r.value.total
                    });
                }
            } else if (!r.reason || r.reason.name !== "AbortError") {
                next.push({ type: type, error: true });
            }
        }
        sections = next;
        noResults = next.length === 0;
        currentKeyword = q;
        searching = false;
    }

    function sectionCount(section) {
        return section.total > section.data.length
            ? "500+"
            : String(section.total);
    }

    onMount(function () {
        (async function () {
            await window.capabilitiesReady();
            if (window.resolve("tags") === "hidden") {
                tagsVisible = false;
                typeChecks = { ...typeChecks, tags: false };
            }
            var q = new URLSearchParams(location.search).get("q");
            if (q) {
                keyword = q;
                doSearch();
            }
        })();
    });
</script>

{#snippet resultCard(type, it)}
    {#if type === "tags"}
        <a
            href={"/tag?tag=" + encodeURIComponent(it.name)}
            class="fade-in no-underline flex gap-3 items-center bg-surface border border-bord-mid rounded-lg py-1.5 px-2 cursor-pointer hover:border-pink"
        >
            <div class="text-sm font-semibold">
                {@html highlightText(it.name, currentKeyword)}
            </div>
            <div class="text-xs text-sub">{it.count} 个条目</div>
        </a>
    {:else}
        {@const primary = window.primaryName(it.name, it.name_cn)}
        {@const sub = window.subName(it.name, it.name_cn)}
        <a
            href={TYPE_LINKS[type] + it.id}
            class="fade-in flex gap-3 items-center bg-surface border border-bord-mid rounded-lg py-1.5 px-2 cursor-pointer hover:border-pink no-underline"
        >
            <img
                alt=""
                src={window.img(TYPE_IMAGES[type], it.id, "grid")}
                use:imgFallback={{
                    kind: TYPE_IMAGES[type],
                    id: it.id,
                    size: "grid"
                }}
                class="w-10 h-10 rounded-lg object-cover bg-surface shrink-0"
            />
            <div class="flex-1 min-w-0">
                <div class="flex items-baseline gap-2 min-w-0">
                    <span class="text-sm font-semibold truncate"
                        >{@html highlightText(
                            snippetAround(primary, currentKeyword, 3, 30),
                            currentKeyword
                        )}</span
                    >
                    {#if sub}
                        <span class="text-xs text-sub truncate max-w-[45%]"
                            >{@html highlightText(
                                snippetAround(sub, currentKeyword, 3, 20),
                                currentKeyword
                            )}</span
                        >
                    {/if}
                </div>
                {#if it.match && (it.match.field === "infobox" || it.match.field === "summary")}
                    <div class="text-xs text-sub truncate leading-tight mt-0.5">
                        {it.match.field === "summary"
                            ? "简介"
                            : it.match.key || "信息"}：{@html highlightText(
                            snippetAround(it.match.text || "", currentKeyword, 4, 40),
                            currentKeyword
                        )}
                    </div>
                {/if}
            </div>
            <div class="text-xs text-sub shrink-0">#{it.id}</div>
        </a>
    {/if}
{/snippet}

<div id="search-area" style="margin-top:{started ? '0' : '25vh'}">
    <div
        class="flex items-stretch bg-input border border-bord-strong rounded-xl overflow-hidden focus-within:border-pink transition-colors"
    >
        <!-- svelte-ignore a11y_autofocus -->
        <input
            id="q"
            placeholder="搜索…"
            class="flex-1 px-4 py-3 bg-transparent text-base outline-none border-0 min-w-0"
            autofocus
            bind:value={keyword}
            onkeydown={(e) => {
                if (e.key === "Enter") doSearch();
            }}
        />
        <button
            type="button"
            onclick={doSearch}
            class="bg-pink-solid text-white font-semibold cursor-pointer border-0 hover:opacity-90 shrink-0 py-3"
            style="width: 72px"
        >
            搜索
        </button>
    </div>
    <div
        id="search-types"
        class="mt-3 flex flex-wrap items-center justify-center gap-x-5 gap-y-2 text-sm text-sub"
    >
        {#each [["subjects", "条目"], ["characters", "角色"], ["persons", "人物"], ["tags", "标签"]] as [key, label]}
            <label
                class="flex items-center gap-1.5 cursor-pointer select-none {key ===
                    'tags' && !tagsVisible
                    ? 'hidden'
                    : ''}"
            >
                <input
                    type="checkbox"
                    checked={typeChecks[key]}
                    onchange={(e) => toggleType(key, e)}
                    class="cursor-pointer"
                    style="accent-color: var(--c-pink)"
                />
                {label}
            </label>
        {/each}
    </div>
</div>

<div id="results" class="mt-6">
    {#if searching}
        <div class="text-center py-10 text-sub">
            <span class="spin"></span>搜索中…
        </div>
    {:else if noResults}
        <div class="text-center py-16 text-sub">未找到结果</div>
    {:else}
        {#each sections as section}
            <div class="mb-6">
                {#if section.error}
                    <SectionTitle text={TYPE_LABELS[section.type]} />
                    <div class="text-center py-6 text-sub">搜索出错</div>
                {:else}
                    <SectionTitle
                        text={TYPE_LABELS[section.type]}
                        extra={'<span class="text-xs text-sub">(' +
                            sectionCount(section) +
                            ")</span>"}
                    />
                    <div class="flex flex-col gap-1">
                        {#each section.data as it (it.id != null ? it.id : it.name)}
                            {@render resultCard(section.type, it)}
                        {/each}
                    </div>
                {/if}
            </div>
        {/each}
    {/if}
</div>
