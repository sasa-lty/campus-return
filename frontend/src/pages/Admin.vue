<script setup lang="ts">
import {
  onMounted,
  onUnmounted,
  ref,
  reactive,
  computed,
  defineAsyncComponent,
  watch,
} from "vue";
import { api, safe, mutate, session, label, time, notify, ui } from "../api";
import { useRoute, useRouter } from "vue-router";
import WorkspaceNav from "../components/WorkspaceNav.vue";
import AdminOverview from "../components/AdminOverview.vue";
import ManagementDialog from "../components/ManagementDialog.vue";
import ContentForm from "../components/ContentForm.vue";
import BaseDialog from "../components/BaseDialog.vue";
import StatusTag from "../components/StatusTag.vue";
import UserAvatar from "../components/UserAvatar.vue";
import FeedbackTrail from "../components/FeedbackTrail.vue";
const reviewTab = ref("pending");
const claimDrafts: Record<string, string> = {};
const pendingClaims = computed(() =>
  (privateData.value?.claims || [])
    .filter((v: any) => v.claim.status === "PENDING")
    .sort((a: any, b: any) =>
      b.claim.createdAt.localeCompare(a.claim.createdAt),
    ),
);
const historicalClaims = computed(() =>
  (privateData.value?.claims || [])
    .filter((v: any) => v.claim.status !== "PENDING")
    .sort((a: any, b: any) =>
      b.claim.createdAt.localeCompare(a.claim.createdAt),
    ),
);
const visibleClaims = computed(() =>
  reviewTab.value === "pending" ? pendingClaims.value : historicalClaims.value,
);

function selectClaimTab(value: string) {
  reviewTab.value = value;
  claimId.value =
    (value === "pending" ? pendingClaims.value : historicalClaims.value)[0]
      ?.claim.id || "";
}

const Scanner = defineAsyncComponent(() => import("../components/Scanner.vue"));
const route = useRoute(),
  router = useRouter();
const managementTarget = ref<any>(null);
const tab = ref("overview"),
  page = ref(1),
  query = ref(""),
  filter = ref("");
const data = ref<any>({ items: [], total: 0 }),
  stats = ref<any>(null),
  selected = ref<any>(null),
  privateData = ref<any>(null),
  editing = ref(false),
  scanning = ref(false);
const note = ref(""),
  claimId = ref(""),
  code = ref(""),
  reportId = ref("");
const appointment = reactive({ location: "", at: "" });
const reviewTarget = ref<{ id: string; title: string } | null>(null);
const reviewLoading = ref(false),
  reviewError = ref("");
const appointmentLoading = ref(false),
  appointmentError = ref("");
let reviewSequence = 0;
let reviewController: AbortController | undefined;
let appointmentController: AbortController | undefined;
function closeReview() {
  reviewSequence++;
  reviewController?.abort();
  appointmentController?.abort();
  reviewTarget.value = null;
  selected.value = null;
  privateData.value = null;
  reviewLoading.value = false;
  appointmentLoading.value = false;
}

watch(claimId, (next, old) => {
  if (old) claimDrafts[old] = note.value;
  note.value = claimDrafts[next] || "";
});
const selectedClaim = computed(() =>
  privateData.value?.claims.find((v: any) => v.claim.id === claimId.value),
);
const canApprove = computed(
  () =>
    selectedClaim.value?.claim.status === "PENDING" &&
    selectedClaim.value?.integrity &&
    privateData.value?.answerIntegrity,
);
const stages: Record<string, string> = {
  "": "全部物品",
  CLAIM_PENDING: "待审核",
  CLAIMED: "待交接",
  RETURNED: "已归还",
  CLOSED: "已关闭",
};
const paths: Record<string, string> = {
  items: "/admin/items",
  users: "/admin/users",
  reports: "/admin/reports",
  evaluations: "/admin/evaluations",
  operations: "/admin/operations",
  audit: "/admin/audit",
};
const tabs: Record<string, string> = {
  overview: "工作台",
  items: "审核与物品",
  reports: "举报与争议",
  evaluations: "评价治理",
  users: "用户与权限",
  operations: "交易进度",
  audit: "操作日志",
};
const descriptions: Record<string, string> = {
  overview: "集中处理认领审核、线下交接与用户反馈。",
  items: "核验认领、办理交接，维护物品内容与展示状态。",
  users: "管理账号状态与角色授权，所有变更均保留记录。",
  reports: "受理举报和交接争议，向用户反馈处理结果。",
  evaluations: "处理违规评价，保留原始交接与治理记录。",
  operations: "查看交易确认进度，核对失败与待确认操作。",
  audit: "追踪管理员治理、权限变更和私密材料访问。",
};
const menus = computed(() =>
  Object.entries(tabs)
    .filter(([key]) => key !== "users" || session.user?.role === "SUPER_ADMIN")
    .map(([key, text]) => ({
      key,
      label: text,
      group: ["overview", "items"].includes(key)
        ? "业务办理"
        : ["reports", "evaluations"].includes(key)
          ? "内容与反馈"
          : "平台管理",
    })),
);
const loading = ref(false),
  loadError = ref(""),
  updatedAt = ref("");
