// フォームの二重送信を防ぐ処理。data-disable-on-submit を持つフォームが送信されたら、送信ボタンを無効にする。
// 応答を待つ間の連打で同じ送信が重なると、ログインのように試行を数える操作では1回の操作で枠を余分に使ってしまう。
// JavaScriptが無効でもフォームはそのまま送信できる (二重送信を防げないことだけが違う)。

// 無効にしたボタンの印。戻る操作で復元したページで、この処理が無効にしたボタンだけを元に戻すのに使う。
const DISABLED_MARK = "submitGuardDisabled";

// initializeSubmitGuard は送信ボタンを無効にする処理を登録する。main.jsから起動時に一度だけ呼ぶ。
export function initializeSubmitGuard() {
  // documentに委譲するのは、後から差し込まれたフォームにもハンドラーを張り直さずに効かせるため。
  document.addEventListener("submit", (event) => {
    const form = event.target;
    if (!(form instanceof HTMLFormElement) || !form.hasAttribute("data-disable-on-submit")) return;

    // 無効にするのは送信が始まった後にする。送信の途中で押されたボタンを無効にすると、
    // そのボタンのname / valueが送信する値から外れる。
    // 他のハンドラーが送信を取り消した場合はボタンを残し、利用者がもう一度送れるようにする。
    setTimeout(() => {
      if (event.defaultPrevented) return;

      for (const element of form.elements) {
        if (element.type !== "submit" || element.disabled) continue;

        element.disabled = true;
        element.dataset[DISABLED_MARK] = "";
      }
    }, 0);
  });

  // ブラウザの戻る操作でBFCacheから復元したページは、無効にした状態のままのボタンを持つ。
  // そのままでは戻った先のフォームを送れないため、この処理が無効にしたボタンを元に戻す。
  window.addEventListener("pageshow", (event) => {
    if (!event.persisted) return;

    for (const element of document.querySelectorAll("[data-submit-guard-disabled]")) {
      element.disabled = false;
      delete element.dataset[DISABLED_MARK];
    }
  });
}
