<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from "vue";
import { api, mutate, safe, label, time, ui } from "../api";
import StatusTag from "./StatusTag.vue";
import BaseDialog from "./BaseDialog.vue";
import UserAvatar from "./UserAvatar.vue";
import FeedbackTrail from "./FeedbackTrail.vue";
const scope = ref(""),
  notice = ref<any>(null);
async function openNotice(id: string) {
  await safe(
    async () => {
      notice.value = await api("/notifications/" + id);
      if (!notice.value.read) await read(id);
    },
    { busy: false },
  );
}
async function readGroup(entries: any[]) {
  await safe(
    async () => {
      for (const n of entries) {
        if (!n.read) await api("/notifications/" + n.id, "PATCH", {});
      }
      await load();
      emit("changed");
    },
    { busy: false },
  );
}

const props = defineProps<{ tab: string; highlight?: string }>();
const emit = defineEmits<{ changed: [] }>();
const data = ref<any>({ items: [], total: 0 }),
  page = ref(1),
  loading = ref(false),
  error = ref("");
const paths: Record<string, string> = {
  items: "/me/items",
  claims: "/me/claims",
  notices: "/notifications",
  reports: "/me/reports",
  operations: "/me/operations",
  evaluations: "/me/evaluations",
};
let sequence = 0;
async function load() {
  const current = ++sequence;
  loading.value = true;
  error.value = "";
  try {
    const result = await api(
      paths[props.tab] +
        "?" +
        new URLSearchParams({
          page: String(page.value),
          grouped: String(["claims", "notices"].includes(props.tab)),
          scope: scope.value,
        }),
    );
    if (current === sequence) data.value = result;
  } catch (e) {
    if (current === sequence) error.value = (e as Error).message;
  } finally {
    if (current === sequence) loading.value = false;
  }
}
async function withdraw(v: any) {
  if (!confirm("确定撤回这次认领申请？")) return;
  await safe(async () => {
    await mutate(
      "/items/" + v.item.id + "/claims/" + v.claim.id + "/withdraw",
      { expectedVersion: v.item.version },
    );
    await load();
    emit("changed");
  });
}
async function read(id?: string) {
  await safe(async () => {
    await api(
      id ? "/notifications/" + id : "/notifications/read-all",
      id ? "PATCH" : "POST",
      {},
    );
    await load();
    emit("changed");
  });
}
watch(
  () => props.tab,
  () => {
    page.value = 1;
    scope.value = "";
    data.value = { items: [], total: 0 };
    load();
  },
  { immediate: true },
);
let timer: ReturnType<typeof setInterval>;
onMounted(() => {
  timer = setInterval(() => {
    if (
      !document.hidden &&
      !loading.value &&
      ["claims", "notices", "operations"].includes(props.tab)
    )
      load();
  }, 15000);
});
onUnmounted(() => {
  sequence++;
  clearInterval(timer);
});
</script>
<template>
  <div class="records-toolbar">
    <div class="record-scopes" v-if="tab === 'claims'">
      <button
        v-for="s in [
          { value: '', name: '全部认领' },
          { value: 'active', name: '进行中' },
          { value: 'ended', name: '已结束' },
        ]"
        :key="s.value"
        :class="{ active: scope === s.value }"
        @click="
          scope = s.value;
          page = 1;
          load();
        "
      >
        {{ s.name }}
      </button>
    </div>
    <span v-else class="muted"
      >共 {{ data.total }} {{ tab === "notices" ? "组消息" : "条记录" }}</span
    >
    <div>
      <button
        v-if="tab === 'notices'"
        class="plain"
        :disabled="ui.busy"
        @click="read()"
      >
        全部已读</button
      ><button class="secondary" :disabled="loading" @click="load">
        {{ loading ? "更新中…" : "刷新记录" }}
      </button>
    </div>
  </div>
  <p v-if="tab === 'claims'" class="list-caption">
    按最近进展排列 · 同一物品的历次申请收纳在一起
  </p>
  <div v-if="error" class="request-error" role="alert">
    {{ error }} <button class="plain" @click="load">重试</button>
  </div>
  <p v-if="loading && !data.items.length" class="workspace-empty" role="status">
    正在加载记录…
  </p>
  <div v-else class="workspace-panel records-list" :aria-busy="loading">
    <article
      v-for="v in data.items"
      :key="v.id || v.item?.id"
      class="record-row"
      :class="{
        'is-highlighted': tab === 'claims' && v.item.id === highlight,
        'is-unread': tab === 'notices' && v.unread,
      }"
    >
      <template v-if="tab === 'items'">
        <RouterLink
          :to="'/items/' + v.id"
          class="record-thumbnail"
          tabindex="-1"
          aria-hidden="true"
          ><img
            v-if="v.content.uploadId"
            :src="'/uploads/' + v.content.uploadId"
            alt=""
          /><span v-else>拾物</span></RouterLink
        >
        <div class="record-main">
          <RouterLink :to="'/items/' + v.id"
            ><h3>{{ v.content.name }}</h3></RouterLink
          >
          <p>{{ v.content.location }}</p>
          <div class="record-metadata">
            <span>登记时间 {{ time(v.createdAt) }}</span
            ><span>最近进展 {{ time(v.updatedAt) }}</span>
          </div>
        </div>
        <div class="record-actions">
          <StatusTag :status="v.status" /><span class="muted">{{
            label(v.visibility)
          }}</span
          ><RouterLink :to="'/items/' + v.id">查看详情 →</RouterLink>
        </div>
      </template>
      <template v-else-if="tab === 'claims'">
        <div class="record-main">
          <div class="record-title">
            <RouterLink :to="'/items/' + v.item.id"
              ><h3>{{ v.item.content.name }}</h3></RouterLink
            ><StatusTag :status="v.latest.claim.status" />
          </div>
          <div class="record-metadata">
            <span>最新申请 {{ time(v.latest.claim.createdAt) }}</span
            ><span v-if="v.latest.feedback.length"
              >处理时间 {{ time(v.latest.feedback[0].at) }}</span
            >
          </div>
          <FeedbackTrail :entries="v.latest.feedback" compact />
          <details class="attempt-history">
            <summary>历次申请与提交材料 · {{ v.attempts.length }} 次</summary>
            <article
              v-for="a in v.attempts"
              :key="a.claim.id"
              class="attempt-entry"
            >
              <div class="record-title">
                <StatusTag :status="a.claim.status" /><time
                  >申请时间 {{ time(a.claim.createdAt) }}</time
                >
              </div>
              <p class="preserve">{{ a.myEvidence || "原材料未留存" }}</p>
              <FeedbackTrail :entries="a.feedback" />
              <details class="integrity-detail">
                <summary>材料存证信息</summary>
                <p>
                  {{
                    a.integrity
                      ? "提交材料未发生变化"
                      : "材料缺失或完整性异常，请联系管理员"
                  }}
                </p>
                <small
                  >用于检查提交材料是否被改动，与审核结果无关。</small
                >
              </details>
            </article>
          </details>
        </div>
        <div class="record-actions">
          <RouterLink :to="'/items/' + v.item.id">{{
            v.latest.claim.status === "APPROVED" ? "安排交接" : "查看物品 →"
          }}</RouterLink
          ><button
            v-if="v.latest.claim.status === 'PENDING'"
            class="plain"
            :disabled="ui.busy"
            @click="withdraw(v.latest)"
          >
            撤回认领
          </button>
        </div>
      </template>
      <template v-else-if="tab === 'notices'">
        <div class="notice-symbol" aria-hidden="true">
          {{ v.process ? "↗" : "✓" }}
        </div>
        <div class="record-main">
          <div class="record-title">
            <h3>{{ v.title }}</h3>
            <span v-if="v.unread" class="notice-unread"
              >{{ v.unread }} 条未读</span
            >
          </div>
          <p class="notice-object">{{ v.itemName || "相关业务记录" }}</p>
          <p v-if="!v.process">{{ v.entries[0]?.body }}</p>
          <p v-if="!v.process && v.entries[0]?.reason" class="notice-reason">
            处理说明：{{ v.entries[0].reason }}
          </p>
          <div class="record-metadata">
            <time>{{ time(v.at) }}</time
            ><span v-if="v.process">{{ v.entries.length }} 条动态</span>
          </div>
          <details v-if="v.process" class="notice-events">
            <summary>展开过程记录</summary>
            <div v-for="n in v.entries" :key="n.id" class="notice-event">
              <div>
                <strong>{{ n.title }}</strong
                ><span v-if="!n.read" class="unread-dot" aria-label="未读" />
              </div>
              <p>{{ n.body }}</p>
              <time>{{ time(n.at) }}</time
              ><button class="plain" @click="openNotice(n.id)">
                查看这次动态
              </button>
            </div>
          </details>
        </div>
        <div class="record-actions">
          <button class="plain" @click="openNotice(v.entries[0].id)">
            查看结果 →</button
          ><button v-if="v.unread" class="plain" @click="readGroup(v.entries)">
            标为已读
          </button>
        </div>
      </template>
      <template v-else-if="tab === 'reports'"
        ><div class="record-main">
          <div class="record-title">
            <h3>{{ v.kind === "DISPUTE" ? "交接争议" : "信息举报" }}</h3>
            <StatusTag :status="v.status" />
          </div>
          <p>{{ v.reason }}</p>
          <p class="notice-reason">
            处理意见：{{ v.resolution || "等待管理员处理" }}
          </p>
          <time>提交时间 {{ time(v.at) }}</time>
          <details v-if="v.history.length" class="attempt-history">
            <summary>查看处理过程</summary>
            <article v-for="h in v.history" :key="h.id" class="notice-event">
              <strong>{{
                h.action === "REPORT_RESOLVED" ? "已结案" : "已受理"
              }}</strong>
              <p>{{ h.reason }}</p>
              <time>{{ time(h.at) }}</time>
            </article>
          </details>
        </div>
        <RouterLink :to="'/items/' + v.itemId">查看物品 →</RouterLink></template
      >
      <template v-else-if="tab === 'evaluations'"
        ><UserAvatar :name="v.displayName" />
        <div class="record-main">
          <div class="record-title">
            <h3>{{ v.itemName }}</h3>
            <StatusTag
              :status="v.status"
              :text="v.status === 'HIDDEN' ? '已隐藏' : '公开'"
            />
          </div>
          <p class="review-stars">
            {{ "★".repeat(v.stars) }}<span>{{ v.stars }} / 5</span>
          </p>
          <p>{{ v.text }}</p>
          <time>评价时间 {{ time(v.at) }}</time>
          <div v-if="v.history.length" class="notice-reason">
            {{ v.status === "HIDDEN" ? "隐藏原因" : "最近处理说明" }}：{{
              v.history[0].reason
            }}<time>{{ time(v.history[0].at) }}</time>
          </div>
        </div>
        <RouterLink :to="'/items/' + v.itemId">查看物品 →</RouterLink></template
      >
      <template v-else
        ><div class="record-main">
          <div class="record-title">
            <h3>{{ label(v.command.action) }}</h3>
            <span class="badge">{{
              v.status === "PENDING" ? "结果待确认" : label(v.status)
            }}</span>
          </div>
          <p v-if="v.command.action === 'CLAIM' && v.status === 'SUCCEEDED'">
            申请已成功提交，认领结果仍需等待管理员审核。
          </p>
          <p v-if="v.error" class="negative">{{ v.error }}</p>
          <time>处理时间 {{ time(v.updatedAt) }}</time>
          <details v-if="v.txId" class="integrity-detail">
            <summary>查看交易编号</summary>
            <code class="break">{{ v.txId }}</code>
          </details>
        </div>
        <RouterLink :to="'/items/' + v.itemId">查看物品 →</RouterLink></template
      >
    </article>
    <div v-if="!data.total && !loading && !error" class="workspace-empty">
      <strong>{{
        tab === "notices" ? "暂时没有新消息" : "这里还没有记录"
      }}</strong>
      <p>
        {{
          tab === "items"
            ? "拾到物品后，登记信息让失主更快找到它。"
            : tab === "claims"
              ? "在招领大厅找到你的物品后，可以发起认领。"
              : "后续进展会显示在这里。"
        }}
      </p>
      <RouterLink v-if="tab === 'items'" to="/publish">登记拾物</RouterLink
      ><RouterLink v-if="tab === 'claims'" to="/">浏览招领大厅</RouterLink>
    </div>
  </div>
  <div v-if="data.total > 12" class="pagination">
    <button
      :disabled="page === 1 || loading"
      @click="
        page--;
        load();
      "
    >
      上一页</button
    ><span>{{ page }} / {{ Math.ceil(data.total / 12) }}</span
    ><button
      :disabled="page * 12 >= data.total || loading"
      @click="
        page++;
        load();
      "
    >
      下一页
    </button>
  </div>
  <BaseDialog v-if="notice" compact :title="notice.title" @close="notice = null"
    ><p class="eyebrow">{{ notice.itemName || "业务通知" }}</p>
    <p>{{ notice.body || notice.title }}</p>
    <div v-if="notice.reason" class="notice-reason">
      <h3>处理说明</h3>
      <p>{{ notice.reason }}</p>
    </div>
    <p>
      <time>{{ time(notice.at) }}</time>
    </p>
    <p class="muted">以上为本次事件发生时的记录，后续操作不会覆盖这条通知。</p>
    <div class="actions">
      <RouterLink
        v-if="notice.itemId"
        :to="'/items/' + notice.itemId"
        @click="notice = null"
        >查看物品当前状态 →</RouterLink
      ><RouterLink
        v-if="notice.action?.startsWith('EVALUATION_')"
        to="/me?tab=evaluations"
        @click="notice = null"
        >查看我的评价</RouterLink
      ><RouterLink
        v-if="notice.action?.startsWith('REPORT_')"
        to="/me?tab=reports"
        @click="notice = null"
        >查看反馈处理</RouterLink
      >
    </div></BaseDialog
  >
</template>
