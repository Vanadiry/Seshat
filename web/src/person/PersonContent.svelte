<script>
    import { personState } from "./state.js";
    import PersonDetail from "./PersonDetail.svelte";

    let state = $derived($personState);
</script>

{#if state.loading}
    <div class="text-center py-20 text-sub"><span class="spin"></span></div>
{:else if state.missing}
    <p class="text-sub text-center py-20">缺少人物 ID</p>
{:else if state.notFound}
    <p class="text-sub text-center py-10">未找到该人物，ID 为 {state.id}</p>
{:else if state.error}
    <p class="text-sub text-center py-20">渲染出错：{state.error}</p>
{:else if state.person}
    <PersonDetail
        person={state.person}
        cn={state.cn}
        infoItems={state.infoItems}
        characters={state.characters}
        subjects={state.subjects}
    />
{/if}
