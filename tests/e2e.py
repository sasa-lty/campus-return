"""真实 HTTP/Fabric 验收。凭据来自环境，不写入测试结果。"""
import base64
import concurrent.futures
import json
import os
import time
import urllib.error
import urllib.request
import uuid
from pathlib import Path

BASE = os.environ.get("BASE_URL", "http://127.0.0.1:8088") + "/api/v1"
SUFFIX = uuid.uuid4().hex[:8]
RESULT = {"mode": "", "items": [], "checks": [], "transactions": []}

def call(method, path, data=None, token="", status=200, key=None):
    headers = {"Content-Type": "application/json", "Idempotency-Key": key or uuid.uuid4().hex}
    if token:
        headers["Authorization"] = "Bearer " + token
    request = urllib.request.Request(BASE + path, data=None if data is None else json.dumps(data).encode(), headers=headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=25) as response:
            code, body = response.status, json.load(response)
    except urllib.error.HTTPError as error:
        code, body = error.code, json.load(error)
    assert code == status, (method, path, code, body)
    return body

def wait(op, token, success=True):
    for _ in range(100):
        current = call("GET", "/operations/" + op["id"], token=token)
        if current["status"] in ("SUCCEEDED", "FAILED"):
            assert (current["status"] == "SUCCEEDED") == success, current
            if success:
                RESULT["transactions"].append({"action": current["command"]["action"], "txId": current["txId"], "itemId": current["itemId"]})
            return current
        time.sleep(0.6)
    raise AssertionError(("operation timeout", op["id"]))

def login(name, password):
    return call("POST", "/auth/login", {"username": name, "password": password})["token"]

def register(prefix):
    name = prefix + SUFFIX
    user = call("POST", "/auth/register", {"username": name, "password": "Sample-Only-2026", "displayName": "验收用户"}, status=201)
    return {**user, "username": name, "token": login(name, "Sample-Only-2026")}

