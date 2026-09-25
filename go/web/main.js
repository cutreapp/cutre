// CutreのクライアントサイドのJavaScriptバンドルのエントリポイント。
// esbuildがこれをまとめて static/js/main.js に出力する。

// BasecoatのJSコンポーネントをすべて登録し、window.basecoatランタイムを公開する。
// ランタイムはDOMContentLoadedでコンポーネントを初期化し、その後に挿入された要素も監視する。
// 対話コンポーネントはまだ使っていないが、ここで all を読み込んでおけば足すときにこのファイルを触らずに済む。
import "basecoat-css/all";

// htmxを読み込むと window.htmx が定義され、hx-* 属性を持つ要素が処理されるようになる。
import "htmx.org";

import { initializeAuthFormRestore } from "./auth-form-restore.js";
import { initializeCopyButton } from "./copy-button.js";
import { initializeDialogCommand } from "./dialog-command.js";
import { initializeDownloadButton } from "./download-button.js";
import { initializeLocaleChoice } from "./locale-choice.js";
import { initializeShareButton } from "./share-button.js";
import { initializeSubmitGuard } from "./submit-guard.js";

initializeAuthFormRestore();
initializeCopyButton();
initializeDialogCommand();
initializeDownloadButton();
initializeLocaleChoice();
initializeShareButton();
initializeSubmitGuard();
