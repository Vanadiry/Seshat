<script>
    import { subjectState } from "./state.js";
    import Overview from "./Overview.svelte";
    import EpisodeList from "./EpisodeList.svelte";
    import CharacterList from "./CharacterList.svelte";
    import PersonList from "./PersonList.svelte";

    let state = $derived($subjectState);
    let myRate = $derived(
        state.myColl && state.myColl.rate > 0 ? Number(state.myColl.rate) : 0
    );
    let myType = $derived(
        state.myColl
            ? (window.COLL_TYPES[state.myColl.type] || "")
            : ""
    );
</script>

{#if state.loading}
    <div class="text-center py-20 text-sub"><span class="spin"></span></div>
{:else if state.notFound}
    <div class="select-text fade-in">
        <p class="text-sub text-center py-20">未找到</p>
    </div>
{:else if state.error}
    <div class="select-text fade-in">
        <p class="text-sub text-center py-20">渲染出错: {state.error}</p>
    </div>
{:else if state.subject && state.tab === "overview"}
    <Overview
        subject={state.subject}
        infoItems={state.infoItems}
        {myRate}
        {myType}
        hasColl={!!state.myColl}
        eloInfo={state.eloInfo}
        eps={state.eps}
        mainChars={state.mainChars}
        rels={state.rels}
    />
{:else if state.tab === "ep"}
    <EpisodeList eps={state.eps} />
{:else if state.tab === "chars"}
    <CharacterList chars={state.chars} />
{:else if state.tab === "persons"}
    <PersonList persons={state.persons} />
{/if}
