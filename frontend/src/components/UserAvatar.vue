<script setup lang="ts">
import { computed } from "vue";
const props = withDefaults(defineProps<{ name?: string; small?: boolean }>(), {
  name: "用户",
});
const colors = ["#e6d9c7", "#dce2d1", "#e5d1cc", "#d6e0e3", "#e2d9e6"];
const color = computed(
  () =>
    colors[
      Array.from(props.name).reduce((n, c) => n + c.charCodeAt(0), 0) %
        colors.length
    ],
);
</script>
<template>
  <span
    class="cr-avatar"
    :class="{ 'cr-avatar-small': small }"
    :style="{ background: color }"
    aria-hidden="true"
    >{{ Array.from(name)[0] || "人" }}</span
  >
</template>
