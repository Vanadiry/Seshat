import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { resolve } from "node:path";

const ENTRIES = {
    subject: "web/src/subject/main.js",
    character: "web/src/character/main.js",
    person: "web/src/person/main.js",
    tag: "web/src/tag/main.js"
};

export default defineConfig(({ mode }) => {
    const name = ENTRIES[mode] ? mode : "subject";
    return {
        plugins: [svelte()],
        build: {
            outDir: "web-dist/assets",
            emptyOutDir: false,
            lib: {
                entry: resolve(ENTRIES[name]),
                formats: ["es"],
                fileName: () => `${name}.js`
            }
        }
    };
});
