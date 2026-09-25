import { describe, expect, it } from "vitest";

import { generateTokensCSS } from "./generate-tokens.mjs";

// 各テストで一部を書き換えられるよう、呼び出すたびに新しいオブジェクトを返す。
function buildTokens() {
  return {
    color: {
      themes: [{ id: "light" }, { id: "dark" }],
      tokens: [
        { name: "primary", value: { light: "{brand-strong}", dark: "{brand-strong}" } },
        { name: "brand-strong", value: { light: "oklch(0.52 0.19 7)", dark: "oklch(0.86 0.1 7)" } },
      ],
    },
    radius: {
      tokens: [{ name: "radius", value: "0.625rem" }],
    },
    fontSize: {
      tokens: [
        { name: "text-xs", value: "0.8125rem" },
        { name: "text-xs--line-height", value: "calc(1.125 / 0.8125)" },
      ],
    },
    type: {
      families: { sans: "ui-sans-serif, sans-serif", mono: "ui-monospace, monospace" },
      fonts: [],
      groups: [],
    },
  };
}

describe("generateTokensCSS", () => {
  it("テーマごとの色・:rootの値・Basecoatに無い色の対応付けを順に出力する", () => {
    const css = generateTokensCSS(buildTokens());

    expect(css)
      .toBe(`/* web/tokens.json から scripts/generate-tokens.mjs で生成する。直接編集せず、\`make -C go tokens-generate\` で作り直すこと。 */

:root {
  --primary: var(--brand-strong);
  --brand-strong: oklch(0.52 0.19 7);
}

.dark {
  --primary: var(--brand-strong);
  --brand-strong: oklch(0.86 0.1 7);
}

:root {
  --radius: 0.625rem;
  --text-xs: 0.8125rem;
  --text-xs--line-height: calc(1.125 / 0.8125);
  --font-sans: ui-sans-serif, sans-serif;
  --font-mono: ui-monospace, monospace;
}

@theme inline {
  --color-brand-strong: var(--brand-strong);
}
`);
  });

  it("Basecoatに無い色が無ければ @theme inline を出力しない", () => {
    const tokens = buildTokens();
    tokens.color.tokens = tokens.color.tokens.filter((token) => token.name !== "brand-strong");
    tokens.color.tokens[0].value = { light: "oklch(0.2 0 0)", dark: "oklch(0.9 0 0)" };

    expect(generateTokensCSS(tokens)).not.toContain("@theme");
  });

  it("fontSize が無くても出力できる", () => {
    const tokens = buildTokens();
    delete tokens.fontSize;

    expect(generateTokensCSS(tokens)).not.toContain("--text-xs");
  });

  it("テーマの値が欠けた色のトークンがあればエラーにする", () => {
    const tokens = buildTokens();
    delete tokens.color.tokens[1].value.dark;

    expect(() => generateTokensCSS(tokens)).toThrow('色のトークン "brand-strong" にテーマ "dark" の値がありません');
  });

  it("存在しないトークンへの参照があればエラーにする", () => {
    const tokens = buildTokens();
    tokens.color.tokens[0].value.light = "{brand}";

    expect(() => generateTokensCSS(tokens)).toThrow(
      '色のトークン "primary" が存在しないトークン "brand" を参照しています',
    );
  });

  it("セレクターを定義していないテーマがあればエラーにする", () => {
    const tokens = buildTokens();
    tokens.color.themes.push({ id: "sepia" });

    expect(() => generateTokensCSS(tokens)).toThrow(
      'テーマ "sepia" のセレクターが THEME_SELECTORS に定義されていません',
    );
  });

  it("文字スタイルが定義されていればエラーにする", () => {
    const tokens = buildTokens();
    tokens.type.groups.push({ name: "Text", family: "sans", styles: [] });

    expect(() => generateTokensCSS(tokens)).toThrow("文字スタイル (type.groups) は出力できません");
  });
});
