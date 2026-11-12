<script>
    import SectionTitle from "../subject/SectionTitle.svelte";
    import AppearanceList from "./AppearanceList.svelte";
    import { imgFallback } from "../lib/img.js";

    let {
        character = {},
        cn = "",
        infoItems = [],
        appearances = []
    } = $props();

    let posterSrc = $derived(window.img("character", character.id, "large"));

    let remoteExtra = $derived(
        window.isRemote("/v0/characters/" + character.id)
            ? window.globeIcon("w-4 h-4")
            : ""
    );
</script>

<div class="fade-in flex flex-col md:flex-row gap-6">
    <div class="w-full md:w-[260px] shrink-0">
        <img
            alt=""
            src={posterSrc}
            use:imgFallback={{
                kind: "character",
                id: character.id,
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
        {#if character.summary || (character.stat && character.stat.collects)}
            <div class="flex flex-col md:flex-row gap-4">
                <div class="flex-1 min-w-0">
                    {#if character.summary}
                        <SectionTitle text="简介" />
                        <p
                            class="select-text text-[14px] leading-relaxed text-sub"
                        >
                            {@html window.formatSummary(
                                window.escapeHtml(character.summary)
                            )}
                        </p>
                    {/if}
                </div>
                {#if character.stat && character.stat.collects}
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
                                    >{character.stat.collects}</span
                                ><span class="text-xs text-sub ml-1">人</span>
                            </div>
                        </div>
                    </div>
                {/if}
            </div>
        {/if}

        {#if character.actors && character.actors.length}
            <div>
                <SectionTitle text="声优" extra={remoteExtra} />
                <div class="flex flex-wrap gap-2">
                    {#each character.actors as a}
                        <a
                            href={"/person?id=" + a.id}
                            class="bg-surface border border-bord rounded-lg px-3 py-1.5 text-sm cursor-pointer hover:border-pink no-underline"
                            >{a.name}</a
                        >
                    {/each}
                </div>
            </div>
        {/if}

        <AppearanceList items={appearances} id={character.id} />
    </div>
</div>
