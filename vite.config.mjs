import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { resolve } from "node:path";

const ENTRIES = {
    subject: "web_raw/src/subject/main.js",
    character: "web_raw/src/character/main.js",
    person: "web_raw/src/person/main.js"
};

export default defineConfig(({ mode }) => {
    const name = ENTRIES[mode] ? mode : "subject";
    return {
        plugins: [svelte()],
        build: {
            outDir: "web/assets",
            emptyOutDir: false,
            lib: {
                entry: resolve(ENTRIES[name]),
                formats: ["es"],
                fileName: () => `${name}.js`
            }
        }
    };
});
