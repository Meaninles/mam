import { readdir, readFile } from "node:fs/promises";
import { basename, extname, join, relative } from "node:path";

const rootDir = process.cwd();
const utf8Decoder = new TextDecoder("utf-8", { fatal: true });
const ignoredDirs = new Set([
  ".git",
  ".tmp",
  "build",
  "coverage",
  "dist",
  "node_modules",
  "target"
]);
const ignoredFiles = new Set([
  "scripts/check-utf8.mjs"
]);
const textExtensions = new Set([
  ".bat",
  ".cmd",
  ".css",
  ".editorconfig",
  ".gitignore",
  ".go",
  ".gitattributes",
  ".html",
  ".js",
  ".json",
  ".jsonc",
  ".jsx",
  ".md",
  ".mjs",
  ".ps1",
  ".rs",
  ".scss",
  ".sql",
  ".toml",
  ".ts",
  ".tsx",
  ".yaml",
  ".yml"
]);
const dotTextFiles = new Set([
  ".editorconfig",
  ".gitattributes",
  ".gitignore"
]);
const suspiciousMojibakeTokens = [
  "鈥",
  "鏈",
  "灏濊瘯",
  "鍙敤",
  "璇峰厛",
  "鐢ㄦ埛",
  "闀滃儚"
];

function isTextFile(filePath) {
  const name = basename(filePath);
  return dotTextFiles.has(name) || textExtensions.has(extname(filePath).toLowerCase());
}

async function collectFiles(dirPath) {
  const entries = await readdir(dirPath, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    if (entry.isDirectory()) {
      if (ignoredDirs.has(entry.name)) {
        continue;
      }

      files.push(...(await collectFiles(join(dirPath, entry.name))));
      continue;
    }

    if (entry.isFile()) {
      const fullPath = join(dirPath, entry.name);
      if (isTextFile(fullPath)) {
        files.push(fullPath);
      }
    }
  }

  return files;
}

const invalidUtf8Files = [];
const suspiciousFiles = [];
const bomFiles = [];

for (const filePath of await collectFiles(rootDir)) {
  const buffer = await readFile(filePath);
  const relPath = relative(rootDir, filePath).replaceAll("\\", "/");

  if (ignoredFiles.has(relPath)) {
    continue;
  }

  if (buffer.length >= 3 && buffer[0] === 0xef && buffer[1] === 0xbb && buffer[2] === 0xbf) {
    bomFiles.push(relPath);
  }

  let text;
  try {
    text = utf8Decoder.decode(buffer);
  } catch {
    invalidUtf8Files.push(relPath);
    continue;
  }

  const hits = suspiciousMojibakeTokens.filter((token) => text.includes(token));
  if (hits.length > 0) {
    suspiciousFiles.push({ path: relPath, hits: [...new Set(hits)] });
  }
}

if (invalidUtf8Files.length > 0) {
  console.error("发现非 UTF-8 文本文件：");
  for (const filePath of invalidUtf8Files) {
    console.error(`- ${filePath}`);
  }
}

if (bomFiles.length > 0) {
  console.warn("发现带 BOM 的 UTF-8 文件：");
  for (const filePath of bomFiles) {
    console.warn(`- ${filePath}`);
  }
}

if (suspiciousFiles.length > 0) {
  console.warn("发现疑似乱码片段，请人工复核：");
  for (const file of suspiciousFiles) {
    console.warn(`- ${file.path} (${file.hits.join(", ")})`);
  }
}

if (invalidUtf8Files.length === 0) {
  console.log("所有扫描到的文本文件都能按 UTF-8 解码。");
}

if (invalidUtf8Files.length > 0) {
  process.exitCode = 1;
}
