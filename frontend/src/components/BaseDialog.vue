<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref } from "vue";
defineProps<{ title: string; busy?: boolean; compact?: boolean }>();
const emit = defineEmits<{ close: [] }>();
const dialog = ref<HTMLElement | null>(null);
const restore = document.activeElement as HTMLElement | null;
// 弹窗类名使用应用专属前缀，避免与浏览器注入的通用 modal 规则冲突。
// 沿用原 activateModal / closeModal 的焦点循环、Esc 关闭与焦点恢复。
function keydown(e: KeyboardEvent) {
  if (e.key === "Escape") {
    e.preventDefault();
    emit("close");
    return;
  }
  if (e.key !== "Tab" || !dialog.value) return;
  const nodes = [
    ...dialog.value.querySelectorAll<HTMLElement>(
      'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])',
    ),
  ].filter((el) => !el.hidden && el.offsetParent !== null);
  if (!nodes.length) {
    e.preventDefault();
    dialog.value.focus();
    return;
  }
  const first = nodes[0]!,
    last = nodes[nodes.length - 1]!;
  if (e.shiftKey && document.activeElement === first) {
    e.preventDefault();
    last.focus();
  } else if (!e.shiftKey && document.activeElement === last) {
    e.preventDefault();
    first.focus();
  }
}
onMounted(async () => {
  document.body.classList.add("cr-dialog-active");
  document.addEventListener("keydown", keydown);
  await nextTick();
  (
    dialog.value?.querySelector<HTMLElement>("[data-dialog-close]") ||
    dialog.value
  )?.focus();
});
onUnmounted(() => {
  document.body.classList.remove("cr-dialog-active");
  document.removeEventListener("keydown", keydown);
  if (restore?.isConnected) restore.focus();
});
</script>
<template>
  <Teleport to="body">
    <div class="cr-dialog-overlay" @click.self="emit('close')">
      <section
        ref="dialog"
        class="cr-dialog"
        :class="{ 'cr-dialog-compact': compact }"
        role="dialog"
        aria-modal="true"
        :aria-label="title"
        :aria-busy="busy"
        tabindex="-1"
      >
        <div class="cr-dialog-heading">
          <h2>{{ title }}</h2>
          <button
            type="button"
            class="close"
            data-dialog-close
            aria-label="关闭详情"
            @click="emit('close')"
          >
            ×
          </button>
        </div>
        <div class="cr-dialog-content"><slot /></div>
      </section>
    </div>
  </Teleport>
</template>
