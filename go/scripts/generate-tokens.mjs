// generate-tokens.mjs はデザイントークンの正本 (web/tokens.json) から web/tokens.css を生成する。
// `--check` を付けると書き込まず、web/tokens.css が生成結果と食い違えば失敗する。
//
// 出力はOxfmtの整形結果と一致させ、`make fmt` を通しても差分が出ないようにしている。
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const WEB_DIR = join(import.meta.dirname, "..", "web");
const SOURCE_PATH = join(WEB_DIR, "tokens.json");
const OUTPUT_PATH = join(WEB_DIR, "tokens.css");

const HEADER =
  "/* web/tokens.json から scripts/generate-tokens.mjs で生成する。直接編集せず、`make -C go tokens-generate` で作り直すこと。 */";

// テーマごとに色の変数を出力するセレクター。
// tokens.json はテーマのidだけを持つため、どの要素に当てるかはここで決める。
// ダークはBasecoatの `dark:` バリアントと `color-scheme: dark` に合わせ、html要素の `.dark` クラスで切り替える。
const THEME_SELECTORS = {
  light: [":root"],
  dark: [".dark"],
};

// Basecoatの `@theme` が `--color-*` に対応付け済みの色 (shadcn/ui互換の変数)。
// これ以外の色は、Tailwindのユーティリティ (`bg-brand` など) を生成するため `@theme inline` で対応付ける。
const BASECOAT_COLOR_NAMES = new Set([
  "background",
  "foreground",
  "card",
  "card-foreground",
  "popover",
  "popover-foreground",
  "primary",
  "primary-foreground",
  "secondary",
  "secondary-foreground",
  "muted",
  "muted-foreground",
  "accent",
  "accent-foreground",
  "destructive",
  "border",
  "input",
  "ring",
  "chart-1",
  "chart-2",
  "chart-3",
  "chart-4",
  "chart-5",
  "sidebar",
  "sidebar-foreground",
  "sidebar-primary",
  "sidebar-primary-foreground",
  "sidebar-accent",
  "sidebar-accent-foreground",
  "sidebar-border",
  "sidebar-ring",
]);

// `{brand-strong}` のように波括弧で囲んだ値は、同じテーマの別のトークンへの参照として var() に置き換える。
const ALIAS_PATTERN = /^\{([a-z0-9-]+)\}$/;

function block(selectors, declarations) {
  const lines = declarations.map(([name, value]) => `  ${name}: ${value};`);
  return `${selectors.join(",\n")} {\n${lines.join("\n")}\n}`;
}

function colorValue(token, themeId, tokenNames) {
  const value = token.value?.[themeId];
  if (value === undefined) {
    throw new Error(`色のトークン "${token.name}" にテーマ "${themeId}" の値がありません`);
  }
  const alias = ALIAS_PATTERN.exec(value);
  if (alias === null) {
    return value;
  }
  if (!tokenNames.has(alias[1])) {
    throw new Error(`色のトークン "${token.name}" が存在しないトークン "${alias[1]}" を参照しています`);
  }
  return `var(--${alias[1]})`;
}

function themeBlocks(color) {
  const tokenNames = new Set(color.tokens.map((token) => token.name));
  return color.themes.map((theme) => {
    const selectors = THEME_SELECTORS[theme.id];
    if (selectors === undefined) {
      throw new Error(`テーマ "${theme.id}" のセレクターが THEME_SELECTORS に定義されていません`);
    }
    return block(
      selectors,
      color.tokens.map((token) => [`--${token.name}`, colorValue(token, theme.id, tokenNames)]),
    );
  });
}

// 角丸・文字サイズ・フォントは、BasecoatとTailwindが参照する変数をそのまま上書きする。
function rootBlock(tokens) {
  return block(
    [":root"],
    [
      ...tokens.radius.tokens.map((token) => [`--${token.name}`, token.value]),
      ...(tokens.fontSize?.tokens ?? []).map((token) => [`--${token.name}`, token.value]),
      ...Object.entries(tokens.type.families).map(([name, value]) => [`--font-${name}`, value]),
    ],
  );
}

function themeInlineBlocks(color) {
  const names = color.tokens.map((token) => token.name).filter((name) => !BASECOAT_COLOR_NAMES.has(name));
  if (names.length === 0) {
    return [];
  }
  return [
    block(
      ["@theme inline"],
      names.map((name) => [`--color-${name}`, `var(--${name})`]),
    ),
  ];
}

export function generateTokensCSS(tokens) {
  // 文字の尺度はTailwindの既定を使うため、独自の文字スタイルは出力しない。
  // 定義されていたら黙って捨てずに知らせる。
  if ((tokens.type.groups ?? []).length > 0) {
    throw new Error(
      "文字スタイル (type.groups) は出力できません。Tailwindの文字サイズ (fontSize) を上書きしてください",
    );
  }
  const blocks = [...themeBlocks(tokens.color), rootBlock(tokens), ...themeInlineBlocks(tokens.color)];
  return `${HEADER}\n\n${blocks.join("\n\n")}\n`;
}

function main(args) {
  const css = generateTokensCSS(JSON.parse(readFileSync(SOURCE_PATH, "utf8")));

  if (!args.includes("--check")) {
    writeFileSync(OUTPUT_PATH, css);
    return 0;
  }

  let current = "";
  try {
    current = readFileSync(OUTPUT_PATH, "utf8");
  } catch (error) {
    if (error.code !== "ENOENT") {
      throw error;
    }
  }
  if (current !== css) {
    console.error("web/tokens.css が web/tokens.json と同期していません。make tokens-generate を実行してください");
    return 1;
  }
  return 0;
}

if (import.meta.main) {
  process.exitCode = main(process.argv.slice(2));
}