let sequence = 0;
async function load() {
  if (!session.user?.role || session.user.role === "USER") return;
  const current = ++sequence;
  const target = tab.value;
  loading.value = true;
  loadError.value = "";
  try {
    const [statistics, list] = await Promise.all([
      api("/admin/stats"),
      target !== "overview"
        ? api(
            paths[target] +
              "?" +
              new URLSearchParams({
                page: String(page.value),
                q: query.value,
                status: filter.value,
              }),
          )
        : Promise.resolve(null),
    ]);
    if (current === sequence) {
      stats.value = statistics;
      if (list) data.value = list;
      updatedAt.value = new Date().toLocaleTimeString("zh-CN", {
        hour12: false,
      });
    }
  } catch (e) {
    if (current === sequence) loadError.value = (e as Error).message;
  } finally {
    if (current === sequence) loading.value = false;
  }
}
function change(t: string, status = "") {
  router.push({
    path: "/admin",
    query: { tab: t, ...(status ? { status } : {}) },
  });
}
function syncRoute() {
  const target =
    typeof route.query.tab === "string" && tabs[route.query.tab]
      ? route.query.tab
      : "overview";
  tab.value =
    target === "users" && session.user?.role !== "SUPER_ADMIN"
      ? "overview"
      : target;
  filter.value =
    typeof route.query.status === "string" ? route.query.status : "";
  page.value = 1;
  query.value = "";
  closeReview();
  data.value = { items: [], total: 0 };
  managementTarget.value = null;
  load();
}
watch(() => route.fullPath, syncRoute);
onUnmounted(() => {
  sequence++;
  closeReview();
});
// 立即显示办理窗口，必要材料和非必要预约分别加载，过期响应不写回新窗口。
async function open(id: string, title = "审核与物品办理") {
  if (reviewTarget.value?.id === id && reviewLoading.value) return;
  const sameItem = reviewTarget.value?.id === id;
  closeReview();
  const current = reviewSequence;
  if (!sameItem) note.value = "";
  reviewTarget.value = { id, title };
  reviewLoading.value = true;
  reviewError.value = "";
  appointmentError.value = "";
  editing.value = false;
  appointment.location = "";
  appointment.at = "";
  reviewController = new AbortController();
  const controller = reviewController;
  const deadline = setTimeout(() => controller.abort(), 15000);
  const read = async (path: string, description: string) => {
    try {
      return await api(path, "GET", undefined, undefined, controller.signal);
    } catch (e) {
      throw new Error(description + "加载失败：" + (e as Error).message);
    }
  };
  try {
    const [item, materials] = await Promise.all([
      read("/items/" + id, "物品信息"),
      read("/admin/items/" + id + "/private", "核验材料"),
    ]);
    if (current !== reviewSequence) return;
    selected.value = item;
    privateData.value = materials;
    if (!sameItem) {
      for (const k in claimDrafts) delete claimDrafts[k];
    }
    reviewTab.value = materials.claims.some(
      (x: any) => x.claim.status === "PENDING",
    )
      ? "pending"
      : "history";
    claimId.value =
      item.claimId ||
      materials.claims.find((x: any) => x.claim.status === "PENDING")?.claim
        .id ||
      materials.claims.at(-1)?.claim.id ||
      "";
    appointment.location = item.content.location;
    if (item.status === "CLAIMED") loadAppointment(id, current);
  } catch (e) {
    if (current === reviewSequence) reviewError.value = (e as Error).message;
  } finally {
    clearTimeout(deadline);
    if (current === reviewSequence) reviewLoading.value = false;
  }
}
async function loadAppointment(id: string, current = reviewSequence) {
  appointmentController?.abort();
  const controller = new AbortController();
  appointmentController = controller;
  appointmentLoading.value = true;
  appointmentError.value = "";
  const deadline = setTimeout(() => controller.abort(), 10000);
  try {
    const ap = await api(
      "/items/" + id + "/appointment",
      "GET",
      undefined,
      undefined,
      controller.signal,
    );
    if (current !== reviewSequence || controller !== appointmentController)
      return;
    appointment.location = ap.location || selected.value.content.location;
    appointment.at = ap.at
      ? new Date(
          new Date(ap.at).getTime() - new Date().getTimezoneOffset() * 60000,
        )
          .toISOString()
          .slice(0, 16)
      : "";
  } catch (e) {
    if (current === reviewSequence && controller === appointmentController)
      appointmentError.value = "预约加载失败：" + (e as Error).message;
  } finally {
    clearTimeout(deadline);
    if (current === reviewSequence && controller === appointmentController)
      appointmentLoading.value = false;
  }
}
async function action(action: string) {
  if (!note.value.trim()) {
    notify("请填写本次处理说明");
    return;
  }
  if (
    ["APPROVE", "REVOKE", "CLOSE", "FLAG_FRAUD", "RETURN"].includes(action) &&
    !confirm(
      {
        APPROVE: "确认将所选申请确权？其他待审核申请将转为未选中。",
        REVOKE: "确认撤销当前确权？系统会追加记录并重新开放认领。",
        CLOSE: "确认关闭认领？待审核申请会一并取消。",
        FLAG_FRAUD: "确认认定虚假交接并纠正相应积分？原归还历史仍会保留。",
        RETURN: "确认已当面核验交接码并完成物品交接？",
      }[action],
    )
  )
    return;
  await safe(async () => {
    await mutate("/admin/items/" + selected.value.id + "/actions", {
      action,
      claimId: claimId.value,
      note: note.value,
      code: code.value,
      reportId: reportId.value,
      expectedVersion: selected.value.version,
    });
    code.value = "";
  });
  if (selected.value) await open(selected.value.id);
  await load();
}
async function edit(q: any) {
  await safe(() =>
    mutate(
      "/admin/items/" + selected.value.id,
      { ...q, expectedVersion: selected.value.version },
      "PATCH",
    ),
  );
  await open(selected.value.id);
  await load();
}
async function book() {
  await safe(() =>
    api("/items/" + selected.value.id + "/appointment", "POST", {
      location: appointment.location,
      at: new Date(appointment.at).toISOString(),
    }),
  );
  notify("预约已确认");
}
async function scan(result: string) {
  const parts = result.split(":");
  if (parts.length !== 4 || parts[0] !== "campus-return") {
    notify("不是有效交接二维码");
    return;
  }
  await open(parts[1]);
  if (selected.value?.claimId !== parts[2]) {
    notify("认领信息与当前物品不一致");
    return;
  }
  code.value = parts[3];
  claimId.value = parts[2];
  scanning.value = false;
  notify("交接码已识别，请核对物品并填写归还说明");
}
watch(
  () => session.user?.id,
  () => syncRoute(),
);
onMounted(syncRoute);
</script>
<template>
  <section
    v-if="
      session.user?.role === 'ADMIN' || session.user?.role === 'SUPER_ADMIN'
    "
    class="workspace-layout admin-workspace"
  >
    <WorkspaceNav
      title="管理工作台分区"
      :name="session.user.displayName"
      :role="label(session.user.role)"
      :items="menus"
      :active="tab"
      @select="change"
    >
      <RouterLink to="/me">个人中心</RouterLink>
    </WorkspaceNav>
    <div class="workspace-content admin-content">
      <div class="workspace-heading">
        <div>
          <p class="workspace-breadcrumb">管理工作台 / {{ tabs[tab] }}</p>
          <h1>{{ tabs[tab] }}</h1>
          <p class="muted">{{ descriptions[tab] }}</p>
        </div>
        <button class="secondary" @click="load" :disabled="loading">
          {{ loading ? "更新中…" : "刷新" }}
        </button>
      </div>
      <div class="admin-context">
        <span>{{
          session.user.role === "SUPER_ADMIN"
            ? "超级管理员 · 业务与账号管理权限"
            : "业务管理员 · 审核、交接与内容治理权限"
        }}</span
        ><span v-if="updatedAt">更新于 {{ updatedAt }}</span>
      </div>
      <p v-if="loadError" class="request-error" role="alert">
        {{ loadError }} <button class="plain" @click="load">重试</button>
      </p>
      <p v-if="loading && !stats" role="status" class="workspace-empty">
        正在加载工作台…
      </p>
      <Scanner v-if="scanning" @result="scan" @close="scanning = false" />
      <template v-if="tab === 'overview'"
        ><AdminOverview
          v-if="stats"
          :stats="stats"
          @navigate="change"
          @open="open"
          @scan="scanning = true"
      /></template>
      <template v-else>
        <div
          v-if="tab === 'items'"
          class="tabs stage-tabs"
          aria-label="办理阶段"
        >
          <button
            v-for="(name, status) in stages"
            :key="status"
            :class="{ active: filter === status }"
            @click="
              filter = status;
              page = 1;
              load();
            "
          >
            {{ name }}
          </button>
        </div>
        <form
          class="filters"
          @submit.prevent="
            page = 1;
            load();
          "
        >
          <input
            v-if="['items', 'users', 'audit', 'evaluations'].includes(tab)"
            v-model="query"
            placeholder="搜索记录"
            aria-label="搜索记录"
          /><select
            v-if="
              ['items', 'reports', 'operations', 'evaluations'].includes(tab)
            "
            v-model="filter"
          >
            <option value="">全部状态</option>
            <template v-if="tab === 'items'"
              ><option
                v-for="st in [
                  'FOUND',
                  'CLAIM_PENDING',
                  'CLAIMED',
                  'RETURNED',
                  'CLOSED',
                ]"
                :value="st"
              >
                {{ label(st) }}
              </option></template
            ><template v-if="tab === 'evaluations'"
              ><option value="PUBLIC">公开</option>
              <option value="HIDDEN">已隐藏</option></template
            ><template v-if="tab === 'reports'"
              ><option
                v-for="st in ['OPEN', 'IN_PROGRESS', 'RESOLVED']"
                :value="st"
              >
                {{ label(st) }}
              </option></template
            ><template v-if="tab === 'operations'"
              ><option
                v-for="st in [
                  'ATTENTION',
                  'QUEUED',
                  'SUBMITTING',
                  'PENDING',
                  'FAILED',
                  'SUCCEEDED',
                ]"
                :value="st"
              >
                {{ label(st) }}
              </option></template
            ></select
          ><button>筛选</button
          ><button
            type="button"
            class="secondary"
            @click="
              query = '';
              filter = '';
              page = 1;
              load();
            "
          >
            重置
          </button>
        </form>
        <div class="table-wrap" :aria-busy="loading">
          <table>
            <thead>
              <tr v-if="tab === 'items'">
                <th>物品</th>
                <th>状态</th>
                <th>展示</th>
                <th>更新时间</th>
                <th>操作</th>
              </tr>
              <tr v-else-if="tab === 'users'">
                <th>用户</th>
                <th>角色</th>
                <th>状态</th>
                <th>操作</th>
              </tr>
              <tr v-else-if="tab === 'reports'">
                <th>类型 / 说明</th>
                <th>进度 / 结果</th>
                <th>操作</th>
              </tr>
              <tr v-else-if="tab === 'evaluations'">
                <th>评价</th>
                <th>状态</th>
                <th>操作</th>
              </tr>
              <tr v-else-if="tab === 'operations'">
                <th>操作</th>
                <th>状态</th>
                <th>交易 / 说明</th>
                <th>时间</th>
              </tr>
              <tr v-else>
                <th>操作</th>
                <th>对象 / 操作者</th>
                <th>原因</th>
                <th>时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="v in data.items" :key="v.id">
                <template v-if="tab === 'items'"
                  ><td>
                    {{ v.content.name }}<small class="break">{{ v.id }}</small>
                  </td>
                  <td><StatusTag :status="v.status" /></td>
                  <td>{{ label(v.visibility) }}</td>
                  <td>{{ time(v.updatedAt) }}</td>
                  <td>
                    <button
                      class="secondary"
                      @click="
                        reportId = '';
                        open(v.id, v.content.name);
                      "
                    >
                      处理
                    </button>
                  </td></template
                ><template v-else-if="tab === 'users'"
                  ><td>
                    {{ v.username }}<small>{{ v.displayName }}</small>
                  </td>
                  <td>
                    {{ label(v.role) }}
                  </td>
                  <td>{{ v.status === "ACTIVE" ? "启用" : "禁用" }}</td>
                  <td>
                    <button class="secondary" @click="managementTarget = v">
                      管理账号
                    </button>
                  </td></template
                >
                <template v-else-if="tab === 'reports'"
                  ><td>
                    {{ v.kind === "DISPUTE" ? "交接争议" : "举报" }}
                    <p>{{ v.reason }}</p>
                    <small>{{ v.itemId }}</small>
                  </td>
                  <td>
                    {{ label(v.status) }}
                    <p>{{ v.resolution }}</p>
                  </td>
                  <td>
                    <button
                      class="secondary"
                      @click="
                        reportId = v.id;
                        open(v.itemId);
                      "
                    >
                      查看物品并处理</button
                    ><button
                      v-if="v.status !== 'RESOLVED'"
                      class="plain"
                      @click="managementTarget = v"
                    >
                      处理反馈
                    </button>
                  </td></template
                ><template v-else-if="tab === 'evaluations'"
                  ><td>
                    <div class="review-author">
                      <UserAvatar :name="v.displayName" small /><strong>{{
                        v.displayName
                      }}</strong
                      ><span>{{ v.participantRole }}</span>
                    </div>
                    <p>{{ "★".repeat(v.stars) }} {{ v.text }}</p>
                    <small>{{ v.itemName }} · {{ time(v.at) }}</small>
                  </td>
                  <td>{{ v.status === "HIDDEN" ? "已隐藏" : "公开" }}</td>
                  <td>
                    <button class="secondary" @click="managementTarget = v">
                      管理评价
                    </button>
                  </td></template
                ><template v-else-if="tab === 'operations'"
                  ><td>
                    {{ label(v.command.action) }}<small>{{ v.itemId }}</small>
                  </td>
                  <td>{{ label(v.status) }}</td>
                  <td class="break">
                    <code>{{ v.txId }}</code>
                    <p>{{ v.error }}</p>
                  </td>
                  <td>{{ time(v.updatedAt) }}</td></template
                ><template v-else
                  ><td>{{ v.action }}</td>
                  <td class="break">
                    {{ v.targetId }}<small>{{ v.actorId }}</small>
                  </td>
                  <td>{{ v.reason }}</td>
                  <td>{{ time(v.at) }}</td></template
                >
              </tr>
            </tbody>
          </table>
        </div>
        <p v-if="!data.total && !loading && !loadError" class="empty">
          没有符合条件的记录。
        </p>
        <div class="pagination">
          <button
            :disabled="page === 1 || loading"
            @click="
              page--;
              load();
            "
          >
            上一页</button
          ><span>{{ page }} / 共 {{ data.total }} 条</span
          ><button
            :disabled="page * 12 >= data.total || loading"
            @click="
              page++;
              load();
            "
          >
            下一页
          </button>
        </div></template
      >
      <ManagementDialog
        v-if="managementTarget"
        :key="tab + managementTarget.id"
        :kind="tab"
        :record="managementTarget"
        @close="managementTarget = null"
        @saved="load"
      />
      <BaseDialog
        v-if="reviewTarget"
        :title="selected?.content.name || reviewTarget.title"
        :busy="ui.busy || reviewLoading"
        @close="closeReview"
      >
        <p v-if="reviewLoading" role="status">正在加载物品和认领核验材料…</p>
        <div v-if="reviewError" class="request-error" role="alert">
          {{ reviewError
          }}<button
            class="secondary"
            @click="open(reviewTarget.id, reviewTarget.title)"
          >
            重新加载审核资料
          </button>
        </div>
        <template v-if="selected && privateData">
          <div class="section-head">
            <StatusTag :status="selected.status" /><span class="hint"
              >版本 {{ selected.version }} ·
              {{ label(selected.visibility) }}</span
            >
          </div>
          <p>{{ selected.content.description }}</p>
          <p>
            {{ selected.content.location }}
          </p>
          <p v-if="reportId" class="warning">
            当前处理关联反馈：{{ reportId }}
          </p>
          <RouterLink :to="'/items/' + selected.id"
            >打开完整详情与追溯</RouterLink
          >
          <div class="review-workspace">
            <section class="review-candidates">
              <div class="review-tabs">
                <button
                  :class="{ active: reviewTab === 'pending' }"
                  @click="selectClaimTab('pending')"
                >
                  待处理 {{ pendingClaims.length }}</button
                ><button
                  :class="{ active: reviewTab === 'history' }"
                  @click="selectClaimTab('history')"
                >
                  历史申请 {{ historicalClaims.length }}
                </button>
              </div>
              <p class="list-caption">
                {{
                  reviewTab === "pending"
                    ? "选择一份申请进行核验"
                    : "历史申请只读，完整保留原处理记录"
                }}
              </p>
              <button
                v-for="v in visibleClaims"
                :key="v.claim.id"
                class="claim-candidate"
                :class="{ selected: claimId === v.claim.id }"
                @click="claimId = v.claim.id"
              >
                <UserAvatar :name="v.claimant?.displayName" small /><span
                  ><strong>{{ v.claimant?.displayName || "平台用户" }}</strong
                  ><time>{{ time(v.claim.createdAt) }}</time></span
                ><StatusTag :status="v.claim.status" />
              </button>
              <p v-if="!visibleClaims.length" class="workspace-empty compact">
                {{
                  reviewTab === "pending" ? "没有待处理的申请" : "暂无历史申请"
                }}
              </p>
            </section>
            <section class="review-materials">
              <div class="reference-answer">
                <span class="section-kicker">仅管理员可见</span>
                <h3>核验参考答案</h3>
                <p>{{ selected.content.question }}</p>
                <p class="preserve">{{ privateData.answer }}</p>
                <details class="integrity-detail">
                  <summary>
                    {{
                      privateData.answerIntegrity
                        ? "参考答案未发生变化"
                        : "参考答案缺失或完整性异常"
                    }}
                  </summary>
                  <small
                    >核对参考答案是否被改动。</small
                  >
                </details>
              </div>
              <article v-if="selectedClaim" class="claim selected-evidence">
                <div class="record-title">
                  <h3>
                    {{
                      selectedClaim.claimant?.displayName || "平台用户"
                    }}的认领证明
                  </h3>
                  <StatusTag :status="selectedClaim.claim.status" />
                </div>
                <p class="preserve">
                  {{ selectedClaim.evidence || "材料未留存" }}
                </p>
                <time>提交时间 {{ time(selectedClaim.claim.createdAt) }}</time
                ><FeedbackTrail :entries="selectedClaim.feedback || []" />
                <details class="integrity-detail">
                  <summary>
                    {{
                      selectedClaim.integrity
                        ? "提交材料未发生变化"
                        : "材料缺失或完整性异常"
                    }}
                  </summary>
                  <small
                    >用于检查材料是否被改动，仍需人工核对答案。</small
                  >
                </details>
              </article>
            </section>
          </div>
          <div class="review-decision">
            <label
              >本次处理说明<textarea
                v-model="note"
                minlength="2"
                maxlength="500"
                placeholder="说明判断依据，相关申请人可以查看本次处理原因；请勿填写参考答案或他人的私密材料。"
              />
            </label>
            <div
              v-if="
                selected.status === 'CLAIM_PENDING' &&
                selectedClaim?.claim.status === 'PENDING'
              "
              class="actions"
            >
              <button
                @click="action('APPROVE')"
                :disabled="ui.busy || !canApprove"
              >
                通过所选申请</button
              ><button
                class="danger"
                @click="action('REJECT')"
                :disabled="ui.busy || selectedClaim?.claim.status !== 'PENDING'"
              >
                拒绝所选申请</button
              ><small
                >正在处理：{{ selectedClaim.claimant?.displayName }} ·
                {{ time(selectedClaim.claim.createdAt) }}</small
              >
            </div>
            <p v-else class="muted">
              历史申请不提供审核操作；下方可办理物品治理与交接。
            </p>
          </div>
          <details
            class="governance-section"
            :open="selected.status === 'CLAIMED'"
          >
            <summary>物品治理与交接办理</summary>
            <h3>内容治理</h3>
            <div class="actions">
              <button
                class="secondary"
                @click="editing = !editing"
                :disabled="
                  !['FOUND', 'CLAIM_PENDING'].includes(selected.status)
                "
              >
                编辑公开内容</button
              ><button
                class="secondary"
                @click="
                  action(selected.visibility === 'PUBLIC' ? 'HIDE' : 'RESTORE')
                "
                :disabled="ui.busy"
              >
                {{
                  selected.visibility === "PUBLIC" ? "下架" : "恢复展示"
                }}</button
              ><button
                v-if="['FOUND', 'CLAIM_PENDING'].includes(selected.status)"
                class="secondary"
                @click="action('CLOSE')"
                :disabled="ui.busy"
              >
                关闭认领</button
              ><button
                v-if="selected.status === 'CLOSED'"
                class="secondary"
                @click="action('REOPEN')"
                :disabled="ui.busy"
              >
                重新开放</button
              ><button
                v-if="selected.status === 'CLAIMED'"
                class="danger"
                @click="action('REVOKE')"
                :disabled="ui.busy"
              >
                撤销当前确权</button
              ><button
                v-if="selected.status === 'RETURNED'"
                class="danger"
                @click="action('FLAG_FRAUD')"
                :disabled="ui.busy"
              >
                认定虚假交接并纠正积分
              </button>
            </div>
            <template v-if="selected.status === 'CLAIMED'"
              ><h3>预约与归还</h3>
              <p v-if="appointmentLoading" role="status">正在读取交接预约…</p>
              <div v-if="appointmentError" class="request-error" role="alert">
                {{ appointmentError
                }}<button class="plain" @click="loadAppointment(selected.id)">
                  重试预约读取
                </button>
              </div>
              <form
                class="form"
                @submit.prevent="book"
                :aria-busy="appointmentLoading"
              >
                <input
                  v-model="appointment.location"
                  placeholder="交接地点"
                  required
                /><input
                  v-model="appointment.at"
                  type="datetime-local"
                  required
                /><button class="secondary" :disabled="ui.busy">
                  确认交接预约
                </button>
              </form>
              <label
                >物主现场交接码<input
                  v-model="code"
                  maxlength="6"
                  inputmode="numeric"
                  placeholder="六位数字" /></label
              ><button @click="action('RETURN')" :disabled="ui.busy">
                核验并确认归还
              </button></template
            >
            <p class="muted">
              处理原因会随结果通知相关用户，请勿包含私密答案。
            </p>
            <p class="governance-help">
              下架：停止公开展示和新增认领，保留已有申请及业务状态，可恢复展示。关闭认领：结束未确权物品的本轮认领，取消待审核申请，可重新开放；历史记录始终保留。
            </p>
          </details>
          <ContentForm
            v-if="editing"
            :initial="selected.content"
            edit
            @submit="edit"
          />
        </template>
      </BaseDialog>
    </div>
  </section>
  <p v-else class="empty">需要管理员账号。</p>
</template>
