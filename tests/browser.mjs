import { adminPassword } from './credentials.mjs';
import {
  chromium,
  expect,
} from "../frontend/node_modules/@playwright/test/index.mjs";
import fs from "node:fs";
const base = process.env.BASE_URL || "http://127.0.0.1:8088";
const password = adminPassword();
const browser = await chromium.launch({ channel: "msedge", headless: true });
const errors = [];
const desktop = await browser.newContext({
  viewport: { width: 1440, height: 1000 },
});
const page = await desktop.newPage();
page.on("pageerror", (e) => errors.push(e.message));
page.setDefaultTimeout(60000);
fs.mkdirSync("artifacts/browser", { recursive: true });
try {
  await page.goto(base);
  await page.getByRole("heading", { name: "招领大厅" }).waitFor();
  await page.screenshot({
    path: "artifacts/browser/hall-desktop.png",
    fullPage: true,
  });
  await page.getByRole("link", { name: "登录 / 注册" }).click();
  await page.getByRole("button", { name: "注册", exact: true }).click();
  const username = "web" + Date.now();
  await page.getByLabel("用户名", { exact: true }).fill(username);
  await page.getByLabel("显示名称", { exact: true }).fill("浏览器验收用户");
  await page.getByLabel("密码", { exact: true }).fill("Web123");
  await page.getByRole("button", { name: "注册并登录", exact: true }).click();
  await page.getByRole("button", { name: "已保存，进入平台" }).click();
  let firstOperation;
  let firstReplyResolve;
  const firstReply = new Promise((r) => (firstReplyResolve = r));
  await page.route("**/api/v1/items", async (route) => {
    if (route.request().method() === "POST" && !firstOperation) {
      const response = await route.fetch();
      firstOperation = await response.json();
      await route.abort("connectionreset");
      firstReplyResolve();
    } else await route.continue();
  });
  await page
    .getByRole("navigation", { name: "主要导航" })
    .getByRole("link", { name: "登记拾物", exact: true })
    .click();
  await page.getByLabel("物品名称", { exact: true }).fill("浏览器验收笔记本");
  await page.getByLabel("拾到地点", { exact: true }).fill("教学楼一楼");
  await page
    .getByLabel("公开描述", { exact: true })
    .fill("仅用于浏览器验收的虚拟物品，封面为绿色。");
  await page.getByLabel("公开核验问题", { exact: true }).fill("封底贴纸文字？");
  await page
    .getByLabel("私密参考答案", { exact: true })
    .fill("WEB-PRIVATE-2026");
  await page.locator("input[type=file]").setInputFiles({
    name: "sample.png", mimeType: "image/png",
    buffer: Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a9N8AAAAASUVORK5CYII=", "base64"),
  });
  await expect(
    page.getByAltText("已选择的物品照片", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("图片已上传，提交时会核对文件。", { exact: true }),
  ).toBeVisible();
  const submit = page.getByRole("button", { name: "提交登记", exact: true });
  await submit.click();
  await firstReply;
  await expect(page.getByRole("status")).toContainText("网络连接中断");
  await expect(page.getByLabel("物品名称", { exact: true })).toHaveValue(
    "浏览器验收笔记本",
  );
  await expect(page.getByLabel("私密参考答案", { exact: true })).toHaveValue(
    "WEB-PRIVATE-2026",
  );
  await expect(submit).toBeEnabled();
  await submit.click();
  await page.waitForURL(/#\/items\//);
  if (!page.url().includes(firstOperation.itemId))
    throw Error("network retry created another item");
  await page
    .getByRole("heading", { name: "浏览器验收笔记本", level: 1, exact: true })
    .waitFor();
  await page.screenshot({
    path: "artifacts/browser/item-desktop.png",
    fullPage: true,
  });
  await page.goto(base + '/#/me');
  await expect(page.getByRole('heading', { name: '个人概览', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: '我的发布 1 查看物品进展', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: '登录与安全' })).toHaveCount(0);
  await page.screenshot({ path: 'artifacts/browser/me-desktop.png', fullPage: true });
  await page.getByRole('button', { name: '我的发布', exact: true }).click();
  await expect(page.getByRole('heading', { name: '浏览器验收笔记本', exact: true })).toBeVisible();
  await page.getByRole('button', { name: '账户设置', exact: true }).click();
  await expect(page.getByRole('button', { name: '更新资料', exact: true })).toBeDisabled();
  await page.getByLabel('显示名称', { exact: true }).fill('浏览器验收用户-资料已更新');
  await page.getByRole('button', { name: '更新资料', exact: true }).click();
  await expect(page.getByRole('button', { name: '更新资料', exact: true })).toBeDisabled();
  await expect(page.locator('.workspace-identity')).toContainText('浏览器验收用户-资料已更新');
  await page.screenshot({ path: 'artifacts/browser/settings-desktop.png', fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(base + '/#/me');
  await expect(page.getByRole('heading', { name: '个人概览', exact: true })).toBeVisible();
  if (await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)) throw Error('mobile personal center overflow');
  await page.screenshot({ path: 'artifacts/browser/me-mobile.png', fullPage: true });
  await page.setViewportSize({ width: 1440, height: 1000 });
  const userToken = await page.evaluate(() => sessionStorage.getItem('return_token'));
  const feedbackReason = '界面验收反馈-' + username;
  const reportResponse = await page.request.post(base + '/api/v1/reports', { headers: { Authorization: 'Bearer ' + userToken, 'Idempotency-Key': crypto.randomUUID() }, data: { itemId: firstOperation.itemId, kind: 'REPORT', reason: feedbackReason } });
  if (reportResponse.status() !== 201) throw Error('feedback fixture creation failed');
  await page.getByRole("button", { name: "退出", exact: true }).click();
  await page.getByLabel("用户名", { exact: true }).fill("admin");
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page
    .locator("form")
    .getByRole("button", { name: "登录", exact: true })
    .click();
  await page.getByRole("heading", { name: "实际业务统计" }).waitFor();
  await page.screenshot({
    path: "artifacts/browser/admin-desktop.png",
    fullPage: true,
  });
  await page.getByRole('button', { name: '用户与权限', exact: true }).click();
  await page.getByPlaceholder('搜索记录', { exact: true }).fill(username);
  await page.getByRole('button', { name: '筛选', exact: true }).click();
  for (const role of ['ADMIN', 'USER']) {
    const row = page.getByRole('row').filter({ hasText: username });
    await row.getByRole('button', { name: '管理账号', exact: true }).click();
    const dialog = page.getByRole('dialog', { name: '管理用户：' + username, exact: true });
    await dialog.getByLabel('角色', { exact: true }).selectOption(role);
    await dialog.getByLabel('操作原因', { exact: true }).fill('虚拟验收账号权限调整');
    page.once('dialog', d => d.accept());
    await dialog.getByRole('button', { name: '保存处理结果', exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(row).toContainText(role === 'ADMIN' ? '业务管理员' : '用户');
  }
  const oldSession = await page.request.get(base + '/api/v1/me', { headers: { Authorization: 'Bearer ' + userToken } });
  if (oldSession.status() !== 401) throw Error('role change failed to revoke session');
  await page.getByRole('button', { name: '举报与争议', exact: true }).click();
  for (const state of ['IN_PROGRESS', 'RESOLVED']) {
    const row = page.getByRole('row').filter({ hasText: feedbackReason });
    await row.getByRole('button', { name: '处理反馈', exact: true }).click();
    const dialog = page.getByRole('dialog', { name: '处理举报与争议', exact: true });
    await dialog.getByLabel('处理进度', { exact: true }).selectOption(state);
    await dialog.getByLabel('处理意见', { exact: true }).fill('虚拟验收反馈办理，流程检查完成');
    await dialog.getByRole('button', { name: '保存处理结果', exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(row).toContainText(state === 'IN_PROGRESS' ? '处理中' : '已结案');
  }
  await page.reload();
  await expect(page.getByRole('heading', { name: '举报与争议', exact: true })).toBeVisible();
  for (const tab of [
    "审核与物品",
    "用户与权限",
    "举报与争议",
    "评价治理",
    "交易进度",
    "操作日志",
  ]) {
    await page.getByRole("button", { name: tab, exact: true }).click();
    await page.getByRole("heading", { name: tab, exact: true }).waitFor();
    await page.waitForTimeout(300);
  }
  await page.getByRole("button", { name: "审核与物品", exact: true }).click();
  await page
    .getByPlaceholder("搜索记录", { exact: true })
    .fill(firstOperation.itemId);
  await page.getByRole("button", { name: "筛选", exact: true }).click();
  const process = page
    .getByRole("row")
    .filter({ hasText: firstOperation.itemId })
    .getByRole("button", { name: "处理", exact: true });
  await process.click();
  const dialog = page.getByRole("dialog", {
    name: "浏览器验收笔记本",
    exact: true,
  });
  await expect(dialog).toBeVisible();
  await expect(
    dialog.getByText("WEB-PRIVATE-2026", { exact: true }),
  ).toBeVisible();
  await expect(
    dialog.getByRole("button", { name: "关闭详情", exact: true }),
  ).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  await expect(
    dialog.getByRole("button", { name: "关闭详情", exact: true }),
  ).not.toBeFocused();
  await page.screenshot({
    path: "artifacts/browser/admin-dialog.png",
    fullPage: true,
  });
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(process).toBeFocused();
  const mobile = await browser.newContext({
    viewport: { width: 390, height: 844 },
    isMobile: true,
    hasTouch: true,
  });
  const mp = await mobile.newPage();
  mp.on("pageerror", (e) => errors.push(e.message));
  await mp.goto(base);
  await mp.getByRole("heading", { name: "招领大厅" }).waitFor();
  await mp.screenshot({
    path: "artifacts/browser/hall-mobile.png",
    fullPage: false,
  });
  if (
    await mp.evaluate(
      () => document.documentElement.scrollWidth > innerWidth + 1,
    )
  )
    throw Error("mobile horizontal overflow");
  await mp.getByRole("link", { name: "浏览招领记录", exact: true }).click();
  await expect(
    mp.getByRole("heading", { name: "招领大厅", exact: true }),
  ).toBeVisible();
  await mp.getByRole("button", { name: "菜单", exact: true }).click();
  await mp
    .getByRole("navigation", { name: "主要导航" })
    .getByRole("link", { name: "登录 / 注册", exact: true })
    .click();
  await mp.getByLabel("用户名", { exact: true }).fill("admin");
  await mp.getByLabel("密码", { exact: true }).fill(password);
  await mp
    .locator("form")
    .getByRole("button", { name: "登录", exact: true })
    .click();
  await mp
    .getByRole("heading", { name: "实际业务统计", exact: true })
    .waitFor();
  if (
    await mp.evaluate(
      () => document.documentElement.scrollWidth > innerWidth + 1,
    )
  )
    throw Error("mobile admin overflow");
  await mp.screenshot({
    path: "artifacts/browser/admin-mobile.png",
    fullPage: true,
  });
  await mobile.close();
  if (errors.length) throw Error(errors.join("\n"));
  const result = {
    ok: true,
    checks: [
      "桌面首页与原卡片组件",
      "六字符密码注册与带图片登记",
      "断网中文提示与已填内容保留",
      "断网响应重试不重复创建",
      "个人中心概览与业务分区",
      "账户设置独立与资料保存反馈",
      "手机个人中心无横向溢出",
      "账号授权弹窗与旧会话失效",
      "反馈受理结案与独立处理意见",
      "管理分区刷新保留",
      "管理端各页面",
      "办理弹窗私密核验",
      "弹窗初始焦点与反向Tab循环",
      "Esc关闭与焦点恢复",
      "移动端大厅",
      "手机导航与管理员工作台无横向溢出",
      "无未处理脚本异常",
    ],
  };
  fs.writeFileSync(
    "artifacts/browser-check.json",
    JSON.stringify(result, null, 2),
  );
  console.log(JSON.stringify(result));
} finally {
  await browser.close();
}
