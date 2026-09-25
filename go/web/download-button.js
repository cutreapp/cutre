// 文字列をテキストファイルとして保存させるボタンの処理。data-download-text を持つボタンを押すと、
// その値を data-download-filename の名前のファイルとしてダウンロードさせる。
// ボタンはサーバーが hidden で描き、ファイルを作って保存させられるブラウザでだけ表示する。
// 使えないボタンを押させないためで、JavaScriptが無効なときは画面の文字を手で書き写す。

// initializeDownloadButton は保存のボタンを表示し、押されたときの処理を登録する。main.jsから起動時に一度だけ呼ぶ。
export function initializeDownloadButton() {
  if (typeof URL.createObjectURL !== "function" || !("download" in HTMLAnchorElement.prototype)) return;

  for (const button of document.querySelectorAll("[data-download-text]")) {
    button.hidden = false;
  }

  document.addEventListener("click", (event) => {
    if (!(event.target instanceof Element)) return;

    const button = event.target.closest("[data-download-text]");
    if (!button) return;

    // サーバーへ取りに行かず、手元の文字列からファイルを作る。保存する内容をもう一度送らずに済むため。
    const blob = new Blob([button.dataset.downloadText], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = button.dataset.downloadFilename;
    link.click();
    // ダウンロードの開始はブラウザによって非同期のため、同じタスクで解放すると保存に失敗することがある。
    setTimeout(() => URL.revokeObjectURL(url), 0);
  });
}
