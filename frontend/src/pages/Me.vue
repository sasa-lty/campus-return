<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { useRouter, useRoute } from "vue-router";
import { api, session, label, time } from "../api";
import WorkspaceNav from "../components/WorkspaceNav.vue";
import AccountSettings from "../components/AccountSettings.vue";
import ContributionPanel from "../components/ContributionPanel.vue";
import PersonalRecords from "../components/PersonalRecords.vue";
const router = useRouter(),
  route = useRoute();
const titles: Record<string, string> = {
  overview: "个人概览",
  items: "我的发布",
  claims: "我的认领",
  notices: "消息",
  points: "贡献积分",
  evaluations: "我的评价",
  reports: "举报与争议",
  account: "账户设置",
  operations: "操作进度",
};
const descriptions: Record<string, string> = {
  overview: "从这里查看认领进展，安排下一次交接。",
  items: "管理已登记的物品，查看认领与归还进展。",
  claims: "跟进你的申请，通过审核后即可安排交接。",
  notices: "审核、交接与反馈处理通知。",
  points: "每一次有效交接，都留下互助的记录。",
  evaluations: "查看交接评价和展示状态，了解管理员处理说明。",
  reports: "查看已提交反馈的处理情况。",
  account: "维护个人资料与登录安全。",
  operations: "查询提交结果与链上交易记录。",
};
const tab = computed(() =>
  session.user?.forcePassword
    ? "account"
    : typeof route.query.tab === "string" && titles[route.query.tab]
      ? route.query.tab
      : "overview",
);
const summary = ref<any>(null),
  points = ref<any>(null),
  loading = ref(false),
  error = ref("");
