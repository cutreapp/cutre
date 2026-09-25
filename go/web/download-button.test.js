import { afterEach, describe, expect, it, vi } from "vitest";

import { initializeDownloadButton } from "./download-button.js";

function render() {
  document.body.innerHTML = `
    <button type="button" data-download-text="abcd-efgh\njkmn-pqrs\n" data-download-filename="cutre-recovery-codes.txt" hidden>
      <span>ファイルに保存</span>
    </button>`;
  return document.querySelector("button");
}

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("ファイルを作れないブラウザ", () => {
  it("ボタンを隠したままにする", () => {
    vi.stubGlobal(
      "URL",
      class extends URL {
        static createObjectURL = undefined;
      },
    );
    const button = render();

    initializeDownloadButton();

    expect(button.hidden).toBe(true);
  });
});

describe("ファイルを作れるブラウザ", () => {
  it("ボタンを表示し、押すと値をファイル名付きで保存させる", async () => {
    vi.useFakeTimers();
    const createObjectURL = vi.fn(() => "blob:cutre");
    const revokeObjectURL = vi.fn();
    vi.stubGlobal(
      "URL",
      class extends URL {
        static createObjectURL = createObjectURL;
        static revokeObjectURL = revokeObjectURL;
      },
    );
    const clicked = [];
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function () {
      clicked.push({ href: this.href, download: this.download });
    });
    const button = render();

    initializeDownloadButton();
    expect(button.hidden).toBe(false);

    button.querySelector("span").dispatchEvent(new MouseEvent("click", { bubbles: true }));

    expect(clicked).toEqual([{ href: "blob:cutre", download: "cutre-recovery-codes.txt" }]);
    const blob = createObjectURL.mock.calls[0][0];
    expect(await blob.text()).toBe("abcd-efgh\njkmn-pqrs\n");
    expect(blob.type).toBe("text/plain;charset=utf-8");
    // 解放はダウンロードを始めたタスクの後に回す。
    expect(revokeObjectURL).not.toHaveBeenCalled();
    vi.runAllTimers();
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:cutre");
  });
});
