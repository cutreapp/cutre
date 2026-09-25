// 配色を決めるスクリプトのエントリポイント。esbuildがこれをまとめて static/js/theme.js に出力する。
//
// main.js (type="module" で遅延実行される) とは分け、<head> の中で同期的に読み込む。
// 本文を描く前に dark クラスを付け、ライトで一度描いてからダークに切り替わるのを防ぐため。
// インラインのスクリプトにしないのは、将来CSP本体を入れるときに 'unsafe-inline' を求めないようにするため。
import { initializeColorScheme } from "./color-scheme.js";

initializeColorScheme();
