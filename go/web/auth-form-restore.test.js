import { beforeAll, beforeEach, describe, expect, it } from "vitest";

import { initializeAuthFormRestore } from "./auth-form-restore.js";

beforeAll(() => {
  initializeAuthFormRestore();
});

beforeEach(() => {
  document.body.innerHTML = "";
});

function pageshow(persisted) {
  const event = new Event("pageshow");
  Object.defineProperty(event, "persisted", { value: persisted });
  window.dispatchEvent(event);
}

describe("認証フォームの履歴復元", () => {
  it("BFCacheから復元したときだけ入力中の値を初期状態に戻す", () => {
    document.body.innerHTML = `
      <form data-clear-on-history-restore>
        <input name="email" value="server@example.com">
        <input name="password" type="password">
      </form>`;
    const email = document.querySelector('[name="email"]');
    const password = document.querySelector('[name="password"]');
    email.value = "typed@example.com";
    password.value = "入力中のパスワード";

    pageshow(false);
    expect(email.value).toBe("typed@example.com");
    expect(password.value).toBe("入力中のパスワード");

    pageshow(true);
    expect(email.value).toBe("server@example.com");
    expect(password.value).toBe("");
  });

  it("対象外のフォームには触れない", () => {
    document.body.innerHTML = `<form><input name="email"></form>`;
    const email = document.querySelector('[name="email"]');
    email.value = "typed@example.com";

    pageshow(true);

    expect(email.value).toBe("typed@example.com");
  });
});
