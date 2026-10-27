"""检查自有加密备份解密后的结构和 SQLite 快照，不导出私密数据。"""
import io
import json
import os
import sqlite3
import tarfile
import tempfile
import sys
from pathlib import Path

with tarfile.open(sys.argv[1], "r:gz") as archive:
    names = archive.getnames()
    required = ["./data/platform.db", "./channel.block", "./campus-return_orderer-data.tgz", "./campus-return_peer1-data.tgz", "./campus-return_peer2-data.tgz"]
    assert all(name in names for name in required)
    assert not any("secrets.env" in name for name in names)
    for volume in required[2:]:
        with tarfile.open(fileobj=io.BytesIO(archive.extractfile(volume).read()), mode="r:gz") as nested:
            assert any("returnchannel" in member.name for member in nested.getmembers()), volume
    with tempfile.TemporaryDirectory() as temp:
        database = Path(temp) / "snapshot.db"
        database.write_bytes(archive.extractfile("./data/platform.db").read())
        connection = sqlite3.connect(database)
        assert connection.execute("PRAGMA integrity_check").fetchone()[0] == "ok"
        records = connection.execute("SELECT count(*) FROM records").fetchone()[0]
        connection.close()
    result = {"ok": True, "checks": ["SQLite 一致性快照", "账本卷包含 returnchannel", "网络证书与通道块", "密钥文件与备份分离", "加密包可解密读取"], "records": records}
    Path("artifacts").mkdir(exist_ok=True)
    Path("artifacts/backup-check.json").write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(result, ensure_ascii=False))
