import { beforeEach, describe, expect, it } from "vitest";

import { initializeColorScheme } from "./color-scheme.js";

// fakeQuery は matchMedia の返り値の代わり。change を発火させて、設定の変更を再現する。
function fakeQuery(matches) {
  const target = new EventTarget();
  return {
    matches,
    addEventListener: (type, listener) => target.addEventListener(type, listener),
    change(nextMatches) {
      this.matches = nextMatches;
      const event = new Event("change");
      event.matches = nextMatches;
      target.dispatchEvent(event);
    },
  };
}

beforeEach(() => {
  document.documentElement.className = "";
});

describe("initializeColorScheme", () => {
  it("端末がダークなら dark クラスを付ける", () => {
    initializeColorScheme(document.documentElement, fakeQuery(true));

    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("端末がライトなら dark クラスを付けない", () => {
    initializeColorScheme(document.documentElement, fakeQuery(false));

    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });

  it("表示中に端末の設定が変わったら追従する", () => {
    const query = fakeQuery(false);
    initializeColorScheme(document.documentElement, query);

    query.change(true);
    expect(document.documentElement.classList.contains("dark")).toBe(true);

    query.change(false);
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });
});
