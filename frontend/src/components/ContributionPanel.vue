<script setup lang="ts">
import { computed } from "vue";
import { time } from "../api";
const props = defineProps<{ points: any }>();
const next = computed(() =>
  [20, 50, 100].find((value) => value > props.points.score),
);
const progress = computed(() =>
  Math.max(0, Math.min(100, (props.points.score / (next.value || 100)) * 100)),
);
</script>
<template>
  <section class="workspace-panel contribution-summary">
    <div>
      <span class="muted">贡献积分</span>
      <div class="score-value">
        {{ points.score }}<span>Lv{{ points.level }}</span>
      </div>
      <p class="muted">
        {{
          next
            ? `距离下一等级还需 ${next - points.score} 分`
            : "已达到当前最高等级"
        }}
      </p>
      <progress :value="progress" max="100" aria-label="贡献等级进度" />
    </div>
    <div class="contribution-facts">
      <p>
        登记拾物<strong>{{ points.published }} 件</strong>
      </p>
      <p>
        已帮助归还<strong>{{ points.returned }} 件</strong>
      </p>
    </div>
  </section>
  <p class="stat-definition">
    登记拾物为累计登记数；已帮助归还为你登记的物品中，已完成交接归还的数量。
  </p>
  <section class="workspace-panel">
    <h2>积分规则</h2>
    <p>{{ points.rules }}</p>
    
  </section>
  <section class="workspace-panel">
    <h2>积分明细</h2>
    <div v-for="entry in points.entries" :key="entry.id" class="workspace-row">
      <div>
        <strong>{{ entry.reason }}</strong
        ><small>{{ time(entry.at) }}</small>
      </div>
      <b :class="entry.delta > 0 ? 'positive' : 'negative'"
        >{{ entry.delta > 0 ? "+" : "" }}{{ entry.delta }}</b
      >
    </div>
    <p v-if="!points.entries.length" class="workspace-empty">
      还没有积分记录。完成有效交接后，积分会自动记入。
    </p>
  </section>
</template>
