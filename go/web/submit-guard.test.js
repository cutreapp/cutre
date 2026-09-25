import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { initializeSubmitGuard } from "./submit-guard.js";

// ハンドラーはdocumentとwindowへ一度だけ登録する。各テストでDOMを作り直す。
beforeAll(() => {
  initializeSubmitGuard();
});

beforeEach(() => {
  vi.useFakeTimers();
  document.body.innerHTML = "";
});

afterEach(() => {
  vi.useRealTimers();
});

// submit はフォームの送信イベントを発火させる。実際の遷移は起こさない。
function submit(form) {
  const event = new Event("submit", { bubbles: true, cancelable: true });
  form.dispatchEvent(event);
  return event;
}

describe("送信ボタンの無効化", () => {
  it("data-disable-on-submit を持つフォームの送信ボタンを、送信の後に無効にする", () => {
    document.body.innerHTML = `
      <form data-disable-on-submit>
        <input name="email">
        <button type="submit">ログイン</button>
        <button>既定の種類も送信ボタン</button>
        <button type="button">送信しない</button>
      </form>`;
    const form = document.querySelector("form");

    submit(form);

    // 送信の途中では無効にしない。押されたボタンの値が送信から外れるため。
    const [submitButton, defaultButton, plainButton] = form.querySelectorAll("button");
    expect(submitButton.disabled).toBe(false);

    vi.runAllTimers();

    expect(submitButton.disabled).toBe(true);
    expect(defaultButton.disabled).toBe(true);
    expect(plainButton.disabled).toBe(false);
  });

  it("印の無いフォームは対象にしない", () => {
    document.body.innerHTML = `<form><button type="submit">送信</button></form>`;

    submit(document.querySelector("form"));
    vi.runAllTimers();

    expect(document.querySelector("button").disabled).toBe(false);
  });

  it("他のハンドラーが送信を取り消したら無効にしない", () => {
    document.body.innerHTML = `<form data-disable-on-submit><button type="submit">送信</button></form>`;
    const form = document.querySelector("form");
    form.addEventListener("submit", (event) => event.preventDefault(), { once: true });

    submit(form);
    vi.runAllTimers();

    expect(document.querySelector("button").disabled).toBe(false);
  });
});

// pageshow はpageshowイベントを発火させる。
// happy-domはPageTransitionEventのpersistedを持たないため、素のEventに値を載せる。
function pageshow(persisted) {
  const event = new Event("pageshow");
  Object.defineProperty(event, "persisted", { value: persisted });
  window.dispatchEvent(event);
}

describe("戻る操作で復元したページ", () => {
  it("この処理が無効にしたボタンだけを元に戻す", () => {
    document.body.innerHTML = `
      <form data-disable-on-submit>
        <button type="submit" id="guarded">送信</button>
        <button type="submit" id="already" disabled>元から無効</button>
      </form>`;

    submit(document.querySelector("form"));
    vi.runAllTimers();
    expect(document.getElementById("guarded").disabled).toBe(true);

    pageshow(true);

    expect(document.getElementById("guarded").disabled).toBe(false);
    expect(document.getElementById("already").disabled).toBe(true);
  });

  it("キャッシュからの復元でなければ何もしない", () => {
    document.body.innerHTML = `<form data-disable-on-submit><button type="submit">送信</button></form>`;

    submit(document.querySelector("form"));
    vi.runAllTimers();
    pageshow(false);

    expect(document.querySelector("button").disabled).toBe(true);
  });
});
