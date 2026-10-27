<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import QRCode from "qrcode";
import { api, safe, mutate, session, label, time, ui, notify } from "../api";
import ItemCard from "../components/ItemCard.vue";
import StatusTag from "../components/StatusTag.vue";
import UserAvatar from "../components/UserAvatar.vue";
import FeedbackTrail from "../components/FeedbackTrail.vue";
const retryClaim = ref(false);
const myFeedback = computed(() =>
  (item.value?.myClaimFeedback || []).filter(
    (f: any) => f.claimId === myClaim.value?.id,
  ),
);
const ownEvaluationExists = ref(false);
const alreadyRated = computed(
  () => ownEvaluationExists.value || evaluations.value.some((e: any) => e.mine),
);
async function copyID() {
  await safe(
    async () => {
      await navigator.clipboard.writeText(id);
      notify("物品编号已复制");
    },
    { busy: false },
  );
}

async function copyTx(value: string) {
  await safe(async () => { await navigator.clipboard.writeText(value); notify("交易编号已复制"); }, { busy: false });
}
const route = useRoute();
const router = useRouter();
const id = String(route.params.id);
const item = ref<any>(null),
  history = ref<any[]>([]),
  similar = ref<any[]>([]),
  evaluations = ref<any[]>([]);
const evidence = ref(""),
  reason = ref(""),
  kind = ref("REPORT"),
  stars = ref(5),
  review = ref(""),
  qr = ref(""),
  code = ref<any>(null),
  appointment = ref<any>({ location: "", at: "", status: "" });
