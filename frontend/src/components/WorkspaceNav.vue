<script setup lang="ts">
import UserAvatar from "./UserAvatar.vue";
defineProps<{
  title: string;
  name: string;
  role: string;
  active: string;
  items: { key: string; label: string; group: string; badge?: number }[];
}>();
defineEmits<{ select: [key: string] }>();
</script>
<template>
  <aside class="workspace-nav">
    <div class="workspace-identity">
      <UserAvatar :name="name" />
      <div>
        <strong>{{ name }}</strong
        ><span class="cr-role-badge">{{ role }}</span>
      </div>
    </div>
    <nav class="workspace-menu" :aria-label="title">
      <template v-for="(item, index) in items" :key="item.key">
        <h2
          v-if="index === 0 || items[index - 1]?.group !== item.group"
          class="nav-group"
        >
          {{ item.group }}
        </h2>
        <button
          :class="{ active: active === item.key }"
          :aria-label="item.label"
          :aria-current="active === item.key ? 'page' : undefined"
          @click="$emit('select', item.key)"
        >
          <span>{{ item.label }}</span
          ><span v-if="item.badge" class="nav-count">{{
            item.badge > 99 ? "99+" : item.badge
          }}</span>
        </button>
      </template>
    </nav>
    <div class="workspace-nav-footer"><slot /></div>
  </aside>
</template>
