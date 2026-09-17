import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    // Cookieとフォーカスを扱うためDOM環境を使う。Secure Cookieを検証できるHTTPSのURLにする。
    environment: "happy-dom",
    include: ["web/**/*.test.js"],
    environmentOptions: {
      happyDOM: {
        url: "https://cutre.example.com/",
        settings: {
          // リンクの既定動作を妨げていないか検証しつつ、テストから外部への遷移は行わない。
          navigation: { disableMainFrameNavigation: true, disableFallbackToSetURL: true },
        },
      },
    },
  },
});
