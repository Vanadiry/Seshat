const fs = require("fs");
const path = require("path");

// 按文件名前缀顺序拼接共享脚本，保持和拆分前的执行顺序一致
const SRC_DIR = path.join(__dirname, "../web/assets/js");
const OUT = path.join(__dirname, "../build/app.js");

const files = fs
    .readdirSync(SRC_DIR)
    .filter(function (f) {
        return f.endsWith(".js");
    })
    .sort();

if (!files.length) {
    console.error("web/assets/js 下没有可拼接的脚本");
    process.exit(1);
}

const code =
    files
        .map(function (f) {
            return fs.readFileSync(path.join(SRC_DIR, f), "utf8").trimEnd();
        })
        .join("\n;\n") + "\n";

fs.mkdirSync(path.dirname(OUT), { recursive: true });
fs.writeFileSync(OUT, code);
console.log(`合并 ${files.length} 个脚本 -> build/app.js`);
