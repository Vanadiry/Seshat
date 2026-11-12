import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { resolve } from "node:path";

export default defineConfig({
    plugins: [svelte()],
    build: {
        outDir: "web/assets",
        emptyOutDir: false,
        lib: {
            entry: resolve("web_raw/src/subject/main.js"),
            formats: ["es"],
            fileName: () => "subject.js"
        }
    }
});
