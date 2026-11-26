import React, { useEffect, useState } from "react";
import { api, Field, Notice, state, t, useAction } from "./shared.jsx";

export default function CommentWebhook() {
  const [open, setOpen] = useState(false);
  return (
    <details className="webhook-settings" onToggle={(e) => setOpen(e.currentTarget.open)}>
      <summary>{t("signed_comment_webhook")}</summary>
      {open && <WebhookForm />}
    </details>
  );
}

function WebhookForm() {
  const [saved, setSaved] = useState(null);
  const [enabled, setEnabled] = useState(false);
  const [url, setURL] = useState("");
  const [secret, setSecret] = useState("");
  const [error, setError] = useState("");
  const [revision, reload] = useState(0);
  const action = useAction();
  useEffect(() => {
    let active = true;
    api("get_comment_webhook").then((info) => {
      if (!active) return;
      setSaved(info);
      setEnabled(info.enabled);
      setURL(info.url);
      setSecret("");
      setError("");
    }).catch((err) => { if (active) setError(err.message); });
    return () => { active = false; };
  }, [revision]);
  if (error) return <p role="alert">{error} <button onClick={() => reload(n => n + 1)}>{t("retry")}</button></p>;
  if (!saved) return <p role="status">{t("working")}</p>;
  const last = saved.last;
  return (
    <form onSubmit={(e) => {
      e.preventDefault();
      action.run(async () => {
        await api("configure_comment_webhook", {enabled, url, secret});
        setSecret("");
        reload(n => n + 1);
      });
    }}>
      <label className="check">
        <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
        {t("enable_comment_webhook")}
      </label>
      <p>{t("webhook_event_hint")}</p>
      <Field type="url" label="webhook_endpoint" value={url} onChange={setURL} required={enabled} maxLength={2048} />
      <Field type="password" label="webhook_secret" value={secret} onChange={setSecret}
        required={!!url && (!saved.hasSecret || url !== saved.url)} minLength={32} maxLength={512} autoComplete="new-password" />
      <p>{t("webhook_secret_hint")}</p>
      <div className="row"><button disabled={action.busy}>{t("save_webhook")}</button></div>
      <Notice action={action} />
      <p>{last ? t("webhook_last").replace("{state}", t("webhook_" + last.state))
        .replace("{time}", new Date(last.at).toLocaleString(state.lang))
        .replace("{attempt}", String(last.attempt))
        .replace("{code}", last.code || t("webhook_no_response")) : t("webhook_no_delivery")}</p>
      <button className="secondary" type="button" disabled={action.busy}
        onClick={() => action.run(async () => {
          const info = await api("get_comment_webhook");
          setSaved(old => ({...old, last:info.last}));
        }, "")}>{t("webhook_refresh")}</button>
    </form>
  );
}
