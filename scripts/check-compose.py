"""Verify every standalone Compose entry point and the PaaS routing contract."""
import json
import os
import pathlib
import subprocess

root = pathlib.Path(__file__).resolve().parents[1]
env = {**os.environ, "TH0UGHTS_MASTER_KEY":"compose-schema-validation-only"}
paths = sorted((root/"deploy").glob("compose.*.yaml"))
assert len(paths) == 9
for path in paths:
    cmd = ["docker","compose","-f",str(path),"config","--format","json"]
    model = json.loads(subprocess.check_output(cmd,env=env))
    app = model["services"]["app"]
    assert pathlib.Path(app["build"]["context"]).resolve() == root
    assert set(app["environment"]) == {"TH0UGHTS_MASTER_KEY"}
    assert any(v["target"]=="/data" and v["type"]=="volume" for v in app["volumes"])
    assert "/healthz" in " ".join(app["healthcheck"]["test"])
    if path.stem in ("compose.coolify","compose.dokploy"):
        assert not app.get("ports") and "8080" in map(str,app["expose"])
        assert not app.get("container_name") and not app.get("labels")
    if "db" in model["services"]:
        assert not model["services"]["db"].get("ports")
        assert model["services"]["db"]["depends_on"]["setup"]["condition"]=="service_completed_successfully"
    print(path.name)
