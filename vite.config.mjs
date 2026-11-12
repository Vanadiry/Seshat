import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { resolve } from "node:path";

const ENTRIES = {
    subject: "web/src/subject/main.js",
    character: "web/src/character/main.js",
    person: "web/src/person/main.js",
    tag: "web/src/tag/main.js",
    characters: "web/src/characters/main.js",
    persons: "web/src/persons/main.js",
    tags: "web/src/tags/main.js",
    home: "web/src/home/main.js",
    rating: "web/src/rating/main.js",
    stats: "web/src/stats/main.js",
    search: "web/src/search/main.js",
    settings: "web/src/settings/main.js",
    manage: "web/src/manage/main.js"
};

export default defineConfig({
    plugins: [svelte()],
    build: {
        outDir: "web-dist/assets",
        emptyOutDir: false,
        lib: {
            entry: Object.fromEntries(
                Object.entries(ENTRIES).map(function ([name, path]) {
                    return [name, resolve(path)];
                })
            ),
            formats: ["es"],
            fileName: (format, entryName) => `${entryName}.js`
        },
        rollupOptions: {
            output: {
                chunkFileNames: "shared-[name].js",
                assetFileNames: "[name][extname]"
            }
        }
    }
});