def run():
    health = call("GET", "/health")
    RESULT["mode"] = health["mode"]
    assert health["mode"] == "hyperledger-fabric", "本测试必须连接真实 Fabric"
    admin = login("admin", os.environ["ADMIN_PASSWORD"])
    finder, bob, carol, manager = [register(name) for name in ("finder", "bob", "carol", "manager")]
    call("PATCH", "/admin/users/" + manager["user"]["id"], {"role": "ADMIN", "reason": "验收业务管理员授权"}, admin)
    call("GET", "/me", token=manager["token"], status=401)
    manager["token"] = login(manager["username"], "Sample-Only-2026")
    call("GET", "/admin/users", token=manager["token"], status=403)
    call("PATCH", "/admin/users/" + bob["user"]["id"], {"role": "SUPER_ADMIN", "reason": "越权尝试"}, manager["token"], status=403)
    payload = {"content": {"name": "验收保温杯 " + SUFFIX, "category": "生活用品", "description": "虚拟物品，用于课程功能与链上验收", "location": "图书馆一楼", "foundAt": "2026-01-01T00:00:00Z", "question": "杯底刻字是什么？"}, "answer": "PRIVATE-ANSWER-" + SUFFIX}
    key = uuid.uuid4().hex
    op = call("POST", "/items", payload, finder["token"], 202, key)
    wait(op, finder["token"])
    item_id = op["itemId"]
    RESULT["items"].append(item_id)
    assert call("POST", "/items", payload, finder["token"], 202, key)["id"] == op["id"]
    RESULT["checks"].append("创建幂等与角色变更会话撤销")

    def item():
        return call("GET", "/items/" + item_id, token=admin)
    def action(action, claim="", success=True, **extra):
        args = {"action": action, "claimId": claim, "note": "课程验收：" + action, "expectedVersion": item()["version"], **extra}
        return wait(call("POST", "/admin/items/" + item_id + "/actions", args, manager["token"], 202), manager["token"], success)
    edit = {**payload, "answer": "", "reason": "更正虚拟物品公开描述", "expectedVersion": item()["version"]}
    edit["content"] = {**payload["content"], "description": "更正后的虚拟物品描述，用于核验版本摘要"}
    wait(call("PATCH", "/admin/items/" + item_id, edit, manager["token"], 202), manager["token"])
    assert item()["integrity"]
    action("HIDE")
    call("GET", "/items/" + item_id, status=404)
    action("RESTORE")
    self_op = call("POST", "/items/" + item_id + "/claims", {"evidence": "登记人自认领", "expectedVersion": item()["version"]}, finder["token"], 202)
    wait(self_op, finder["token"], False)
    version = item()["version"]
    def submit(user):
        return call("POST", "/items/" + item_id + "/claims", {"evidence": "PRIVATE-PROOF-" + user["username"], "expectedVersion": version}, user["token"], 202)
    with concurrent.futures.ThreadPoolExecutor(2) as pool:
        submitted = list(pool.map(submit, [bob, carol]))
    claims = []
    for user, submitted_op in zip([bob, carol], submitted):
        for _ in range(100):
            current = call("GET", "/operations/" + submitted_op["id"], token=user["token"])
            if current["status"] in ("SUCCEEDED", "FAILED"):
                break
            time.sleep(.4)
        if current["status"] == "FAILED":
            submitted_op = call("POST", "/items/" + item_id + "/claims", {"evidence": "PRIVATE-PROOF-" + user["username"], "expectedVersion": item()["version"]}, user["token"], 202)
        wait(submitted_op, user["token"])
        claims.append(submitted_op["command"]["data"]["claimId"])
    private = call("GET", "/admin/items/" + item_id + "/private", token=manager["token"])
    assert private["answerIntegrity"] and all(v["integrity"] for v in private["claims"])
    call("GET", "/admin/items/" + item_id + "/private", token=bob["token"], status=403)
    forbidden_edit = {**edit, "content": {**edit["content"], "question": "试图在认领后更换问题"}, "expectedVersion": item()["version"]}
    wait(call("PATCH", "/admin/items/" + item_id, forbidden_edit, manager["token"], 202), manager["token"], False)
    public = json.dumps(call("GET", "/items/" + item_id)) + json.dumps(call("GET", "/items/" + item_id + "/history"))
    assert "PRIVATE-ANSWER" not in public and "PRIVATE-PROOF" not in public
    call("POST", "/admin/items/" + item_id + "/actions", {"action": "APPROVE", "claimId": claims[0], "note": "旧版本拒绝", "expectedVersion": 1}, manager["token"], status=409)
    action("REJECT", claims[0])
    assert item()["status"] == "CLAIM_PENDING"
    wait(call("POST", "/items/" + item_id + "/claims/" + claims[1] + "/withdraw", {"expectedVersion": item()["version"]}, carol["token"], 202), carol["token"])
    assert item()["status"] == "FOUND"
    submitted_op = call("POST", "/items/" + item_id + "/claims", {"evidence": "PRIVATE-PROOF-RESUBMIT", "expectedVersion": item()["version"]}, bob["token"], 202)
    wait(submitted_op, bob["token"])
    claim_id = submitted_op["command"]["data"]["claimId"]
    action("APPROVE", claim_id)
    appointment = {"location": "图书馆服务台", "at": "2027-01-01T09:00:00Z"}
    call("POST", "/items/" + item_id + "/appointment", appointment, bob["token"])
    assert call("POST", "/items/" + item_id + "/appointment", appointment, manager["token"])["status"] == "CONFIRMED"
    handover = call("POST", "/items/" + item_id + "/handover-code", {}, bob["token"])
    call("POST", "/admin/items/" + item_id + "/actions", {"action": "RETURN", "claimId": claim_id, "note": "错误交接码拒绝", "code": "wrong!", "expectedVersion": item()["version"]}, manager["token"], status=400)
    action("RETURN", claim_id, code=handover["code"])
    assert call("GET", "/me/points", token=finder["token"])["score"] == 10
    assert call("GET", "/me/points", token=bob["token"])["score"] == 2
    evaluation = call("POST", "/items/" + item_id + "/evaluations", {"stars": 5, "text": "虚拟验收：交接顺利"}, bob["token"], 201)
    call("PATCH", "/admin/evaluations/" + evaluation["id"], {"status": "HIDDEN", "reason": "测试评价治理"}, manager["token"])
    assert call("GET", "/items/" + item_id + "/evaluations") == []
    report = call("POST", "/reports", {"kind": "DISPUTE", "itemId": item_id, "reason": "虚拟验收争议，用于核验纠正流程"}, finder["token"], 201)
    call("PATCH", "/admin/reports/" + report["id"], {"status": "IN_PROGRESS", "resolution": "已受理核查"}, manager["token"])
    action("FLAG_FRAUD", claim_id, reportId=report["id"])
    assert call("GET", "/me/points", token=finder["token"])["score"] == 0
    assert item()["status"] == "RETURNED"
    assert call("GET", "/notifications", token=finder["token"])["total"] > 0
    call("PATCH", "/admin/users/" + bob["user"]["id"], {"status": "DISABLED", "reason": "验证禁用账号旧会话"}, admin)
    call("GET", "/me", token=bob["token"], status=401)
    call("PATCH", "/admin/users/" + bob["user"]["id"], {"status": "ACTIVE", "reason": "恢复验收账号"}, admin)
    call("GET", "/me", token=bob["token"], status=401)
    call("POST", "/auth/recover", {"username": bob["username"], "recoveryCode": bob["recoveryCode"], "password": "Changed-Pass-2026"})
    bob["token"] = login(bob["username"], "Changed-Pass-2026")
    second_payload = {**payload, "content": {**payload["content"], "name": "验收钥匙 " + SUFFIX}}
    second_op = call("POST", "/items", second_payload, finder["token"], 202)
    wait(second_op, finder["token"])
    item_id = second_op["itemId"]
    RESULT["items"].append(item_id)
    action("CLOSE")
    assert item()["status"] == "CLOSED"
    action("REOPEN")
    assert item()["status"] == "FOUND"
    claim_op = call("POST", "/items/" + item_id + "/claims", {"evidence": "PRIVATE-PROOF-REVOKE", "expectedVersion": item()["version"]}, bob["token"], 202)
    wait(claim_op, bob["token"])
    claim_id = claim_op["command"]["data"]["claimId"]
    action("APPROVE", claim_id)
    action("REVOKE", claim_id)
    assert item()["status"] == "FOUND" and not item()["ownerId"]
    reset = call("POST", "/admin/users/" + carol["user"]["id"] + "/reset", {"reason": "验收管理员重置与首次改密"}, admin)
    call("GET", "/me", token=carol["token"], status=401)
    temporary_token = login(carol["username"], reset["temporaryPassword"])
    call("GET", "/me/points", token=temporary_token, status=403)
    call("POST", "/me/password", {"oldPassword": reset["temporaryPassword"], "password": "New-Carol-2026"}, temporary_token)
    call("GET", "/me", token=temporary_token, status=401)
    assert not call("GET", "/me", token=login(carol["username"], "New-Carol-2026"))["forcePassword"]
    super_id = call("GET", "/me", token=admin)["id"]
    call("PATCH", "/admin/users/" + super_id, {"role": "USER", "reason": "验证超管自我降权保护"}, admin, status=400)
    RESULT["checks"] += ["真实双组织链上闭环", "并发认领与旧版本拒绝", "拒绝撤回后再次认领", "私密材料摘要与公开脱敏", "编辑和上下架", "认领后核验问题不可更换", "预约与一次性交接码", "评价治理与消息", "积分及争议纠正", "禁用恢复与密码找回", "关闭归档与重新开放", "撤销确权并保留历史", "管理员重置凭据与首次强制改密", "超管自我降权保护"]
    out = Path(os.environ.get("EVIDENCE_DIR", "artifacts"))
    out.mkdir(parents=True, exist_ok=True)
    (out / "fabric-e2e.json").write_text(json.dumps(RESULT, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"ok": True, "checks": RESULT["checks"], "itemId": item_id, "transactions": len(RESULT["transactions"])}, ensure_ascii=False))

if __name__ == "__main__":
    run()
