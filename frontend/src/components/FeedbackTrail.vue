<script setup lang="ts">
import { label, time } from "../api";
defineProps<{ entries: any[]; compact?: boolean }>();
</script>
<template>
  <div
    v-if="entries?.length"
    class="feedback-trail"
    :class="{ 'is-compact': compact }"
  >
    <article v-for="f in compact ? entries.slice(0, 1) : entries" :key="f.id">
      <div>
        <strong>{{ label(f.status || f.action) }}</strong
        ><time>{{ time(f.at) }}</time>
      </div>
      <p>{{ f.reason || "历史原因未留存" }}</p>
    </article>
  </div>
</template>
