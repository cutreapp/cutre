// 文字列をクリップボードにコピーするボタンの処理。data-copy-text を持つボタンを押すと、その値をコピーする。
// ボタンはサーバーが hidden で描き、クリップボードへ書き込めるブラウザでだけ表示する。
// 使えないボタンを押させないためで、JavaScriptが無効なときは隣の入力欄から手で選んでコピーする。

// コピーしたことを伝える文言を、元の文言へ戻すまでの時間。
const COPIED_LABEL_DURATION = 2000;

// initializeCopyButton はコピーのボタンを表示し、押されたときの処理を登録する。main.jsから起動時に一度だけ呼ぶ。
export function initializeCopyButton() {
  // クリップボードのAPIは安全なコンテキスト (HTTPS) でだけ定義される。
  if (!navigator.clipboard?.writeText) return;

  for (const button of document.querySelectorAll("[data-copy-text]")) {
    button.hidden = false;
  }

  const resetTimers = new WeakMap();

  document.addEventListener("click", async (event) => {
    if (!(event.target instanceof Element)) return;

    const button = event.target.closest("[data-copy-text]");
    if (!button) return;

    try {
      await navigator.clipboard.writeText(button.dataset.copyText);
    } catch {
      // 利用者が許可しなかったときなど。文言を変えず、コピーできたと誤って伝えない。
      return;
    }

    // 文言はボタンの中のライブリージョンに入れ、フォーカスがボタンにあっても読み上げられるようにする。
    const label = button.querySelector("[aria-live]") ?? button;
    if (!("originalLabel" in label.dataset)) {
      label.dataset.originalLabel = label.textContent;
    }
    label.textContent = button.dataset.copiedLabel;

    // 続けて押されたときは、最後に押してから数え直す。
    clearTimeout(resetTimers.get(button));
    resetTimers.set(
      button,
      setTimeout(() => {
        label.textContent = label.dataset.originalLabel;
      }, COPIED_LABEL_DURATION),
    );
  });
}
