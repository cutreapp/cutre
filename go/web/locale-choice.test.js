import { beforeAll, beforeEach, describe, expect, it } from "vitest";

import { initializeLocaleChoice } from "./locale-choice.js";

// ハンドラーはdocumentへ一度だけ登録する。各テストでDOMを作り直し、後から追加された要素も検証する。
beforeAll(() => {
  initializeLocaleChoice();
});

beforeEach(() => {
  document.body.innerHTML = "";
  window.history.replaceState(null, "", "/");
  // 同じミリ秒内でも確実に削除されるよう、期限を過去の固定時刻にする。
  document.cookie = "selected_locale=; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT; SameSite=Lax; Secure";
});

function click(element) {
  const event = new MouseEvent("click", { bubbles: true, cancelable: true });
  element.dispatchEvent(event);
  return event;
}

describe.each([
  { locale: "ja", otherLocale: "en", label: "日本語", path: "/" },
  { locale: "en", otherLocale: "ja", label: "English", path: "/en" },
])("$localeの言語選択", ({ locale, otherLocale, label, path }) => {
  it.each(["言語スイッチャー", "案内の移動リンク"])("%sで移動先の言語を保存する", (source) => {
    const link = `<a href="${path}" data-locale-choice="${locale}"><span>${label}</span></a>`;
    document.body.innerHTML =
      source === "言語スイッチャー"
        ? `<nav>${link}</nav><aside data-locale-suggestion>案内</aside>`
        : `<aside data-locale-suggestion>${link}</aside>`;
    const anchor = document.querySelector("a");

    // 子要素を押した場合もリンクのdata属性まで辿り、リンク自身の移動は妨げない。
    const event = click(anchor.querySelector("span"));

    expect(document.cookie).toBe(`selected_locale=${locale}`);
    expect(event.defaultPrevented).toBe(false);
    expect(anchor.getAttribute("href")).toBe(path);
    expect(document.querySelector("[data-locale-suggestion]")).not.toBeNull();
  });

  it("閉じると表示中の言語を保存し、案内を取り除いて本文へフォーカスを移す", () => {
    document.body.innerHTML = `
      <aside data-locale-suggestion lang="${otherLocale}">
        <button type="button" data-locale-suggestion-dismiss data-locale-choice="${locale}"><span>閉じる</span></button>
      </aside>
      <main id="main" tabindex="-1">本文</main>
    `;
    const button = document.querySelector("button");
    button.focus();
    expect(document.activeElement).toBe(button);

    click(button.querySelector("span"));

    expect(document.cookie).toBe(`selected_locale=${locale}`);
    expect(document.querySelector("[data-locale-suggestion]")).toBeNull();
    expect(document.activeElement).toBe(document.getElementById("main"));
  });

  it("保存済みの選択を上書きし、別のパスでも選択を保持する", () => {
    document.cookie = `selected_locale=${otherLocale}; Path=/; SameSite=Lax; Secure`;
    window.history.replaceState(null, "", "/en/items");
    document.body.innerHTML = `<a href="${path}" data-locale-choice="${locale}">${label}</a>`;

    click(document.querySelector("a"));
    window.history.replaceState(null, "", "/items");

    expect(document.cookie).toBe(`selected_locale=${locale}`);
  });
});

describe("言語選択以外の操作", () => {
  it.each([
    '<button type="button">無関係なボタン</button>',
    '<button type="button" data-locale-choice="">空の選択</button>',
  ])("対象外の要素では保存済みの選択と案内を変更しない: %s", (button) => {
    document.cookie = "selected_locale=ja; Path=/; SameSite=Lax; Secure";
    document.body.innerHTML = `<aside data-locale-suggestion>${button}</aside>`;

    click(document.querySelector("button"));

    expect(document.cookie).toBe("selected_locale=ja");
    expect(document.querySelector("[data-locale-suggestion]")).not.toBeNull();
  });

  it("Element以外を起点にしたイベントではCookieを書かない", () => {
    click(document);

    expect(document.cookie).toBe("");
  });
});
