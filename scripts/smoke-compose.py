"""Exercise a new, isolated Compose project through its public HTTP entry point."""
import http.cookiejar
import json
import os
import secrets
import subprocess
import sys
import urllib.request
from pathlib import Path

driver = sys.argv[1]
if driver not in {"sqlite","json","json-dir","redb","lmdb","postgres","mysql","mariadb","mongodb"}:
    raise SystemExit("Unknown database profile")
key = secrets.token_hex(32)
project = "thoughts-smoke-"+secrets.token_hex(5)
root = Path(__file__).resolve().parents[1]
compose = ["docker","compose","-p",project,"-f",str(root/"deploy"/f"compose.{driver}.yaml")]
env = {**os.environ,"TH0UGHTS_MASTER_KEY":key}
client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def docker(*args):
    subprocess.run(compose+list(args),cwd=root,env=env,check=True)
def request(path,data=None):
    body = None if data is None else json.dumps(data).encode()
    req = urllib.request.Request("http://localhost:8080"+path,data=body,headers={"Content-Type":"application/json","X-Th0ughts-Request":"1"})
    with client.open(req,timeout=15) as res:
        return json.load(res)
try:
    docker("up","-d","--build","--wait","--wait-timeout","240")
    request("/auth/bootstrap",{"master":key})
    request("/auth/setup",{"password":secrets.token_urlsafe(24)})
    status = request("/api/v1/update_status", {})
    assert status["managed"] and status["current"] == (root/"updates/VERSION").read_text().strip()
    post = request("/api/v1/posts",{"original":"en","text":{"en":{"body":"Persisted post","published":True}}})
    theme = '<meta name="theme" content="compose" title="Compose">'
    site = request("/api/v1/get_settings", {})
    site["theme"] = "compose"
    site["comments"] = {"mode":"review","images":2,"markdown":True}
    request("/api/v1/publish_theme", {"source": theme, "settings": site})
    docker("down")
    docker("up","-d","--wait","--wait-timeout","180")
    assert request("/api/v1/get_settings", {})["theme"] == "compose"
    assert request("/api/v1/get_theme", {"id": "compose"})["source"] == theme
    assert request("/api/v1/list_comments", {"post":post["id"]})["items"] == []
    assert request("/api/v1/pending_comments", {})["items"] == []
    restored=request("/api/v1/posts/"+post["id"])
    assert restored["text"]["en"]["body"]=="Persisted post"
except Exception:
    docker("logs","--no-color","app")
    raise
finally:
    docker("down","-v")
