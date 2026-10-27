<script setup lang="ts">
import UserAvatar from "./components/UserAvatar.vue";
import { onMounted, ref, watch } from "vue";
import { useRouter, useRoute } from "vue-router";
import { api, session, ui, clearLogin, safe, label } from "./api";
const router = useRouter();
const route = useRoute();
const menuOpen = ref(false);
function skipContent() {
  document.getElementById("page-content")?.focus();
}
watch(
  () => route.fullPath,
  () => {
    menuOpen.value = false;
  },
);
onMounted(() => {
  if (session.token)
    safe(
      async () => {
        session.user = await api("/me");
      },
      { busy: false },
    );
});
function reloadPage() {
  window.location.reload();
}
async function logout() {
  await safe(() => api("/auth/logout", "POST", {}));
  clearLogin();
  router.push("/login");
}
</script>
<template>
  <a class="skip-link" href="#page-content" @click.prevent="skipContent"
    >跳到主要内容</a
  >
  <header class="topbar">
    <div class="container nav-wrap">
      <RouterLink to="/" class="brand"
        >校园失物招领平台</RouterLink
      >
      <button
        class="secondary mobile-menu"
        :aria-expanded="menuOpen"
        aria-controls="primary-nav"
        @click="menuOpen = !menuOpen"
      >
        {{ menuOpen ? "收起导航" : "菜单" }}
      </button>
      <nav
        id="primary-nav"
        class="nav"
        :class="{ 'is-open': menuOpen }"
        aria-label="主要导航"
      >
        <RouterLink to="/" :class="{ 'home-active': route.path === '/' && !route.hash }">首页</RouterLink
        ><RouterLink :to="{ path: '/', hash: '#hall' }" :class="{ 'hall-active': route.path === '/' && route.hash === '#hall' }">招领大厅</RouterLink
        ><RouterLink to="/share">手机访问</RouterLink
        ><RouterLink v-if="session.user?.role === 'USER'" to="/publish"
          >登记拾物</RouterLink
        ><RouterLink v-if="session.user" to="/me">个人中心</RouterLink
        ><RouterLink
          v-if="
            session.user?.role === 'ADMIN' ||
            session.user?.role === 'SUPER_ADMIN'
          "
          to="/admin"
          >管理工作台</RouterLink
        ><RouterLink v-if="!session.user" to="/login">登录 / 注册</RouterLink>
      </nav>
      <div v-if="session.user" class="nav-user">
        <RouterLink to="/me" class="user-chip"
          ><UserAvatar
            :name="session.user.displayName || session.user.username"
            small
          />
          <span class="user-chip-info"
            ><strong>{{
              session.user.displayName || session.user.username
            }}</strong
            ><span class="role cr-role-badge">{{
              label(session.user.role)
            }}</span></span
          ></RouterLink
        >
        <button class="plain" @click="logout">退出</button>
      </div>
    </div>
  </header>
  <main
    id="page-content"
    class="app-shell"
    :class="{ 'is-admin': route.path === '/admin', 'home-shell': route.path === '/' }"
    tabindex="-1"
  >
    <div v-if="ui.reloadRequired" class="warning" role="alert">
      页面已更新，请重新加载后继续。未提交的内容请先保存。
      <button class="secondary" @click="reloadPage">重新加载页面</button>
    </div>
    <div v-if="session.user?.forcePassword" class="warning">
      请先到个人中心修改临时密码，其他操作暂不可用。
    </div>
    <div v-if="ui.message" role="status" class="toast" @click="ui.message = ''">
      {{ ui.message }}
    </div>
    <RouterView :key="$route.path" />
  </main>
  <footer class="footer">
    <div class="container footer-inner">
      <span class="footer-brand">校园失物招领平台</span
      ><span>
        {{
          session.user
            ? session.user.displayName + " / " + label(session.user.role)
            : "浏览无需登录，认领需注册"
        }}</span
      ><span>请勿公开上传证件号码、电话号码等个人信息。</span>
    </div>
  </footer>
</template>
