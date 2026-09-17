// 利用者が明示的に選んだ言語を記録する処理。Cookieを1つ書き、サーバーが案内の判定で
// ブラウザの言語設定より優先する。記録するだけで移動はリンク自身に任せるため、
// JavaScriptが無効でも言語版の移動はできる (案内が出続けることだけが違う)。
// Cookie名は internal/middleware/i18n.go の SelectedLocaleCookie と揃える。
const SELECTED_LOCALE_COOKIE = "selected_locale";
const SELECTED_LOCALE_MAX_AGE = 60 * 60 * 24 * 365;

// initializeLocaleChoice は言語選択を記録し、閉じる操作では案内を取り除く処理を登録する。
// main.jsから起動時に一度だけ呼ぶ。
export function initializeLocaleChoice() {
  // 書き込み口は言語スイッチャーのリンク、案内の移動リンク、案内の閉じるボタンの3つで、
  // いずれもdata-locale-choiceに記録するロケールを持つ。
  // documentに委譲するのは、案内やスイッチャーがOOBスワップなどで差し替わっても
  // ハンドラーを張り直さずに済むため。
  document.addEventListener("click", (event) => {
    if (!(event.target instanceof Element)) return;

    const chooser = event.target.closest("[data-locale-choice]");
    if (!chooser) return;

    const locale = chooser.dataset.localeChoice;
    if (!locale) return;

    document.cookie = `${SELECTED_LOCALE_COOKIE}=${locale}; path=/; max-age=${SELECTED_LOCALE_MAX_AGE}; samesite=lax; secure`;

    // 閉じるボタンは移動を伴わないため、記録に加えてその場で案内を取り除く。
    if (!("localeSuggestionDismiss" in chooser.dataset)) return;

    const suggestion = chooser.closest("[data-locale-suggestion]");

    // 押したボタンごと案内を取り除くため、取り除く前にフォーカスを本文へ移す。
    // 移さないとフォーカスがbodyに落ち、キーボードの利用者は次のTabがページ先頭へ戻る。
    document.getElementById("main")?.focus();

    suggestion?.remove();
  });
}
