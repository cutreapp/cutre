import { beforeEach, describe, expect, it, vi } from "vitest";

import { initializeShareButton } from "./share-button.js";

const share = vi.fn();

// setShare はブラウザの共有のAPIを差し替える。undefinedを渡すと、APIを持たないブラウザにする。
function setShare(fn) {
  Object.defineProperty(navigator, "share", { value: fn, configurable: true });
}

function render() {
  document.body.innerHTML = `
    <button type="button" data-share-url="https://cutre.example.com/i/token" data-share-title="Cutreへの招待" hidden>
      <svg></svg>URLを送る
    </button>`;
  return document.querySelector("button");
}

describe("共有のAPIを持たないブラウザ", () => {
  it("ボタンを隠したままにする", () => {
    setShare(undefined);
    const button = render();

    initializeShareButton();

    expect(button.hidden).toBe(true);
  });
});

describe("共有のAPIを持つブラウザ", () => {
  // ハンドラーはdocumentへ一度だけ登録するため、初期化はこのdescribeの最初のテストで行う。
  let initialized = false;

  beforeEach(() => {
    share.mockReset();
    setShare(share);
  });

  function setup() {
    const button = render();
    if (!initialized) {
      initializeShareButton();
      initialized = true;
    } else {
      button.hidden = false;
    }
    return button;
  }

  it("ボタンを表示し、押すと題名とURLを共有の画面へ渡す", () => {
    share.mockResolvedValue(undefined);
    const button = setup();
    expect(button.hidden).toBe(false);

    button.querySelector("svg").dispatchEvent(new MouseEvent("click", { bubbles: true }));

    expect(share).toHaveBeenCalledWith({ title: "Cutreへの招待", url: "https://cutre.example.com/i/token" });
  });

  // 拒否を受け止めなければ、処理されない拒否としてテストの実行そのものが失敗する。
  it("共有の画面を閉じられても、拒否を処理されないまま残さない", async () => {
    share.mockRejectedValue(new DOMException("閉じた", "AbortError"));
    const button = setup();

    button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(share).toHaveBeenCalledOnce();
  });
});
