import React, { useEffect, useState } from "react";
import {
  state,
  t,
  api,
  req,
  Field,
  Notice,
  useAction,
  SignIn,
  ceremony,
} from "./shared.jsx";
const sections = ["site", "appearance", "security", "tokens", "storage"];
export default function Settings() {
  const [open, show] = useState(false),
    [tab, setTab] = useState("site"),
    [prefs, setPrefs] = useState(state.site),
    [themes, setThemes] = useState([]),
    [tokens, setTokens] = useState([]),
    [preview, setPreview] = useState(false),
    [draft, setDraft] = useState(null),
    [asOwner, as] = useState(false),
    [sample, setSample] = useState(false);
  const [name, setName] = useState(""),
    [days, setDays] = useState(30),
    [scopes, setScopes] = useState(["posts:read"]),
    [secret, setSecret] = useState(""),
    [otp, setOTP] = useState(""),
    [recovery, codes] = useState(""),
    [token, issued] = useState(""),
    [reauth, verify] = useState(false),
    [location, setLocation] = useState("disk"),
    [s3, config] = useState({
      endpoint: "",
      bucket: "",
      access: "",
      secret: "",
      tls: true,
    });
  const a = useAction();
  useEffect(() => {
    if (open)
      a.run(async () => {
        if (tab === "appearance") setThemes(await api("list_themes"));
        if (tab === "tokens") setTokens(await api("list_tokens"));
      });
  }, [open, tab]);
  const set = (k, v) => setPrefs((p) => ({ ...p, [k]: v }));
  const available = draft
    ? [...themes.filter((item) => item.id !== draft.theme.id), draft.theme]
    : themes;
  const active = available.find((item) => item.id === prefs.theme);
  const previewURL = new URLSearchParams({
    preview: prefs.theme,
    view: asOwner ? "owner" : "public",
    lang: state.lang,
    sample: sample ? "1" : "0",
  });
  if (draft?.theme.id === prefs.theme) previewURL.set("draft", draft.ticket);
  for (const [key, value] of Object.entries(
    prefs.options?.[prefs.theme] || {},
  )) {
    previewURL.set("option." + key, value);
  }
  const save = () =>
    a.run(async () => {
      if (draft?.theme.id === prefs.theme) {
        await api("install_theme", { source: draft.source });
      }
      await api("save_settings", prefs);
      locationReload();
    });
  const locationReload = () => window.location.reload();
  return (
    <>
      <div className="toolbar">
        <button
          className="secondary"
          aria-expanded={open}
          onClick={() => show(!open)}
        >
          {t(open ? "close" : "settings")}
        </button>
        <a href={"/?view=public&lang=" + state.lang}>{t("reader")}</a>
      </div>
      {open && (
        <section className="panel">
          <h2>{t("settings")}</h2>
          <div className="tabs">
            {sections.map((k) => (
              <button
                key={k}
                aria-pressed={tab === k}
                onClick={() => setTab(k)}
              >
                {t(k)}
              </button>
            ))}
          </div>
          {tab === "site" && (
            <>
              {["name", "description", "author", "url", "cover"].map((k) => (
                <Field
                  key={k}
                  label={k}
                  value={prefs[k]}
                  onChange={(v) => set(k, v)}
                />
              ))}
              <label>
                {t("default_lang")}
                <select
                  value={prefs.lang}
                  onChange={(e) => set("lang", e.target.value)}
                >
                  <option value="en">{t("lang_en")}</option>
                  <option value="tr">{t("lang_tr")}</option>
                </select>
              </label>
              <button onClick={save}>{t("save")}</button>
            </>
          )}
          {tab === "appearance" && (
            <>
              <label>
                {t("active_theme")}
                <select
                  value={prefs.theme}
                  onChange={(e) => set("theme", e.target.value)}
                >
                  {available.map((k) => (
                    <option key={k.id} value={k.id}>
                      {k.name}
                    </option>
                  ))}
                </select>
              </label>
              {active?.missing?.length > 0 && (
                <p role="status">
                  {t("theme_missing").replace(
                    "{items}",
                    active.missing.join(", "),
                  )}
                </p>
              )}
              {(active?.options || []).map((o) => {
                const v = prefs.options?.[prefs.theme]?.[o.key] ?? o.default;
                const change = (value) =>
                  set("options", {
                    ...prefs.options,
                    [prefs.theme]: {
                      ...prefs.options?.[prefs.theme],
                      [o.key]: value,
                    },
                  });
                const label = (labels) =>
                  labels[state.lang] || labels.en || Object.values(labels)[0];
                return (
                  <label key={o.key}>
                    {label(o.labels)}
                    {o.type === "select" ? (
                      <select
                        value={v}
                        onChange={(e) => change(e.target.value)}
                      >
                        {o.choices.map((c) => (
                          <option key={c.value} value={c.value}>
                            {label(c.labels)}
                          </option>
                        ))}
                      </select>
                    ) : o.type === "toggle" ? (
                      <input
                        type="checkbox"
                        checked={v === "1"}
                        onChange={(e) => change(e.target.checked ? "1" : "0")}
                      />
                    ) : (
                      <input
                        type={o.type === "range" ? "range" : "color"}
                        min={o.min ?? 0}
                        max={o.max}
                        value={v}
                        onChange={(e) => change(e.target.value)}
                      />
                    )}
                  </label>
                );
              })}
              <label>
                {t("upload_theme")}
                <input
                  type="file"
                  accept=".html"
                  onChange={(e) => {
                    const f = e.target.files[0];
                    if (f)
                      a.run(async () => {
                        const source = await f.text();
                        const result = await api("preview_theme", { source });
                        setDraft({ ...result, source });
                        set("theme", result.theme.id);
                      });
                  }}
                />
              </label>
              <div className="row">
                <button onClick={save}>{t("save")}</button>
                <button
                  className="secondary"
                  onClick={() =>
                    a.run(async () => {
                      if (!preview && draft?.theme.id === prefs.theme) {
                        const result = await api("preview_theme", {
                          source: draft.source,
                        });
                        setDraft({ ...result, source: draft.source });
                      }
                      setPreview(!preview);
                    })
                  }
                >
                  {t("preview")}
                </button>
                <button
                  className="secondary"
                  onClick={() =>
                    a.run(async () => {
                      await api("install_theme", { rollback: prefs.theme });
                      setDraft(null);
                      setThemes(await api("list_themes"));
                    })
                  }
                >
                  {t("rollback")}
                </button>
              </div>
              {preview && (
                <>
                  <label className="check">
                    <input
                      type="checkbox"
                      checked={asOwner}
                      onChange={(e) => as(e.target.checked)}
                    />
                    {t("owner")}
                  </label>
                  <label className="check">
                    <input
                      type="checkbox"
                      checked={sample}
                      onChange={(e) => setSample(e.target.checked)}
                    />
                    {t("sample")}
                  </label>
                  <iframe
                    title={t("preview_holder")}
                    sandbox=""
                    src={"/?" + previewURL.toString()}
                  />
                </>
              )}
            </>
          )}
          {tab === "security" && (
            <>
              <div className="row">
                <button onClick={() => a.run(() => ceremony(true))}>
                  {t("new_passkey")}
                </button>
                <button
                  onClick={() =>
                    a.run(async () => {
                      const x = await req("/auth/otp-begin");
                      setSecret(x.secret);
                    })
                  }
                >
                  {t("enable_totp")}
                </button>
              </div>
              {secret && (
                <>
                  <p>{t("otp_hint")}</p>
                  <code>{secret}</code>
                  <Field label="otp" value={otp} onChange={setOTP} />
                  <button
                    onClick={() =>
                      a.run(async () => {
                        codes(
                          (await req("/auth/otp-finish", { code: otp })).join(
                            "\n",
                          ),
                        );
                        setSecret("");
                        setOTP("");
                      })
                    }
                  >
                    {t("save")}
                  </button>
                </>
              )}
              {recovery && (
                <>
                  <p>{t("recovery")}</p>
                  <output>{recovery}</output>
                </>
              )}
            </>
          )}
          {tab === "tokens" && (
            <>
              <Field label="token_name" value={name} onChange={setName} />
              <Field
                label="expiry"
                type="number"
                value={days}
                min={1}
                max={365}
                onChange={setDays}
              />
              {["posts:read", "posts:write", "settings", "themes", "media"].map(
                (k) => (
                  <label className="check" key={k}>
                    <input
                      type="checkbox"
                      checked={scopes.includes(k)}
                      onChange={(e) =>
                        setScopes(
                          e.target.checked
                            ? [...scopes, k]
                            : scopes.filter((s) => s !== k),
                        )
                      }
                    />
                    {t(k)}
                  </label>
                ),
              )}
              <button
                onClick={() =>
                  a.run(async () => {
                    issued(
                      await api("create_token", {
                        name,
                        scopes,
                        expires: new Date(
                          Date.now() + Number(days) * 86400000,
                        ).toISOString(),
                      }),
                    );
                    setTokens(await api("list_tokens"));
                  })
                }
              >
                {t("create")}
              </button>
              {token && (
                <>
                  <p>{t("copy_token")}</p>
                  <output>{token}</output>
                </>
              )}
              {tokens.map((x) => (
                <p key={x.id}>
                  {x.grant.name}{" "}
                  <button
                    className="secondary"
                    onClick={() =>
                      a.run(async () => {
                        await api("revoke_token", { id: x.id });
                        setTokens(await api("list_tokens"));
                      })
                    }
                  >
                    {t("revoke")}
                  </button>
                </p>
              ))}
            </>
          )}
          {tab === "storage" && (
            <>
              <label>
                {t("media")}
                <select
                  value={location}
                  onChange={(e) => setLocation(e.target.value)}
                >
                  <option value="disk">{t("disk")}</option>
                  <option>S3</option>
                </select>
              </label>
              {location === "S3" &&
                ["endpoint", "bucket", "access", "secret"].map((k) => (
                  <Field
                    key={k}
                    label={k}
                    value={s3[k]}
                    type={k === "secret" ? "password" : "text"}
                    onChange={(v) => config({ ...s3, [k]: v })}
                  />
                ))}
              <button
                onClick={() =>
                  a.run(() =>
                    api("configure_media", location === "disk" ? {} : s3),
                  )
                }
              >
                {t("save")}
              </button>
              <p>
                <a href="/api/v1/backup">{t("backup")}</a>
              </p>
              <label>
                {t("restore")}
                <input
                  type="file"
                  accept=".zip"
                  onChange={(e) => {
                    const f = e.target.files[0];
                    if (f && confirm(t("confirm_restore")))
                      a.run(async () => {
                        await req("/api/v1/restore", f, true);
                        locationReload();
                      });
                  }}
                />
              </label>
            </>
          )}
          <Notice action={a} />
          <div className="row">
            <button className="secondary" onClick={() => verify(!reauth)}>
              {t("reauth")}
            </button>
            <button
              className="secondary"
              onClick={() =>
                a.run(async () => {
                  await req("/auth/logout");
                  sessionStorage.removeItem("editing");
                  window.location.assign("/");
                })
              }
            >
              {t("logout")}
            </button>
          </div>
          {reauth && <SignIn reauth onDone={() => verify(false)} />}
        </section>
      )}
    </>
  );
}
