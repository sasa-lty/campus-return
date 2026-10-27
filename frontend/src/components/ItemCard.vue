<script setup lang="ts">
import { label, time } from "../api";
import StatusTag from "./StatusTag.vue";
defineProps<{ item: any }>();
// 由原 itemCard 模板迁移，字段适配数据库内容与账本状态。
const initials: Record<string, string> = {
  数码设备: "数码",
  证件卡片: "卡片",
  书籍文具: "书文",
  生活用品: "生活",
  衣物配饰: "衣饰",
};
</script>
<template>
  <RouterLink
    :to="'/items/' + item.id"
    class="item-card"
    :aria-label="'查看' + item.content.name + '详情'"
  >
    <div class="item-image">
      <img
        v-if="item.content.uploadId"
        :src="'/uploads/' + item.content.uploadId"
        :alt="item.content.name"
        loading="lazy"
        decoding="async"
      />
      <div v-else class="image-placeholder" aria-hidden="true">
        {{ initials[item.content.category] || "拾物" }}
      </div>
      <span class="item-card-arrow" aria-hidden="true">↗</span>
    </div>
    <div class="item-body">
      <div class="item-meta">
        <span class="category">{{ item.content.category }}</span
        ><StatusTag :status="item.status" />
      </div>
      <h3>{{ item.content.name }}</h3>
      <div class="item-desc">
        {{ item.content.description || "暂无补充描述" }}
      </div>
      <div class="item-loc">
        发现地点：{{ item.content.location }}<br />发现时间：{{
          time(item.content.foundAt)
        }}
      </div>
      <div class="item-footer">
        <span>{{ label(item.visibility) }}记录</span><span>查看完整信息 →</span>
      </div>
    </div>
  </RouterLink>
</template>
