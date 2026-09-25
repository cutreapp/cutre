import { afterEach, beforeAll, describe, expect, it } from "vitest";

import { initializeDialogCommand } from "./dialog-command.js";

function render() {
  document.body.innerHTML = `
    <button type="button" commandfor="confirm-dialog" command="show-modal"><svg></svg>作り直す</button>
    <dialog id="confirm-dialog">
      <button type="button" commandfor="confirm-dialog" command="close">キャンセル</button>
    </dialog>`;
  const [open, close] = document.querySelectorAll("button");
  return { open, close, dialog: document.querySelector("dialog") };
}

describe("Invoker Commandsに対応していないブラウザ", () => {
  // ハンドラーはdocumentへ一度だけ登録するため、初期化はこのdescribeの最初に1回だけ行う。
  beforeAll(() => {
    initializeDialogCommand();
  });

  afterEach(() => {
    document.querySelector("dialog")?.close();
  });

  it("show-modal のボタンを押すとダイアログをモーダルで開き、close のボタンで閉じる", () => {
    const { open, close, dialog } = render();

    open.querySelector("svg").dispatchEvent(new MouseEvent("click", { bubbles: true }));
    expect(dialog.open).toBe(true);

    close.click();
    expect(dialog.open).toBe(false);
  });

  it("開いているダイアログを開き直そうとしても例外を出さない", () => {
    const { open, dialog } = render();

    open.click();
    expect(() => open.click()).not.toThrow();
    expect(dialog.open).toBe(true);
  });

  it("commandfor がダイアログ以外を指していれば何もしない", () => {
    document.body.innerHTML = `
      <button type="button" commandfor="not-dialog" command="show-modal">開く</button>
      <div id="not-dialog"></div>`;

    expect(() => document.querySelector("button").click()).not.toThrow();
  });
});

describe("Invoker Commandsに対応しているブラウザ", () => {
  afterEach(() => {
    delete HTMLButtonElement.prototype.commandForElement;
  });

  it("ブラウザに開閉を任せ、処理を登録しない", () => {
    Object.defineProperty(HTMLButtonElement.prototype, "commandForElement", { value: null, configurable: true });
    const listeners = [];
    const original = document.addEventListener;
    document.addEventListener = (...args) => listeners.push(args);

    try {
      initializeDialogCommand();
    } finally {
      document.addEventListener = original;
    }

    expect(listeners).toEqual([]);
  });
});
