<script>
    let trackerVisible = window.resolve("tracker") !== "hidden";

    function refreshUser() {
        if (!window.USERNAME) {
            window.showError(window.MSG.errNoUsername);
            return;
        }
        window.confirmAction(window.MSG.confirmUserFetch(), window.doUserFetch);
    }

    function importCollections() {
        window.confirmAction(
            "将用当前收藏列表重建 user.json。\nuser.toml 不受影响。\n已取消收藏的条目会从 user.json 移除。",
            window.doImportCollections
        );
    }
</script>

<section class="mb-8">
    <div class="flex items-center gap-2 mb-3">
        <span class="section-bar"></span>
        <h2 class="font-bold text-sm">用户数据</h2>
    </div>
    <p class="text-xs text-sub mb-3">
        重新拉取当前用户的信息、头像和收藏，或把收藏同步到 Tracker。
    </p>
    <div class="flex flex-col gap-2">
        <button
            type="button"
            onclick={refreshUser}
            class="h-10 w-full px-4 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text"
        >
            刷新用户数据
        </button>
        {#if trackerVisible}
            <button
                type="button"
                onclick={importCollections}
                class="h-10 w-full px-4 py-2 rounded-lg border border-bord bg-transparent text-sub text-sm cursor-pointer hover:bg-active hover:text-text"
            >
                同步收藏到 Tracker
            </button>
        {/if}
    </div>
</section>
