import { createApp } from "vue";
import { createRouter, createWebHashHistory } from "vue-router";
import App from "./App.vue";
import { ui, notify } from "./api";
import Hall from "./pages/Hall.vue";
import "./style.css";
import "./styles/original-foundation.css";
import "./styles/vue-adapter.css";
import "./styles/workspace.css";
import "./styles/editorial.css";
import "./styles/showcase.css";
const router = createRouter({
  history: createWebHashHistory(),
  scrollBehavior: async (to, _from, saved) => {
    if (saved) return saved;
    if (to.hash) {
      await new Promise(resolve => requestAnimationFrame(resolve));
      return { el: to.hash, top: 110, behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth" };
    }
    return { left: 0, top: 0 };
  },
  routes: [
    { path: "/", component: Hall },
    { path: "/login", component: () => import("./pages/Auth.vue") },
    { path: "/publish", component: () => import("./pages/Publish.vue") },
    { path: "/items/:id", component: () => import("./pages/Detail.vue") },
    { path: "/me", component: () => import("./pages/Me.vue") },
    { path: "/admin", component: () => import("./pages/Admin.vue") },
    { path: "/share", component: () => import("./pages/Share.vue") },
  ],
});
// 长时间打开的页面可能跨越发布；资源失效时给出可操作提示，不静默丢失导航。
window.addEventListener("vite:preloadError", (event) => {
  event.preventDefault();
  ui.reloadRequired = true;
});
router.onError((error) => {
  if (
    /dynamically imported module|Importing a module script failed|error loading dynamically imported module/i.test(
      error.message,
    )
  )
    ui.reloadRequired = true;
  else notify("页面打开失败，请重试。");
});
createApp(App).use(router).mount("#app");
