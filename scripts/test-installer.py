"""Run the streamed VPS wizard against a real Docker daemon on a disposable CI host."""
import functools
import http.server
import json
import os
import pathlib
import re
import secrets
import shlex
import socket
import subprocess
import sys
import tempfile
import threading
import urllib.request

root = pathlib.Path(__file__).resolve().parents[1]
choices = {"sqlite": "1", "postgres": "2", "redb": "8", "lmdb": "9"}
driver = sys.argv[1]
project = "thoughts-wizard-" + secrets.token_hex(4)
with tempfile.TemporaryDirectory() as temporary:
    area = pathlib.Path(temporary)
    destination = area / "my feed"
    scripts = area / "web"
    scripts.mkdir()
    ref = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
    source = (root / "deploy/install.sh").read_text().replace("\nSOURCE_REF=main\n", "\nSOURCE_REF="+ref+"\n")
    (scripts / "install.sh").write_text(source)
    handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=scripts)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        port = s.getsockname()[1]
    url = f"http://127.0.0.1:{server.server_port}/install.sh"
    command = f"curl -fsSL {url} | bash"
    if os.getenv("GITHUB_EVENT_NAME") == "pull_request":
        command += " -s -- --source " + shlex.quote(str(root))
    def wizard(settings, check=True):
        return subprocess.run(["script","-qefc",command,"/dev/null"], input="\n".join(settings)+"\n", text=True, check=check, timeout=900)
    manage = destination / "manage"
    def compose(*args):
        return subprocess.check_output([manage,*args], text=True)
    try:
        lang = "en" if driver=="sqlite" else "tr"
        wizard([lang, choices[driver], str(port), "127.0.0.1", str(destination), project, "y"])
        assert (destination.stat().st_mode & 0o777) == 0o700
        envfile = destination / ".env"
        assert (envfile.stat().st_mode & 0o777) == 0o600
        assert re.fullmatch(r"TH0UGHTS_MASTER_KEY=[0-9a-f]{64}\n", envfile.read_text())
        assert compose("port","app","8080").strip() == f"127.0.0.1:{port}"
        config = json.loads(compose("config","--format","json"))
        assert set(config["services"]["app"]["environment"]) == {"TH0UGHTS_MASTER_KEY"}
        assert driver in config["services"]["app"]["command"]
        assert subprocess.check_output(["git","rev-parse","HEAD"], cwd=destination/"source", text=True).strip()==ref
        with urllib.request.urlopen(f"http://127.0.0.1:{port}/healthz",timeout=5) as response:
            assert response.read()==b"ok"
        assert wizard(["en","1",str(port),"127.0.0.1",str(destination)],check=False).returncode!=0
        print(f"Verified interactive {driver} installation on port {port}.")
    finally:
        if manage.exists():
            subprocess.run([manage,"down","-v"], check=True)
        server.shutdown()
        server.server_close()
