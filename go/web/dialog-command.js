// ボタンからダイアログを開閉する宣言的な仕組み (Invoker Commands) を、対応していないブラウザで補う処理。
// commandfor でダイアログのidを、command で show-modal か close を指したボタンを押すと、そのダイアログを開閉する。
// 対応しているブラウザではブラウザ自身が開閉するため、何もしない (二重に開閉しない)。

// initializeDialogCommand は未対応のブラウザで開閉の処理を登録する。main.jsから起動時に一度だけ呼ぶ。
export function initializeDialogCommand() {
  if ("commandForElement" in HTMLButtonElement.prototype) return;

  document.addEventListener("click", (event) => {
    if (!(event.target instanceof Element)) return;

    const button = event.target.closest("button[commandfor]");
    if (!button) return;

    const dialog = document.getElementById(button.getAttribute("commandfor"));
    if (!(dialog instanceof HTMLDialogElement)) return;

    switch (button.getAttribute("command")) {
      case "show-modal":
        if (!dialog.open) dialog.showModal();
        break;
      case "close":
        dialog.close();
        break;
    }
  });
}
