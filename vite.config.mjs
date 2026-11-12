import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { resolve } from "node:path";

export default defineConfig({
    plugins: [svelte()],
    build: {
        outDir: "web/assets",
        emptyOutDir: false,
        lib: {
            entry: {
                subject: resolve("web_raw/src/subject/main.js"),
                character: resolve("web_raw/src/character/main.js")
            },
            formats: ["es"],
            fileName: (format, entryName) => `${entryName}.js`
        },
        rollupOptions: {
            output: {
                chunkFileNames: "shared.js"
            }
        }
    }
});
