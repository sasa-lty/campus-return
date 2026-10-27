import { reactive } from "vue";
export const session = reactive<{ token: string; user: any }>({
  token: sessionStorage.getItem("return_token") || "",
  user: null,
});
export const ui = reactive({ message: "", busy: false, reloadRequired: false });
let busyCount = 0;
export function notify(text: string) {
  ui.message = text;
  setTimeout(() => {
    if (ui.message === text) ui.message = "";
  }, 8000);
}
export function saveLogin(v: any) {
  session.token = v.token;
  session.user = v.user;
  sessionStorage.setItem("return_token", v.token);
}
export function clearLogin() {
  session.token = "";
  session.user = null;
  sessionStorage.removeItem("return_token");
}
export async function api(
  path: string,
  method = "GET",
  data?: any,
  operationKey?: string,
  signal?: AbortSignal,
) {
  const headers: Record<string, string> = {};
  if (session.token) headers.Authorization = "Bearer " + session.token;
  if (data !== undefined && !(data instanceof FormData))
    headers["Content-Type"] = "application/json";
  if (method !== "GET")
    headers["Idempotency-Key"] = operationKey || crypto.randomUUID();
  let r: Response;
  try {
    r = await fetch("/api/v1" + path, {
      method,
      headers,
      signal,
      body:
        data === undefined
          ? undefined
          : data instanceof FormData
            ? data
            : JSON.stringify(data),
    });
  } catch {
    if (signal?.aborted) throw new Error("读取超时或已取消，请重试。");
    throw new Error("网络连接中断，请稍后重试；当前表单不会清空。");
  }
  if ([502, 503, 504, 530].includes(r.status)) {
    throw new Error("服务入口暂时不可用，请稍后重试；当前表单不会清空。");
  }
  let v: any;
  try {
    v = await r.json();
  } catch {
    throw new Error("未收到完整服务响应，请稍后重试；当前表单不会清空。");
  }
  if (!r.ok) {
    if (r.status === 401) clearLogin();
    throw new Error(v.error || "请求失败");
  }
  return v;
}
export async function mutate(path: string, data: any, method = "POST") {
  const digest = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(
      session.user?.id + method + path + JSON.stringify(data),
    ),
  );
  const signature = Array.from(new Uint8Array(digest))
    .map((x) => x.toString(16).padStart(2, "0"))
    .join("");
  const keys: Record<string, string> = JSON.parse(
    sessionStorage.getItem("return_operations") || "{}",
  );
  const operationKey = keys[signature] || crypto.randomUUID();
  keys[signature] = operationKey;
  sessionStorage.setItem("return_operations", JSON.stringify(keys));
  const completed = () => {
    const latest = JSON.parse(
      sessionStorage.getItem("return_operations") || "{}",
    );
    delete latest[signature];
    sessionStorage.setItem("return_operations", JSON.stringify(latest));
  };
  let v = await api(path, method, data, operationKey);
  if (!v.id || !v.command) return v;
  for (let i = 0; i < 100; i++) {
    if (v.status === "SUCCEEDED") {
      completed();
      notify("操作已完成");
      return v;
    }
    if (v.status === "FAILED") {
      completed();
      throw new Error(v.error || "操作未完成");
    }
    await new Promise((r) => setTimeout(r, 900));
    v = await api("/operations/" + v.id);
  }
  notify("交易仍在确认，可在个人中心查看结果");
  return v;
}
export async function safe(
  fn: () => Promise<any>,
  options: { busy?: boolean } = {},
) {
  const blocking = options.busy !== false;
  try {
    if (blocking) {
      busyCount++;
      ui.busy = true;
    }
    return await fn();
  } catch (e: any) {
    notify(e.message);
    return null;
  } finally {
    if (blocking) {
      busyCount--;
      ui.busy = busyCount > 0;
    }
  }
}
export const labels: Record<string, string> = {
  FOUND: "待认领",
  CLAIM_PENDING: "审核中",
  CLAIMED: "已确权",
  RETURNED: "已归还",
  CLOSED: "已关闭",
  PUBLIC: "公开",
  HIDDEN: "已下架",
  PENDING: "待审核",
  APPROVED: "已通过",
  REJECTED: "已拒绝",
  WITHDRAWN: "已撤回",
  SUPERSEDED: "未选中",
  CANCELLED: "已取消",
  REVOKED: "已撤销",
  ATTENTION: "待核对与失败",
  QUEUED: "排队中",
  SUBMITTING: "提交中",
  SUCCEEDED: "成功",
  FAILED: "失败",
  OPEN: "待受理",
  IN_PROGRESS: "处理中",
  RESOLVED: "已结案",
  ACTIVE: "启用",
  DISABLED: "禁用",
  USER: "用户",
  ADMIN: "业务管理员",
  SUPER_ADMIN: "超级管理员",
  CREATE: "登记拾物",
  CLAIM: "提交认领",
  APPROVE: "审核通过",
  REJECT: "拒绝认领",
  WITHDRAW: "撤回申请",
  RETURN: "确认归还",
  UPDATE: "编辑内容",
  HIDE: "下架物品",
  RESTORE: "恢复展示",
  CLOSE: "关闭认领",
  REOPEN: "重新开放",
  REVOKE: "撤销确权",
  FLAG_FRAUD: "纠正虚假交接",
};
export function label(s: string) {
  return labels[s] || s;
}
export function time(s: string) {
  return s
    ? new Date(s).toLocaleString("zh-CN", {
        hour12: false,
        timeZone: "Asia/Shanghai",
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      })
    : "—";
}
