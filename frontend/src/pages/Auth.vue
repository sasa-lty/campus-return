<script setup lang="ts">
import { reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { api, saveLogin, safe, ui } from "../api";
const router = useRouter();
const mode = ref("login");
const recovery = ref("");
const form = reactive({
  username: "",
  password: "",
  displayName: "",
  recoveryCode: "",
});
async function submit() {
  await safe(async () => {
    if (mode.value === "recover") {
      const v = await api("/auth/recover", "POST", form);
      recovery.value = v.recoveryCode;
      mode.value = "recovered";
      return;
    }
    if (mode.value === "register") {
      const v = await api("/auth/register", "POST", form);
      recovery.value = v.recoveryCode;
    }
    const v = await api("/auth/login", "POST", form);
    saveLogin(v);
    if (!recovery.value) router.push(v.user.role === "USER" ? "/" : "/admin");
  });
}
function go() {
  router.push("/");
}
</script>
<template>
  <section class="narrow auth">
    <p class="eyebrow">欢迎参与校园互助</p>
    <h1>
      {{
        mode === "register"
          ? "注册账号"
          : mode === "recover"
            ? "恢复账号"
            : "登录平台"
      }}
    </h1>
    <div v-if="recovery" class="warning">
      <h3>请保存你的恢复码</h3>
      <p>恢复码仅显示一次，用于忘记密码时找回账号。不要分享给其他人。</p>
      <code class="long-code">{{ recovery }}</code
      ><button v-if="mode !== 'recovered'" @click="go">已保存，进入平台</button
      ><button
        v-else
        @click="
          recovery = '';
          mode = 'login';
        "
      >
        返回登录
      </button>
    </div>
    <template v-else
      ><div class="tabs">
        <button :class="{ active: mode === 'login' }" @click="mode = 'login'">
          登录</button
        ><button
          :class="{ active: mode === 'register' }"
          @click="mode = 'register'"
        >
          注册</button
        ><button
          :class="{ active: mode === 'recover' }"
          @click="mode = 'recover'"
        >
          忘记密码
        </button>
      </div>
      <form class="form" @submit.prevent="submit">
        <label
          >用户名<input
            v-model="form.username"
            required
            autocomplete="username"
            pattern="[a-z0-9_]{3,32}" /></label
        ><label v-if="mode === 'register'"
          >显示名称<input
            v-model="form.displayName"
            maxlength="30"
            placeholder="可以使用昵称" /></label
        ><label v-if="mode === 'recover'"
          >恢复码<input
            v-model="form.recoveryCode"
            required
            autocomplete="off" /></label
        ><label
          >{{ mode === "recover" ? "设置新密码" : "密码"
          }}<input
            v-model="form.password"
            type="password"
            required
            :minlength="mode === 'login' ? 1 : 6"
            :autocomplete="
              mode === 'login' ? 'current-password' : 'new-password'
            " /></label
        ><button :disabled="ui.busy">
          {{
            mode === "register"
              ? "注册并登录"
              : mode === "recover"
                ? "恢复账号"
                : "登录"
          }}
        </button>
      </form>
</template
    >
  </section>
</template>
