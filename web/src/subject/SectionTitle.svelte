<script>
    let { text = "", outer = "mb-3", extra = "", label = "" } = $props();
</script>

<div class="flex items-center gap-2 {outer}">
    <span class="section-bar"></span>
    <span class="font-bold {label}">{text}</span>
    {@html extra}
</div>
