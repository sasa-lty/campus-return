<script setup lang="ts">
import { reactive, ref } from "vue";
import { api, notify, label } from "../api";
import BaseDialog from "./BaseDialog.vue";
const props = defineProps<{ kind: string; record: any }>();
const emit = defineEmits<{ close: []; saved: [] }>();
const draft = reactive({
  role: props.record.role,
  status: props.record.status,
  reason: "",
});
const saving = ref(false),
  error = ref(""),
  credential = ref("");
async function save(reset = false) {
  error.value = "";
  if (draft.reason.trim().length < 2) {
    error.value = "请填写至少 2 个字的处理说明";
    return;
  }
  if (saving.value) return;
  if (
    props.kind === "users" &&
    !confirm(
      reset
        ? "重置凭据后该用户所有会话失效。确认继续？"
        : "账号权限或状态变更后，该用户所有会话失效。确认保存？",
    )
  )
    return;
  saving.value = true;
  try {
    if (props.kind === "users") {
      const result = await api(
        "/admin/users/" + props.record.id + (reset ? "/reset" : ""),
        reset ? "POST" : "PATCH",
        reset
          ? { reason: draft.reason }
          : { role: draft.role, status: draft.status, reason: draft.reason },
      );
      if (reset) credential.value = result.temporaryPassword;
    } else if (props.kind === "reports") {
      await api("/admin/reports/" + props.record.id, "PATCH", {
        status: draft.status,
        resolution: draft.reason,
      });
    } else {
      await api(
        "/admin/evaluations/" + encodeURIComponent(props.record.id),
        "PATCH",
        { status: draft.status, reason: draft.reason },
      );
    }
    emit("saved");
    notify("处理结果已保存");
    if (!reset) emit("close");
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    saving.value = false;
  }
}
</script>
<template>
  <BaseDialog
    compact
    :title="
      kind === 'users'
        ? '管理用户：' + record.username
        : kind === 'reports'
          ? '处理举报与争议'
          : '评价治理'
    "
    :busy="saving"
    @close="$emit('close')"
  >
    <div v-if="credential" class="warning">
      <h3>临时密码仅显示本次</h3>
      <code class="break">{{ credential }}</code>
      <p>请将临时密码安全交给该用户。首次登录后需要修改密码。</p>
      <button @click="$emit('close')">已保存，关闭</button>
    </div>
    <form v-else class="form" @submit.prevent="save()">
      <template v-if="kind === 'users'"
        ><p class="muted">
          {{ record.displayName }} ·
          更改后，此账号需重新登录。
        </p>
        <label
          >角色<select v-model="draft.role" aria-label="角色">
            <option value="USER">用户</option>
            <option value="ADMIN">业务管理员</option>
            <option value="SUPER_ADMIN">超级管理员</option>
          </select></label
        ><label
          >账号状态<select
            v-model="draft.status"
            :aria-label="kind === 'users' ? '账号状态' : '展示状态'"
          >
            <option value="ACTIVE">启用</option>
            <option value="DISABLED">禁用</option>
          </select></label
        ></template
      >
      <template v-else-if="kind === 'reports'"
        ><p>{{ record.reason }}</p>
        <p class="muted">当前状态：{{ label(record.status) }}</p>
        <label
          >处理进度<select
            v-model="draft.status"
            required
            aria-label="处理进度"
          >
            <option value="OPEN" disabled>待受理</option>
            <option value="IN_PROGRESS">处理中</option>
            <option value="RESOLVED">已结案</option>
          </select></label
        ></template
      >
      <template v-else
        ><p>{{ "★".repeat(record.stars) }} {{ record.text }}</p>
        <label
          >展示状态<select
            v-model="draft.status"
            :aria-label="kind === 'users' ? '账号状态' : '展示状态'"
          >
            <option value="PUBLIC">公开</option>
            <option value="HIDDEN">隐藏</option>
          </select></label
        ></template
      >
      <label
        >{{ kind === "reports" ? "处理意见" : "操作原因"
        }}<textarea
          v-model="draft.reason"
          minlength="2"
          maxlength="300"
          required
          placeholder="说明本次操作的依据，保存后将留存记录"
        />
      </label>
      <p v-if="error" class="request-error" role="alert">{{ error }}</p>
      <div class="settings-actions">
        <button :disabled="saving">
          {{ saving ? "保存中…" : "保存处理结果" }}</button
        ><button
          v-if="kind === 'users'"
          type="button"
          class="danger"
          :disabled="saving"
          @click="save(true)"
        >
          重置登录凭据
        </button>
      </div>
    </form>
  </BaseDialog>
</template>
