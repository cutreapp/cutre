// 端末の配色の設定 (prefers-color-scheme) に合わせて、html要素の dark クラスを付け外しする処理。
// Basecoatの dark: バリアントと tokens.css のダークの値は、html.dark を基準にしている。
// JavaScriptが無効な環境では dark クラスが付かず、ライトで表示する。

const DARK_QUERY = "(prefers-color-scheme: dark)";

// initializeColorScheme は今の設定を反映し、表示中に設定が変わったときも追従させる。theme.jsから一度だけ呼ぶ。
export function initializeColorScheme(root = document.documentElement, query = window.matchMedia(DARK_QUERY)) {
  root.classList.toggle("dark", query.matches);
  query.addEventListener("change", (event) => {
    root.classList.toggle("dark", event.matches);
  });
}
