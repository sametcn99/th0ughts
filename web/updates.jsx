import React, { useEffect, useState } from "react";
import { api, t } from "./shared.jsx";

const releases = "https://github.com/sametcn99/th0ughts/releases";

export default function Updates() {
  const [info, setInfo] = useState(null);
  const [checked, setChecked] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [target, setTarget] = useState("");
  const [installed, setInstalled] = useState(false);

  useEffect(() => {
    let active = true;
    api("update_status").then(v => { if (active) setInfo(v); })
      .catch(e => { if (active) setError(e.message); });
    return () => { active = false; };
  }, []);

  useEffect(() => {
    if (!target) return;
    let active = true, timer;
    const deadline = Date.now() + 180000;
    async function poll() {
      try {
        const v = await api("update_status");
        if (!active) return;
        if (v.current === target || v.failed === target) {
          setInfo(v);
          setInstalled(v.current === target);
          setChecked(false);
          setTarget("");
          return;
        }
      } catch { /* The server may be restarting. */ }
      if (!active) return;
      if (Date.now() > deadline) {
        setTarget("");
        setError(t("update_wait_timeout"));
        return;
      }
      timer = setTimeout(poll, 2000);
    }
    timer = setTimeout(poll, 1000);
    return () => { active = false; clearTimeout(timer); };
  }, [target]);

  async function run(install) {
    setBusy(true);
    setError("");
    setInstalled(false);
    try {
      if (install) {
        await api("install_update", { version: info.latest });
        setTarget(info.latest);
      } else {
        setInfo(await api("check_update"));
        setChecked(true);
      }
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <fieldset>
      <legend>{t("app_version")}</legend>
      {info && <p>{t("running_version")} <strong>{info.current}</strong></p>}
      {info?.failed && <p role="alert">{t("update_rolled_back").replace("{version}",info.failed)}</p>}
      {checked && info && <p role="status">{!info.latest ? t("release_none")
        : info.available ? t("release_new").replace("{version}",info.latest) : t("release_current")}</p>}
      {info?.managed && !checked && !target && !installed && <p>{t("managed_update_hint")}</p>}
      {info && !info.managed && <p>{t("manual_update_hint")}</p>}
      {checked && info?.managed && info.available && !info.installable && <p>{t("release_missing_asset")}</p>}
      <div className="row">
        <button type="button" disabled={busy || !!target || (!info && !error)} onClick={() => run(false)}>{t("check_update")}</button>
        {info?.installable && <button type="button" disabled={busy || !!target} onClick={() => run(true)}>
          {t("install_version").replace("{version}",info.latest)}
        </button>}
        <a href={info?.latest ? releases + "/tag/" + info.latest : releases} target="_blank" rel="noreferrer">{t("github_releases")}</a>
      </div>
      {(busy || target || installed) && <p role="status">{t(target ? "update_restarting" : installed ? "update_installed" : "working")}</p>}
      {error && <p role="alert">{error}</p>}
    </fieldset>
  );
}
