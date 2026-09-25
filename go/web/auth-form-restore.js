// BFCacheから認証フォームが復元されたとき、離れる前の入力内容を残さない。
// no-storeはHTTPキャッシュを制御するが、BFCacheからの除外は保証しない。

// initializeAuthFormRestore は入力内容を消す処理を登録する。main.jsから起動時に一度だけ呼ぶ。
export function initializeAuthFormRestore() {
  window.addEventListener("pageshow", (event) => {
    if (!event.persisted) return;

    for (const form of document.querySelectorAll("form[data-clear-on-history-restore]")) {
      form.reset();
    }
  });
}
