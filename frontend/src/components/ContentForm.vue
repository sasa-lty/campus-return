<script setup lang="ts">
import { reactive, watch, ref, onUnmounted } from "vue";
import { api, safe, ui, notify } from "../api";
const props = defineProps<{ initial?: any; edit?: boolean }>();
const emit = defineEmits(["submit"]);
const preview = ref("");
function clearPreview() {
  if (preview.value) URL.revokeObjectURL(preview.value);
  preview.value = "";
}
onUnmounted(clearPreview);
function localInput(value: string) {
  const d = new Date(value);
  return new Date(d.getTime() - d.getTimezoneOffset() * 60000)
    .toISOString()
    .slice(0, 16);
}
const form = reactive<any>({
  name: "",
  category: "生活用品",
  description: "",
  location: "",
  foundAt: localInput(new Date().toISOString()),
  question: "",
  answer: "",
  uploadId: "",
  imageHash: "",
  reason: "",
});
watch(
  () => props.initial,
  (v) => {
    if (v)
      Object.assign(form, v, {
        answer: "",
        foundAt: localInput(v.foundAt),
      });
  },
  { immediate: true },
);
async function upload(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0];
  if (!file) return;
  if (
    file.size > 5 * 1024 * 1024 ||
    !["image/jpeg", "image/png", "image/webp"].includes(file.type)
  ) {
    notify("请选择不超过 5 MB 的 JPG、PNG 或 WebP 图片");
    return;
  }
  await safe(async () => {
    const fd = new FormData();
    fd.append("file", file);
    const up = await api("/uploads", "POST", fd);
    form.uploadId = up.id;
    form.imageHash = up.hash;
    clearPreview();
    preview.value = URL.createObjectURL(file);
  });
}
function submit() {
  const { answer, reason, ...content } = form;
  emit("submit", {
    content: { ...content, foundAt: new Date(form.foundAt).toISOString() },
    answer,
    reason,
  });
}
</script>
<template>
  <form class="form" @submit.prevent="submit">
    <h2>物品信息</h2>
    <label
      >物品名称<input
        v-model="form.name"
        required
        minlength="2"
        maxlength="80"
        placeholder="例如：黑色保温杯"
    /></label>
    <div class="two">
      <label
        >分类<select v-model="form.category">
          <option>生活用品</option>
          <option>数码设备</option>
          <option>证件卡片</option>
          <option>书籍文具</option>
          <option>衣物配饰</option>
          <option>其他</option>
        </select></label
      ><label
        >拾到时间<input v-model="form.foundAt" type="datetime-local" required
      /></label>
    </div>
    <label
      >拾到地点<input
        v-model="form.location"
        required
        minlength="2"
        maxlength="100"
        placeholder="校区、楼宇和附近位置" /></label
    ><label
      >公开描述<textarea
        v-model="form.description"
        required
        minlength="5"
        maxlength="2000"
        placeholder="描述外观，但保留只有物主知道的细节用于核验"
      ></textarea>
    </label>
    <div class="upload-box">
      <label
        >图片（可选，最大 5 MB）<input
          type="file"
          accept="image/jpeg,image/png,image/webp"
          @change="upload"
        /><small v-if="form.uploadId"
          >图片已上传。</small
        ></label
      >
      <img
        v-if="preview"
        class="preview"
        :src="preview"
        alt="已选择的物品照片"
      />
    </div>
    <h2>认领核验</h2>
    <label
      >公开核验问题<input
        v-model="form.question"
        required
        minlength="2"
        maxlength="200"
        placeholder="例如：杯底刻了什么？" /></label
    ><label
      >{{ edit ? "参考答案（留空表示保留）" : "私密参考答案"
      }}<input
        v-model="form.answer"
        :required="!edit"
        type="password"
        minlength="2"
        maxlength="500"
        autocomplete="off" /></label
    ><label v-if="edit"
      >修改原因<input v-model="form.reason" required minlength="2"
    /></label>
    <p class="muted">
      答案只供管理员核验，公开详情不会展示。已有待审核认领后不能更换问题和答案。
    </p>
    <button :disabled="ui.busy">{{ edit ? "保存修改" : "提交登记" }}</button>
  </form>
</template>
