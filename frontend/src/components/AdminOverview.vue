<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import { api, time } from "../api";
import StatusTag from "./StatusTag.vue";
const props = defineProps<{ stats: any }>();
defineEmits<{
  navigate: [tab: string, status?: string];
  open: [id: string];
  scan: [];
}>();
const stage = ref("CLAIM_PENDING"),
  queues = ref<Record<string, any[]>>({}),
  loading = ref(false),
  error = ref("");
const currentQueue = computed(() => queues.value[stage.value] || []);
const categories = computed(() =>
  Object.entries(props.stats.categories).sort(
    (a, b) => Number(b[1]) - Number(a[1]),
  ),
);
const days = computed(() =>
  Object.entries(props.stats.daily)
    .sort(([a], [b]) => a.localeCompare(b))
    .slice(-7),
);
const maxDay = computed(() =>
  Math.max(1, ...days.value.map(([, value]) => Number(value))),
);
let sequence = 0;
async function loadQueue() {
  const current = ++sequence;
  loading.value = true;
  error.value = "";
  try {
    const [pending, handovers] = await Promise.all(
      ["CLAIM_PENDING", "CLAIMED"].map((status) =>
        api(
          "/admin/items?" +
            new URLSearchParams({ status, page: "1", pageSize: "5" }),
        ),
      ),
    );
    if (current === sequence)
      queues.value = { CLAIM_PENDING: pending.items, CLAIMED: handovers.items };
  } catch (e) {
    if (current === sequence) error.value = (e as Error).message;
  } finally {
    if (current === sequence) loading.value = false;
  }
}
watch(() => props.stats, loadQueue, { immediate: true });
onUnmounted(() => {
  sequence++;
});
</script>
<template>
  <div class="workspace-metrics admin-task-metrics">
    <button @click="$emit('navigate', 'items', 'CLAIM_PENDING')">
      <span>待审核物品</span><strong>{{ stats.counts.CLAIM_PENDING }}</strong
      ><small>核对材料并处理申请</small>
    </button>
    <button @click="$emit('navigate', 'items', 'CLAIMED')">
      <span>待交接物品</span><strong>{{ stats.counts.CLAIMED }}</strong
      ><small>预约、核码与确认归还</small>
    </button>
    <button @click="$emit('navigate', 'reports')">
      <span>未结反馈</span><strong>{{ stats.management.openReports }}</strong
      ><small>受理举报与交接争议</small>
    </button>
    <button @click="$emit('navigate', 'operations', 'ATTENTION')">
      <span>待核对与失败操作</span
      ><strong>{{ stats.management.abnormalOperations }}</strong
      ><small>查看交易结果与异常</small>
    </button>
  </div>
  <div class="workspace-columns">
    <section class="workspace-panel queue-panel">
      <div class="workspace-panel-head">
        <h2>待办队列</h2>
        <button class="plain" @click="$emit('navigate', 'items', stage)">
          查看全部
        </button>
      </div>
      <div class="tabs queue-tabs" aria-label="待办队列类型">
        <button
          :class="{ active: stage === 'CLAIM_PENDING' }"
          @click="stage = 'CLAIM_PENDING'"
        >
          认领审核 <span>{{ stats.counts.CLAIM_PENDING }}</span></button
        ><button
          :class="{ active: stage === 'CLAIMED' }"
          @click="stage = 'CLAIMED'"
        >
          交接归还 <span>{{ stats.counts.CLAIMED }}</span>
        </button>
      </div>
      <p v-if="error" class="request-error" role="alert">
        {{ error }}<button class="plain" @click="loadQueue">重试</button>
      </p>
      <p
        v-else-if="loading && !currentQueue.length"
        class="workspace-empty"
        role="status"
      >
        正在加载待办…
      </p>
      <template v-else
        ><div v-for="item in currentQueue" :key="item.id" class="workspace-row">
          <div class="queue-item">
            <strong>{{ item.content.name }}</strong
            ><small
              >{{ item.content.location }} · {{ time(item.updatedAt) }}</small
            ><small v-if="stage === 'CLAIM_PENDING'"
              >{{
                item.claims?.filter((v: any) => v.status === "PENDING").length
              }}
              份待审核申请</small
            >
          </div>
          <button class="secondary" @click="$emit('open', item.id)">
            {{ stage === "CLAIM_PENDING" ? "审核申请" : "办理交接" }}
          </button>
        </div>
        <div v-if="!currentQueue.length" class="workspace-empty">
          <strong>{{
            stage === "CLAIM_PENDING" ? "暂无待审核申请" : "暂无待交接物品"
          }}</strong>
          <p>新任务出现后会显示在这里。</p>
        </div></template
      >
      <p v-if="currentQueue.length" class="muted queue-footnote">
        显示最近更新的 {{ currentQueue.length }} 件物品
      </p>
    </section>
    <section class="workspace-panel">
      <h2>快捷办理</h2>
      <button class="workspace-shortcut" @click="$emit('scan')">
        扫描交接二维码<span>现场核验</span></button
      ><button class="workspace-shortcut" @click="$emit('navigate', 'items')">
        查询与管理物品<span>编辑、下架、关闭</span></button
      ><button class="workspace-shortcut" @click="$emit('navigate', 'audit')">
        查看操作日志<span>追踪处理记录</span>
      </button>
      <div class="workspace-tip">
        <strong>办理提醒</strong>
        <p>
          对照参考答案核验认领证明，归还前请当面核验交接码。
        </p>
      </div>
    </section>
  </div>
  <section class="workspace-panel">
    <div class="workspace-panel-head">
      <h2>业务概况</h2>
      
    </div>
    <div class="stats-strip">
      <div>
        <span>登记物品</span><strong>{{ stats.counts.total }}</strong>
      </div>
      <div>
        <span>完成归还</span><strong>{{ stats.counts.RETURNED }}</strong>
      </div>
      <div>
        <span>已下架</span><strong>{{ stats.counts.hidden }}</strong>
      </div>
      <div>
        <span>平台账号</span><strong>{{ stats.management.userCount }}</strong>
      </div>
    </div>
    <div class="workspace-columns stats-details">
      <div>
        <h3>物品分类</h3>
        <div
          v-for="[name, count] in categories"
          :key="name"
          class="category-row"
        >
          <span>{{ name || "未分类" }}</span
          ><meter
            :value="Number(count)"
            :max="Math.max(stats.counts.total, 1)"
            :aria-label="name || '未分类'"
          /><b>{{ count }}</b>
        </div>
        <p v-if="!categories.length" class="muted">还没有登记数据。</p>
      </div>
      <div>
        <h3>近期登记</h3>
        <div v-for="[day, count] in days" :key="day" class="category-row">
          <span>{{ day }}</span
          ><meter :value="Number(count)" :max="maxDay" :aria-label="day" /><b>{{
            count
          }}</b>
        </div>
        <p class="muted">展示最近 7 个有登记记录的日期。</p>
      </div>
    </div>
  </section>
</template>
