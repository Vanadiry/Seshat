const fs = require("fs");
const path = require("path");
const { minify } = require("@swc/html");

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
    const { code } = await minify(content, {
        collapseWhitespaces: "all",
        removeComments: true,
        minifyJs: true,
        minifyCss: true,
        removeEmptyAttributes: true,
        removeRedundantAttributes: "all",
        tagOmission: false
    });
    const destDir = path.dirname(destPath);
    fs.mkdirSync(destDir, { recursive: true });
    fs.writeFileSync(destPath, code, "utf-8");
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
    await Promise.all(FILES.map(processFile));
    copyStatic();
}

main();
