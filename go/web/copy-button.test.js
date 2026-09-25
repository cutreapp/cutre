import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { initializeCopyButton } from "./copy-button.js";

const writeText = vi.fn();

// setClipboard はブラウザのクリップボードのAPIを差し替える。undefinedを渡すと、APIを持たないブラウザにする。
function setClipboard(clipboard) {
  Object.defineProperty(navigator, "clipboard", { value: clipboard, configurable: true });
}

function render() {
  document.body.innerHTML = `
    <input value="https://cutre.example.com/i/token" readonly>
    <button type="button" data-copy-text="https://cutre.example.com/i/token" data-copied-label="コピーしました" hidden>
      <span aria-live="polite">コピー</span>
    </button>`;
  return document.querySelector("button");
}

// click はボタンを押し、クリップボードへの書き込みを待つ。
async function click(button) {
  button.querySelector("span").dispatchEvent(new MouseEvent("click", { bubbles: true }));
  await vi.waitFor(() => expect(writeText).toHaveBeenCalled());
  await Promise.resolve();
}

describe("クリップボードのAPIを持たないブラウザ", () => {
  it("ボタンを隠したままにする", () => {
    setClipboard(undefined);
    const button = render();

    initializeCopyButton();

    expect(button.hidden).toBe(true);
  });
});

describe("クリップボードのAPIを持つブラウザ", () => {
  // ハンドラーはdocumentへ一度だけ登録するため、初期化はこのdescribeの最初のテストで行う。
  let initialized = false;

  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    writeText.mockReset();
    writeText.mockResolvedValue(undefined);
    setClipboard({ writeText });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function setup() {
    const button = render();
    if (!initialized) {
      initializeCopyButton();
      initialized = true;
    } else {
      button.hidden = false;
    }
    return button;
  }

  it("ボタンを表示し、押すとURLをコピーして、しばらくしたら文言を戻す", async () => {
    const button = setup();
    expect(button.hidden).toBe(false);

    await click(button);

    expect(writeText).toHaveBeenCalledWith("https://cutre.example.com/i/token");
    expect(button.textContent.trim()).toBe("コピーしました");

    vi.advanceTimersByTime(2000);
    expect(button.textContent.trim()).toBe("コピー");
  });

  it("書き込みを拒まれたときは、コピーしたと伝えない", async () => {
    writeText.mockRejectedValue(new Error("拒否"));
    const button = setup();

    await click(button);

    expect(button.textContent.trim()).toBe("コピー");
  });
});