const myClaim = computed(() =>
  [...(item.value?.claims || [])]
    .reverse()
    .find((claim: any) => claim.userId === session.user?.id),
);
const hasPendingClaim = computed(() => myClaim.value?.status === "PENDING");
async function load() {
  await safe(
    async () => {
      item.value = await api("/items/" + id);
      [history.value, similar.value, evaluations.value] = await Promise.all([
        api("/items/" + id + "/history"),
        api("/items/" + id + "/similar"),
        api("/items/" + id + "/evaluations"),
      ]);
      if (
        session.user &&
        item.value.status === "RETURNED" &&
        [item.value.ownerId, item.value.reporterId].includes(session.user.id)
      ) {
        const own = await api("/me/evaluations?itemId=" + id);
        ownEvaluationExists.value = own.total > 0;
      }
      if (
        session.user &&
        (item.value.ownerId === session.user.id ||
          item.value.reporterId === session.user.id)
      )
        appointment.value = await api("/items/" + id + "/appointment");
      if (appointment.value.at) {
        const d = new Date(appointment.value.at);
        appointment.value.at = new Date(
          d.getTime() - d.getTimezoneOffset() * 60000,
        )
          .toISOString()
          .slice(0, 16);
      }
    },
    { busy: false },
  );
}
onMounted(load);
async function claim() {
  await safe(async () => {
    const operation = await mutate("/items/" + id + "/claims", {
      evidence: evidence.value,
      expectedVersion: item.value.version,
    });
    if (operation.status === "SUCCEEDED") {
      evidence.value = "";
      notify("认领申请已提交，等待管理员审核");
      await router.push({ path: "/me", query: { tab: "claims", item: id } });
    } else {
      await router.push({ path: "/me", query: { tab: "operations" } });
    }
  });
}
async function withdraw() {
  if (!hasPendingClaim.value || !confirm("确定撤回这次认领申请？")) return;
  await safe(async () => {
    const operation = await mutate(
      "/items/" + id + "/claims/" + myClaim.value.id + "/withdraw",
      { expectedVersion: item.value.version },
    );
    if (operation.status === "SUCCEEDED") {
      notify("认领申请已撤回");
      await load();
    }
  });
}
async function report() {
  await safe(async () => {
    await api("/reports", "POST", {
      itemId: id,
      kind: kind.value,
      reason: reason.value,
    });
    reason.value = "";
    notify("反馈已提交，可在个人中心跟踪");
  });
}
async function issue() {
  await safe(async () => {
    code.value = await api("/items/" + id + "/handover-code", "POST", {});
    qr.value = await QRCode.toDataURL(code.value.payload, {
      width: 220,
      margin: 1,
    });
  });
}
async function rate() {
  await safe(async () => {
    await api("/items/" + id + "/evaluations", "POST", {
      stars: stars.value,
      text: review.value,
    });
    notify("评价已提交");
  });
  await load();
}
async function book() {
  await safe(async () => {
    appointment.value = await api("/items/" + id + "/appointment", "POST", {
      location: appointment.value.location,
      at: new Date(appointment.value.at).toISOString(),
    });
    notify("预约已提交，等待管理员确认");
  });
}
</script>
<template>
  <section v-if="item" class="item-detail">
    <nav class="detail-breadcrumb">
      <RouterLink to="/">招领大厅</RouterLink><span>/</span
      ><span>{{ item.content.category }}</span>
    </nav>
    <header class="detail-heading">
      <div>
        <div class="detail-status">
          <StatusTag :status="item.status" /><span
            v-if="item.visibility !== 'PUBLIC'"
            class="badge"
            >已下架</span
          >
        </div>
        <h1>{{ item.content.name }}</h1>
        <p>
          {{ item.content.location }}<span> · </span>拾到于
          {{ time(item.content.foundAt) }}
        </p>
      </div>
      <RouterLink to="/">返回大厅 ↗</RouterLink>
    </header>
    <div class="detail-grid">
      <div class="detail-story">
        <div class="detail-image-frame">
          <img
            v-if="item.content.uploadId"
            class="detail-photo"
            :src="'/uploads/' + item.content.uploadId"
            :alt="item.content.name"
          />
          <div v-else class="detail-no-photo">
            {{ item.content.category }}<span>暂无物品图片</span>
          </div>
        </div>
        <section class="detail-section">
          <h2>基本信息</h2>
          <p class="preserve detail-description">
            {{ item.content.description }}
          </p>
          <dl>
            <dt>拾到地点</dt>
            <dd>{{ item.content.location }}</dd>
            <dt>拾到时间</dt>
            <dd>{{ time(item.content.foundAt) }}</dd>
            <dt>登记人</dt>
            <dd>
              <span class="inline-person"
                ><UserAvatar :name="item.reporterName" small />{{
                  item.reporterName || "平台用户"
                }}</span
              >
            </dd>
            <dt>登记时间</dt>
            <dd>{{ time(item.createdAt) }}</dd>
          </dl>
        </section>
        <section class="detail-section verification-question">
          <span class="section-kicker">认领前请确认</span>
          <h2>核验问题</h2>
          <p>{{ item.content.question }}</p>
          <small
            >请在认领证明中回答，并补充只有物主知道的细节。参考答案不公开。</small
          >
        </section>
        <section v-if="item.status === 'RETURNED'" class="detail-section">
          <div class="section-head">
            <h2>交接评价</h2>
            <span>{{ evaluations.length }} 条评价</span>
          </div>
          <article v-for="v in evaluations" :key="v.id" class="review-entry">
            <UserAvatar :name="v.displayName" />
            <div>
              <div class="review-author">
                <strong>{{ v.displayName || "平台用户" }}</strong
                ><span>{{ v.participantRole }}</span>
              </div>
              <div class="review-stars">
                {{ "★".repeat(v.stars) }}<time>{{ time(v.at) }}</time>
              </div>
              <p>{{ v.text }}</p>
            </div>
          </article>
          <p v-if="!evaluations.length" class="muted">
            交接已完成，等待双方分享体验。
          </p>
          <form
            v-if="
              session.user &&
              (item.ownerId === session.user.id ||
                item.reporterId === session.user.id) &&
              !alreadyRated
            "
            class="form review-form"
            @submit.prevent="rate"
          >
            <h3>分享本次交接体验</h3>
            <label
              >体验评分<select v-model="stars">
                <option v-for="n in 5" :key="n" :value="n">{{ n }} 星</option>
              </select></label
            ><label
              >评价内容<textarea
                v-model="review"
                required
                minlength="2"
                maxlength="300"
                placeholder="例如核验是否清楚、交接地点是否方便"
              /></label
            ><button :disabled="ui.busy">提交评价</button>
          </form>
          <RouterLink v-else-if="alreadyRated" to="/me?tab=evaluations"
            >已提交评价，查看我的评价 →</RouterLink
          >
        </section>
        <details class="detail-section integrity-detail">
          <summary>物品编号与存证详情</summary>
          <div class="identifier-row">
            <code class="break">{{ item.id }}</code
            ><button class="secondary" @click="copyID">复制编号</button>
          </div>
          <p :class="item.integrity ? 'positive' : 'negative'">
            {{
              item.integrity
                ? "公开内容未发生变化"
                : "公开内容完整性异常，请联系管理员"
            }}
          </p>
          <small
            >用于检查本版本内容是否被改动。</small
          ><code class="break content-digest">{{ item.contentHash }}</code>
        </details>
      </div>
      <aside class="detail-action-column">
        <section
          v-if="myClaim"
          class="panel claim-progress"
          aria-label="我的认领状态"
        >
          <span class="section-kicker">本次办理</span>
          <h2>我的认领申请</h2>
          <StatusTag :status="myClaim.status" />
          <p v-if="hasPendingClaim">
            申请已进入审核队列，无需重复填写。管理员处理后会通知你。
          </p>
          <p v-else-if="myClaim.status === 'APPROVED'">
            认领已通过，请安排线下交接，到场后向管理员出示交接码。
          </p>
          <p v-else-if="myClaim.status === 'REJECTED'">
            本次申请未通过审核。请先阅读原因，再决定是否补充材料。
          </p>
          <p v-else-if="myClaim.status === 'RETURNED'">
            物品已交接归还，你可以在下方分享交接体验。
          </p>
          <p v-else>本次申请已结束，可在个人中心查看完整处理记录。</p>
          <FeedbackTrail :entries="myFeedback" compact /><time
            >申请时间 {{ time(myClaim.createdAt) }}</time
          >
          <div class="actions">
            <RouterLink
              :to="{ path: '/me', query: { tab: 'claims', item: id } }"
              >查看历次申请 →</RouterLink
            ><button
              v-if="hasPendingClaim"
              class="secondary"
              :disabled="ui.busy"
              @click="withdraw"
            >
              撤回认领
            </button>
          </div>
        </section>
        <section
          class="panel claim-form-panel"
          v-if="
            session.user?.role === 'USER' &&
            session.user.id !== item.reporterId &&
            ['FOUND', 'CLAIM_PENDING'].includes(item.status) &&
            !hasPendingClaim &&
            item.visibility === 'PUBLIC'
          "
        >
          <template v-if="myClaim && !retryClaim"
            ><h3>补充证明后再试</h3>
            <p>同一物品可以重新申请，之前的处理记录仍会保留。</p>
            <button @click="retryClaim = true">
              补充材料并重新申请
            </button></template
          >
          <template v-else
            ><span class="section-kicker">找回你的物品</span>
            <h2>{{ myClaim ? "重新提交认领" : "提交认领" }}</h2>
            <div class="claim-question">
              <strong>请回答</strong>
              <p>{{ item.content.question }}</p>
            </div>
            <form class="form" @submit.prevent="claim">
              <label
                >认领证明<textarea
                  v-model="evidence"
                  required
                  minlength="2"
                  maxlength="2000"
                  placeholder="回答上方问题，并补充物品特征"
                /></label
              ><small>证明原文只供管理员核验，不向其他申请人公开。</small
              ><button :disabled="ui.busy">提交申请</button
              ><button
                v-if="myClaim"
                type="button"
                class="plain"
                @click="retryClaim = false"
              >
                暂不申请
              </button>
            </form></template
          >
        </section>
        <section v-if="!session.user" class="panel">
          <h2>这是你的物品吗？</h2>
          <p>登录后回答核验问题，由管理员协助确认。</p>
          <RouterLink class="btn btn-primary" to="/login"
            >登录后提交认领</RouterLink
          >
        </section>
        <section
          v-if="session.user?.id === item.reporterId"
          class="panel reporter-summary"
        >
          <span class="section-kicker">我登记的物品</span>
          <h2>归还进展</h2>
          <p>
            {{
              item.status === "RETURNED"
                ? "物品已完成交接，感谢你的帮助。"
                : item.status === "CLAIMED"
                  ? "管理员已确认认领人，等待线下交接。"
                  : item.status === "CLAIM_PENDING"
                    ? "已收到认领申请，由管理员核验材料。"
                    : item.status === "CLOSED"
                      ? "本次认领流程已关闭。"
                      : "物品正在等待失主认领。"
            }}
          </p>
          <RouterLink to="/me?tab=notices">查看处理动态 →</RouterLink>
        </section>
        <section
          v-if="item.status === 'CLAIMED' && session.user?.id === item.ownerId"
          class="panel"
        >
          <h2>安排交接</h2>
          <form class="form" @submit.prevent="book">
            <label
              >建议交接地点<input
                v-model="appointment.location"
                required
                placeholder="例如图书馆一楼服务台" /></label
            ><label
              >建议交接时间<input
                v-model="appointment.at"
                type="datetime-local"
                required /></label
            ><button :disabled="ui.busy">提交预约</button>
          </form>
          <p>
            预约状态：{{
              appointment.status === "CONFIRMED"
                ? "管理员已确认"
                : appointment.status === "PROPOSED"
                  ? "等待管理员确认"
                  : "尚未预约"
            }}
          </p>
          <button @click="issue" :disabled="ui.busy">生成十分钟交接码</button>
          <div v-if="code" class="handover-pass">
            <img :src="qr" alt="交接二维码" />
            <h2>{{ code.code }}</h2>
            <small
              >有效至 {{ time(code.expiresAt) }}，请到场后向管理员出示。</small
            >
          </div>
        </section>
        <details v-if="session.user" class="panel report-form-panel">
          <summary>反馈问题与交接争议</summary>
          <form class="form" @submit.prevent="report">
            <label
              >反馈类型<select v-model="kind">
                <option value="REPORT">举报信息</option>
                <option
                  v-if="
                    item.ownerId === session.user.id ||
                    item.reporterId === session.user.id
                  "
                  value="DISPUTE"
                >
                  交接争议
                </option>
              </select></label
            ><label
              >具体说明<textarea
                v-model="reason"
                required
                minlength="5"
                maxlength="1000"
                placeholder="说明问题及依据"
              /></label
            ><button class="secondary" :disabled="ui.busy">提交反馈</button
            ><RouterLink to="/me?tab=reports">查看我的反馈进度 →</RouterLink>
          </form>
        </details>
      </aside>
    </div>
    <details class="detail-section trace-section">
      <summary>操作追溯 · {{ history.length }} 个版本</summary>
      <article
        class="history-entry"
        v-for="h in history"
        :key="h.state.version"
      >
        <div>
          <strong>{{ label(h.state.events.at(-1)?.action) }}</strong
          ><StatusTag :status="h.state.status" />
        </div>
        <time>{{ time(h.state.updatedAt) }} · 版本 {{ h.state.version }}</time>
        <details>
          <summary>查看链上存证</summary>
          <code class="break">{{ h.state.events.at(-1)?.txId }}</code
          ><button class="plain trace-copy" @click="copyTx(h.state.events.at(-1)?.txId)">复制交易编号</button
          ><small
            >本版本公开内容{{
              h.integrity ? "与留存摘要一致" : "完整性异常"
            }}</small
          >
        </details>
      </article>
    </details>
    <section v-if="similar.length" class="similar-section">
      <div class="section-head">
        <h2>相关拾物记录</h2>
        <span class="muted">按分类、地点和名称匹配</span>
      </div>
      <div class="cards">
        <div v-for="v in similar.slice(0, 3)" :key="v.item.id">
          <ItemCard :item="v.item" />
        </div>
      </div>
    </section>
  </section>
</template>
