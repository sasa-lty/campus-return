<script setup lang="ts">
import { reactive, ref, computed } from "vue";
import { useRouter } from "vue-router";
import { api, session, clearLogin, notify } from "../api";
const router = useRouter();
const profile = reactive({ displayName: session.user?.displayName || "" });
const password = reactive({ oldPassword: "", password: "", confirmation: "" });
const saving = ref(""),
  error = ref("");
const dirty = computed(
  () => profile.displayName.trim() !== session.user?.displayName,
);
async function saveProfile() {
  if (saving.value || !dirty.value) return;
  saving.value = "profile";
  error.value = "";
  try {
    session.user = await api("/me", "PATCH", {
      displayName: profile.displayName.trim(),
    });
    notify("资料已更新");
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    saving.value = "";
  }
}
async function changePassword() {
  error.value = "";
  if (password.password !== password.confirmation) {
    error.value = "两次输入的新密码不一致";
    return;
  }
  if (saving.value || !confirm("修改密码后，所有设备需要重新登录。确认修改？"))
    return;
  saving.value = "password";
  try {
    await api("/me/password", "POST", {
      oldPassword: password.oldPassword,
      password: password.password,
    });
    clearLogin();
    router.push("/login");
    notify("密码已修改，请重新登录");
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    saving.value = "";
  }
}
</script>
<template>
  <p v-if="session.user?.forcePassword" class="warning">
    当前使用临时密码，请先修改密码后继续操作。
  </p>
  <p v-if="error" class="request-error" role="alert">{{ error }}</p>
  <section
    v-if="!session.user?.forcePassword"
    class="workspace-panel settings-section"
  >
    <div>
      <h2>基本资料</h2>
      <p class="muted">显示名称用于发布、认领和交接记录。</p>
    </div>
    <form class="form" @submit.prevent="saveProfile">
      <label>用户名<input :value="session.user?.username" disabled /></label>
      <label
        >显示名称<input
          v-model="profile.displayName"
          required
          maxlength="30"
          autocomplete="nickname"
      /></label>
      <div class="settings-actions">
        <button :disabled="!!saving || !dirty">
          {{ saving === "profile" ? "保存中…" : "更新资料" }}</button
        ><span class="muted">用户名用于登录，无法修改。</span>
      </div>
    </form>
  </section>
  <section class="workspace-panel settings-section">
    <div>
      <h2>登录与安全</h2>
      <p class="muted">密码至少 6 个字符，修改后需重新登录。</p>
    </div>
    <form class="form" @submit.prevent="changePassword">
      <label
        >当前密码<input
          v-model="password.oldPassword"
          type="password"
          placeholder="原密码或临时密码"
          required
          autocomplete="current-password"
      /></label>
      <label
        >新密码<input
          v-model="password.password"
          type="password"
          placeholder="新密码（至少6个字符）"
          minlength="6"
          required
          autocomplete="new-password"
      /></label>
      <label
        >确认新密码<input
          v-model="password.confirmation"
          type="password"
          required
          autocomplete="new-password"
      /></label>
      <div>
        <button class="secondary" :disabled="!!saving">
          {{ saving === "password" ? "修改中…" : "修改密码并退出登录" }}
        </button>
      </div>
    </form>
  </section>
</template>