const menus = computed(() =>
  Object.entries(titles).map(([key, text]) => ({
    key,
    label: text,
    group: ["overview", "items", "claims"].includes(key)
      ? "我的业务"
      : ["notices", "points", "reports", "evaluations"].includes(key)
        ? "互动与贡献"
        : "账户与记录",
    badge: key === "notices" ? summary.value?.counts.unread : undefined,
  })),
);
const highlight = computed(() =>
  typeof route.query.item === "string" ? route.query.item : undefined,
);
function switchTab(key: string) {
  router.replace({ path: "/me", query: { tab: key } });
}
let sequence = 0;
async function refresh() {
  if (!session.token || session.user?.forcePassword) return;
  const current = ++sequence;
  loading.value = true;
  error.value = "";
  try {
    const [overview, contribution] = await Promise.all([
      api("/me/overview"),
      api("/me/points"),
    ]);
    if (current === sequence) {
      summary.value = overview;
      points.value = contribution;
    }
  } catch (e) {
    if (current === sequence) error.value = (e as Error).message;
  } finally {
    if (current === sequence) loading.value = false;
  }
}
watch(tab, () => {
  if (["overview", "points"].includes(tab.value)) refresh();
});
let timer: ReturnType<typeof setInterval>;
onMounted(async () => {
  if (!session.token) return;
  try {
    session.user = await api("/me");
    await refresh();
  } catch (e) {
    error.value = (e as Error).message;
  }
  timer = setInterval(() => {
    if (!document.hidden && !loading.value && tab.value === "overview")
      refresh();
  }, 15000);
});
onUnmounted(() => {
  sequence++;
  clearInterval(timer);
});
</script>
<template>
  <section v-if="!session.token" class="workspace-panel workspace-empty">
    <h1>个人中心</h1>
    <p>登录后查看你的发布、认领和交接进展。</p>
    <RouterLink to="/login">登录平台</RouterLink>
  </section>
  <section v-else class="workspace-layout personal-workspace">
    <WorkspaceNav
      title="个人中心分区"
      :name="session.user?.displayName || '个人中心'"
      :role="label(session.user?.role || 'USER')"
      :items="menus"
      :active="tab"
      @select="switchTab"
    >
      <RouterLink
        v-if="['ADMIN', 'SUPER_ADMIN'].includes(session.user?.role)"
        to="/admin"
        >进入管理工作台</RouterLink
      ><RouterLink v-else to="/">返回招领大厅</RouterLink>
    </WorkspaceNav>
    <div class="workspace-content">
      <div class="workspace-heading">
        <div>
          <p class="workspace-breadcrumb">个人中心 / {{ titles[tab] }}</p>
          <h1>{{ titles[tab] }}</h1>
          <p class="muted">{{ descriptions[tab] }}</p>
        </div>
        <RouterLink
          v-if="tab === 'overview' || tab === 'items'"
          class="workspace-primary-link"
          to="/publish"
          >登记拾物</RouterLink
        >
      </div>
      <div v-if="error" class="request-error" role="alert">
        {{ error }} <button class="plain" @click="refresh">重试</button>
      </div>
      <template v-if="tab === 'overview'">
        <p v-if="loading && !summary" class="workspace-empty" role="status">
          正在加载个人概览…
        </p>
        <template v-if="summary">
          <div class="workspace-metrics">
            <button @click="switchTab('items')">
              <span>我的发布</span><strong>{{ summary.counts.items }}</strong
              ><small>查看物品进展</small>
            </button>
            <button @click="switchTab('claims')">
              <span>待审核申请</span
              ><strong>{{ summary.counts.pendingClaims }}</strong
              ><small>我的认领进度</small>
            </button>
            <button @click="switchTab('claims')">
              <span>待领取物品</span
              ><strong>{{ summary.counts.handovers }}</strong
              ><small>审核通过，准备交接</small>
            </button>
            <button @click="switchTab('notices')">
              <span>未读消息</span><strong>{{ summary.counts.unread }}</strong
              ><small>查看最新通知</small>
            </button>
          </div>
          <div class="workspace-columns">
            <div>
              <section class="workspace-panel">
                <div class="workspace-panel-head">
                  <h2>待我领取</h2>
                  <button class="plain" @click="switchTab('claims')">
                    全部认领
                  </button>
                </div>
                <div
                  v-for="item in summary.handovers"
                  :key="item.id"
                  class="workspace-row"
                >
                  <div>
                    <strong>{{ item.name }}</strong
                    ><small>{{ item.location }}</small>
                  </div>
                  <RouterLink :to="'/items/' + item.id">安排交接</RouterLink>
                </div>
                <div
                  v-if="!summary.handovers.length"
                  class="workspace-empty compact"
                >
                  <strong>暂无待领取物品</strong>
                  <p>认领通过审核后，可在这里进入详情安排交接。</p>
                  <RouterLink to="/">浏览招领大厅</RouterLink>
                </div>
              </section>
              <section class="workspace-panel">
                <div class="workspace-panel-head">
                  <h2>最近消息</h2>
                  <button class="plain" @click="switchTab('notices')">
                    全部消息
                  </button>
                </div>
                <div
                  v-for="notice in summary.notices"
                  :key="notice.id"
                  class="workspace-row"
                >
                  <div>
                    <strong
                      >{{ notice.title
                      }}<span
                        v-if="!notice.read"
                        class="unread-dot"
                        aria-label="未读"
                    /></strong>
                    <p class="notice-object">{{ notice.itemName }}</p>
                    <small>{{ time(notice.at) }}</small>
                  </div>
                  <RouterLink
                    v-if="notice.itemId"
                    :to="{ path: '/me', query: { tab: 'notices' } }"
                    >查看</RouterLink
                  >
                </div>
                <p
                  v-if="!summary.notices.length"
                  class="workspace-empty compact"
                >
                  还没有消息，业务进展会显示在这里。
                </p>
              </section>
            </div>
            <div>
              <section v-if="points" class="workspace-panel">
                <div class="workspace-panel-head">
                  <h2>互助贡献</h2>
                  <span class="badge">Lv{{ points.level }}</span>
                </div>
                <div class="score-value">
                  {{ points.score }}<span>积分</span>
                </div>
                <p class="muted">已协助归还 {{ points.returned }} 件物品</p>
                <button
                  class="secondary full-width"
                  @click="switchTab('points')"
                >
                  查看积分与规则
                </button>
              </section>
              <section class="workspace-panel">
                <h2>其他进展</h2>
                <button
                  class="workspace-shortcut"
                  @click="switchTab('reports')"
                >
                  举报与争议<span
                    >{{ summary.counts.openReports }} 项处理中</span
                  ></button
                ><button
                  class="workspace-shortcut"
                  @click="switchTab('operations')"
                >
                  操作进度<span
                    >{{ summary.counts.pendingOperations }} 项待确认</span
                  >
                </button>
                <p class="muted">
                  申请已提交后，无需重复操作；审核结果会通过消息通知。
                </p>
              </section>
            </div>
          </div>
        </template>
      </template>
      <AccountSettings v-else-if="tab === 'account'" />
      <template v-else-if="tab === 'points'"
        ><ContributionPanel v-if="points" :points="points" />
        <p v-else-if="loading" role="status">正在加载积分…</p></template
      >
      <PersonalRecords
        v-else
        :tab="tab"
        :highlight="highlight"
        @changed="refresh"
      />
    </div>
  </section>
</template>
