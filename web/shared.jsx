import React, { useState } from "react";
import { createRoot } from "react-dom/client";
export const state =
  typeof document === "undefined"
    ? {}
    : JSON.parse(document.querySelector("#state").textContent);
export const t = (k) => state.messages[k] || k;
export async function req(url, body = {}, raw = false) {
  const res = await fetch(url, {
    method: "POST",
    headers: {
      "X-Th0ughts-Request": "1",
      ...(!raw && { "Content-Type": "application/json" }),
    },
    body: raw ? body : JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({ error: "internal_error" }));
  if (!res.ok) {
    const d = data.diagnostic;
    if (d)
      throw Error(
        t("theme_diagnostic")
          .replace("{line}", d.line)
          .replace("{kind}", t("theme_error_" + d.kind))
          .replace("{token}", () => d.token),
      );
    throw Error(t(data.error));
  }
  return data;
}
export const api = (name, data) => req("/api/v1/" + name, data);
export function mount(id, node) {
  const host = document.getElementById(id);
  if (!host) return;
  const root = host.attachShadow({ mode: "open" });
  const css = document.createElement("link");
  css.rel = "stylesheet";
  css.href = "/assets/panel.css";
  root.append(css);
  const box = document.createElement("div");
  root.append(box);
  createRoot(box).render(node);
}
export function useAction() {
  const [message, updateMessage] = useState("");
  const [failed, setFailed] = useState(false);
  function set(text) {
    setFailed(false);
    updateMessage(text);
  }
  const [busy, lock] = useState(false);
  async function run(fn, success = t("saved")) {
    if (busy) return;
    set("");
    lock(true);
    try {
      await fn();
      set(success);
    } catch (e) {
      setFailed(true);
      updateMessage(e.message);
    } finally {
      lock(false);
    }
  }
  return { run, busy, message, failed, set };
}
export function Field({ label, value, onChange, type = "text", ...props }) {
  return (
    <label>
      {t(label)}
      <input
        type={type}
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value)}
        {...props}
      />
    </label>
  );
}
export function Notice({ action }) {
  return (
    <p className="notice" role={action.failed ? "alert" : "status"}>
      {action.busy ? t("working") : action.message}
    </p>
  );
}
export async function ceremony(register) {
  const opts = await req("/auth/passkey-" + (register ? "register" : "login"));
  const pk = opts.publicKey;
  const publicKey = register
    ? PublicKeyCredential.parseCreationOptionsFromJSON(pk)
    : PublicKeyCredential.parseRequestOptionsFromJSON(pk);
  const credential = await navigator.credentials[register ? "create" : "get"]({
    publicKey,
  });
  return req(
    "/auth/" + (register ? "register" : "login") + "-finish",
    credential.toJSON(),
  );
}
export function SignIn({
  reauth = false,
  onDone = () => location.assign("/?lang=" + state.lang),
}) {
  const [password, p] = useState(""),
    [code, c] = useState("");
  const a = useAction();
  const init = state.newSite && !reauth;
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        a.run(async () => {
          await req(
            "/auth/" + (init ? "bootstrap" : "login"),
            init ? { master: password } : { password, code },
          );
          onDone();
        });
      }}
    >
      {reauth ? <h3>{t("reauth")}</h3> : <h1>{t("login")}</h1>}
      <Field
        label={init ? "master" : "password"}
        type="password"
        value={password}
        onChange={p}
        autoComplete="current-password"
        required
      />
      {!init && (
        <Field
          label="otp"
          value={code}
          onChange={c}
          autoComplete="one-time-code"
        />
      )}
      <div className="row">
        <button disabled={a.busy}>{t("login")}</button>
        {!init &&
          (typeof window === "undefined" || window.PublicKeyCredential) && (
            <button
              type="button"
              className="secondary"
              onClick={() =>
                a.run(async () => {
                  await ceremony(false);
                  onDone();
                })
              }
            >
              {t("passkey")}
            </button>
          )}
      </div>
      <Notice action={a} />
    </form>
  );
}
