<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { BrowserQRCodeReader } from "@zxing/browser";
const emit = defineEmits(["result", "close"]);
const video = ref<HTMLVideoElement>();
const error = ref("");
let controls: any;
onMounted(async () => {
  try {
    controls = await new BrowserQRCodeReader().decodeFromVideoDevice(
      undefined,
      video.value!,
      (result) => {
        if (result) {
          emit("result", result.getText());
          controls?.stop();
        }
      },
    );
  } catch {
    error.value = "无法打开摄像头，请允许摄像头访问，或使用手动输入交接码。";
  }
});
onUnmounted(() => controls?.stop());
</script>
<template>
  <div class="panel">
    <video ref="video" class="scanner" muted playsinline></video>
    <p v-if="error" class="warning">{{ error }}</p>
    <button class="secondary" @click="emit('close')">关闭扫描</button>
  </div>
</template>
