import { createApp } from "vue";
import App from "./App.vue";
import { installMouseNavigation } from "./state/mouseNav";
import { hydrateWorkspace } from "./state/workspace";
import "./style.css";

async function bootstrap(): Promise<void> {
  const plugin = window.dbxPlugin;
  if (plugin) {
    try {
      await plugin.ready;
    } catch {
      // 宿主未就绪时仍然渲染，后续调用会给出可读错误。
    }
    document.documentElement.dataset.theme = plugin.theme?.appearance === "dark" ? "dark" : "light";
  }

  await hydrateWorkspace();
  createApp(App).mount("#app");
  installMouseNavigation();
}

void bootstrap();
