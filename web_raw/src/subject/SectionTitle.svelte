<script>
    let { text = "", outer = "mb-3", extra = "" } = $props();
</script>

<div class="flex items-center gap-2 {outer}">
    <span class="section-bar"></span>
    <span class="font-bold">{text}</span>
    {@html extra}
</div>
