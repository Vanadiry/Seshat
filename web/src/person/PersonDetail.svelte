<script>
    import SectionTitle from "../subject/SectionTitle.svelte";
    import CreditList from "./CreditList.svelte";
    import { imgFallback } from "../lib/img.js";

    let {
        person = {},
        cn = "",
        infoItems = [],
        characters = [],
        subjects = []
    } = $props();

    let career = $derived.by(() => {
        var value = person.career;
        if (typeof value === "string") {
            try {
                value = JSON.parse(value);
            } catch (_) {
                value = [];
            }
        }
        return Array.isArray(value) ? value : [];
    });

    let hasLeft = $derived(career.length > 0 || !!person.summary);
    let hasCollects = $derived(!!(person.stat && person.stat.collects));

    let posterSrc = $derived(window.img("person", person.id, "large"));
</script>

{#snippet leftBlock()}
    {#if career.length}
        <SectionTitle text="职业" />
        <div class="flex flex-wrap gap-1.5">
            {#each career as c}
                <span
                    class="bg-pink-bg text-pink px-2 py-0.5 rounded-md text-[12px]"
                    >{window.MSG.careerMap[c] || c}</span
                >
            {/each}
        </div>
    {/if}
    {#if person.summary}
        <div class={career.length ? "mt-4" : ""}>
            <SectionTitle text="简介" />
            <p class="select-text text-[14px] leading-relaxed text-sub">
                {@html window.formatSummary(window.escapeHtml(person.summary))}
            </p>
        </div>
    {/if}
{/snippet}

<div class="fade-in flex flex-col md:flex-row gap-6">
    <div class="w-full md:w-[260px] shrink-0">
        <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
        <img
            alt=""
            src={posterSrc}
            use:imgFallback={{
                kind: "person",
                id: person.id,
                size: "large"
            }}
            class="w-full rounded-xl cursor-pointer"
            tabindex="0"
            role="button"
            aria-label="查看大图"
            onkeydown={(e) => window.clickKeyNav(e)}
            onclick={() => window.showLightbox(posterSrc)}
        />
        {#if infoItems.length}
            <div class="mt-3 flex flex-col gap-0.5">
                {#each infoItems as item}
                    <div
                        class="select-text py-1 border-b border-bord-light text-[14px]"
                        class:ml-4={item[2]}
                    >
                        <span class="text-sub"
                            >{item[0] ? item[0] + ": " : ""}</span
                        ><span>{@html String(item[1])}</span>
                    </div>
                {/each}
            </div>
        {/if}
    </div>

    <div class="flex-1 min-w-0 flex flex-col gap-5">
        {#if hasLeft}
            {#if hasCollects}
                <div class="flex flex-col md:flex-row gap-4">
                    <div class="flex-1 min-w-0">
                        {@render leftBlock()}
                    </div>
                    <div
                        class="w-full md:w-[230px] shrink-0 self-start bg-surface-alt border border-bord rounded-xl overflow-hidden"
                    >
                        <div class="bg-surface-alt px-4 py-2">
                            <span class="font-bold text-sm">收藏</span>
                        </div>
                        <div class="border-t border-bord-mid"></div>
                        <div class="px-4 pt-3 pb-3">
                            <div class="flex items-baseline justify-center">
                                <span class="text-2xl font-bold"
                                    >{person.stat.collects}</span
                                ><span class="text-xs text-sub ml-1">人</span>
                            </div>
                        </div>
                    </div>
                </div>
            {:else}
                <div>{@render leftBlock()}</div>
            {/if}
        {/if}

        <CreditList
            items={characters}
            title="出演角色"
            kind="chars"
            id={person.id}
        />
        <CreditList
            items={subjects}
            title="参与制作"
            kind="subjects"
            id={person.id}
        />
    </div>
</div>
