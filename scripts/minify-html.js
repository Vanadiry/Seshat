const fs = require("fs");
const path = require("path");
const { minify } = require("html-minifier-terser");

const SRC_DIR = path.join(__dirname, "../web");
const DEST_DIR = path.join(__dirname, "../web-dist");

const FILES = [
    "character.html",
    "characters.html",
    "index.html",
    "manage.html",
    "person.html",
    "persons.html",
    "rating.html",
    "search.html",
    "settings.html",
    "stats.html",
    "subject.html",
    "tag.html",
    "tags.html"
];

const STATIC_FILES = ["favicon.svg", "global.svg", "link.svg", "no-image.png"];

async function processFile(file) {
    const srcPath = path.join(SRC_DIR, file);
    const destPath = path.join(DEST_DIR, file);
    const content = fs.readFileSync(srcPath, "utf-8");
    const result = await minify(content, {
        collapseWhitespace: true,
        removeComments: true,
        minifyJS: true,
        minifyCSS: true,
        removeEmptyAttributes: true,
        removeRedundantAttributes: true,
        useShortDoctype: true
    });
    const destDir = path.dirname(destPath);
    fs.mkdirSync(destDir, { recursive: true });
    fs.writeFileSync(destPath, result, "utf-8");
}

function copyStatic() {
    const destDir = path.join(DEST_DIR, "assets");
    fs.mkdirSync(destDir, { recursive: true });
    for (const file of STATIC_FILES) {
        fs.copyFileSync(
            path.join(SRC_DIR, "assets", file),
            path.join(destDir, file)
        );
    }
}

async function main() {
    for (const file of FILES) {
        await processFile(file);
    }
    copyStatic();
}

main();
