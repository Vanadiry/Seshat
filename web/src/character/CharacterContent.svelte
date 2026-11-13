<script>
    import { characterState } from "./state.js";
    import CharacterDetail from "./CharacterDetail.svelte";

    let state = $derived($characterState);
</script>

{#if state.loading}
    <div class="text-center py-20 text-sub"><span class="spin"></span></div>
{:else if state.missing}
    <p class="text-sub text-center py-20">缺少角色 ID</p>
{:else if state.notFound}
    <p class="text-sub text-center py-10">未找到该角色，ID 为 {state.id}</p>
{:else if state.error}
    <p class="text-sub text-center py-20">渲染出错：{state.error}</p>
{:else if state.character}
    <CharacterDetail
        character={state.character}
        cn={state.cn}
        infoItems={state.infoItems}
        appearances={state.appearances}
    />
{/if}
