// URLを端末の共有機能 (Web Share API) で送るボタンの処理。data-share-url を持つボタンを押すと、共有の画面を開く。
// ボタンはサーバーが hidden で描き、共有機能を持つブラウザでだけ表示する。
// 共有機能を持たない環境では、同じURLをコピーのボタンや入力欄から渡す。

// initializeShareButton は共有のボタンを表示し、押されたときの処理を登録する。main.jsから起動時に一度だけ呼ぶ。
export function initializeShareButton() {
  if (typeof navigator.share !== "function") return;

  for (const button of document.querySelectorAll("[data-share-url]")) {
    button.hidden = false;
  }

  document.addEventListener("click", async (event) => {
    if (!(event.target instanceof Element)) return;

    const button = event.target.closest("[data-share-url]");
    if (!button) return;

    try {
      await navigator.share({ title: button.dataset.shareTitle, url: button.dataset.shareUrl });
    } catch {
      // 利用者が共有の画面を閉じたときも拒否として届く。知らせることは無いため何もしない。
    }
  });
}
