import { adminPassword as readAdminPassword, managerAccount } from './credentials.mjs';
import {
  chromium,
  expect,
} from "../frontend/node_modules/@playwright/test/index.mjs";
import fs from "node:fs";
const base = process.env.BASE_URL || "http://127.0.0.1:8088";
const interference =
  ".modal,.modal-backdrop{display:none!important}.modal-open{overflow:auto!important}";
const browser = await chromium.launch({ channel: "msedge", headless: true });
const checks = [];
const adminPassword = readAdminPassword();
const manager = managerAccount();
async function login(page, name, password) {
  await page.getByRole("link", { name: "登录 / 注册", exact: true }).click();
  await page.getByLabel("用户名", { exact: true }).fill(name);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page
    .locator("form")
    .getByRole("button", { name: "登录", exact: true })
    .click();
  await page.getByRole("heading", { name: "待办队列", exact: true }).waitFor();
}
try {
  if (process.env.LEGACY_INDEX_FILE) {
    const legacy = await browser.newPage();
    await legacy.route(base + "/legacy-dialog-check", (route) =>
      route.fulfill({
        status: 200,
        contentType: "text/html",
        body: fs.readFileSync(process.env.LEGACY_INDEX_FILE, "utf8"),
      }),
    );
    await legacy.goto(base + "/legacy-dialog-check#/");
    await login(legacy, manager.username, manager.password);
    await legacy.addStyleTag({ content: interference });
    await legacy
      .locator(".workspace-row")
      .first()
      .getByRole("button", { name: "审核申请", exact: true })
      .click();
    await expect(legacy.locator(".modal-backdrop")).toHaveCount(1);
    await expect(legacy.locator(".modal-backdrop")).toBeHidden();
    if (
      !(await legacy
        .locator("body")
        .evaluate((body) => body.classList.contains("modal-open")))
    )
      throw Error("旧版复现条件不一致");
    checks.push("旧版在通用弹窗隐藏规则下注入节点但不显示，复现截图现象");
    await legacy.close();
  }
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1000 },
  });
  await page.goto(base);
  await login(page, manager.username, manager.password);
  await page.addStyleTag({ content: interference });
  const token = await page.evaluate(() =>
    sessionStorage.getItem("return_token"),
  );
  const pending = await (
    await page.request.get(
      base + "/api/v1/admin/items?status=CLAIM_PENDING&pageSize=100",
      { headers: { Authorization: "Bearer " + token } },
    )
  ).json();
  const itemId = pending.items[0]?.id;
  if (!itemId) throw Error("需要现有待审核虚拟记录");
  const before = await (
    await page.request.get(base + "/api/v1/items/" + itemId)
  ).json();
  await page
    .locator(".workspace-row")
    .first()
    .getByRole("button", { name: "审核申请", exact: true })
    .click();
  const review = page.getByRole("dialog", {
    name: before.content.name,
    exact: true,
  });
  await expect(review).toBeVisible();
  await expect(
    review.getByRole("heading", { name: "核验参考答案", exact: true }),
  ).toBeVisible();
  const styles = await page
    .locator("body")
    .evaluate((body) => ({
      bodyClass: body.className,
      overflow: getComputedStyle(body).overflow,
      display: getComputedStyle(body.querySelector(".cr-dialog-overlay"))
        .display,
    }));
  if (
    styles.overflow !== "hidden" ||
    styles.display !== "grid" ||
    styles.bodyClass.includes("modal-open")
  )
    throw Error("样式隔离未生效");
  checks.push("相同注入规则下，新版审核窗口可见且滚动锁定正常");
  fs.mkdirSync("artifacts/browser", { recursive: true });
  await page.screenshot({
    path: "artifacts/browser/dialog-isolation.png",
    fullPage: true,
    mask: [
      page.locator(".cr-dialog .preserve"),
      page.locator(".cr-dialog .claim p"),
    ],
  });
  await page.keyboard.press("Escape");
  await page
    .locator(".queue-panel")
    .getByRole("button", { name: "查看全部", exact: true })
    .click();
  await page
    .getByRole("row")
    .filter({ hasText: itemId })
    .getByRole("button", { name: "处理", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  checks.push("审核列表处理按钮打开相同共用弹窗");
  await page.getByRole("button", { name: "举报与争议", exact: true }).click();
  const reportButtons = page.getByRole("button", {
    name: "处理反馈",
    exact: true,
  });
  if (await reportButtons.count()) {
    await reportButtons.first().click();
    await expect(
      page.getByRole("dialog", { name: "处理举报与争议", exact: true }),
    ).toBeVisible();
    await page.keyboard.press("Escape");
    checks.push("反馈处理弹窗在注入规则下可见");
  } else throw Error("缺少待处理反馈的现有测试记录");
  await page.getByRole("button", { name: "评价治理", exact: true }).click();
  const evaluationButtons = page.getByRole("button", {
    name: "管理评价",
    exact: true,
  });
  await evaluationButtons.first().click();
  await expect(
    page.getByRole("dialog", { name: "评价治理", exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  checks.push("评价治理弹窗在注入规则下可见");
  await page.getByRole("button", { name: "退出", exact: true }).click();
  await page.goto(base);
  await login(page, "admin", adminPassword);
  await page.getByRole("button", { name: "用户与权限", exact: true }).click();
  await page.getByPlaceholder("搜索记录", { exact: true }).fill("account_test");
  await page.getByRole("button", { name: "筛选", exact: true }).click();
  await page
    .getByRole("row")
    .filter({ hasText: "account_test" })
    .getByRole("button", { name: "管理账号", exact: true })
    .click();
  await expect(
    page.getByRole("dialog", { name: "管理用户：account_test", exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  checks.push("超管账号管理弹窗在注入规则下可见，未保存任何权限变更");
  await page.getByRole("button", { name: "退出", exact: true }).click();
  const after = await (
    await page.request.get(base + "/api/v1/items/" + itemId)
  ).json();
  if (after.version !== before.version || after.status !== before.status)
    throw Error("测试期间申请状态改变");
  const result = {
    ok: true,
    checkedAt: new Date().toISOString(),
    itemId,
    status: after.status,
    version: after.version,
    checks,
  };
  fs.writeFileSync(
    "artifacts/dialog-isolation-check.json",
    JSON.stringify(result, null, 2),
  );
  console.log(JSON.stringify(result));
} finally {
  await browser.close();
}
