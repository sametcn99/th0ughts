import React, { useEffect, useRef, useState } from "react";
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
import CommentSettings from "./comment-settings.jsx";
const sections = [
  "site",
  "appearance",
  "comments",
  "security",
  "tokens",
  "storage",
];
const iconPaths = {
  comments: "M3 4h18v13H9l-5 4v-4H3z",
  site: "M3 5h18v14H3z M3 9h18 M7 5v4",
  appearance:
    "M12 3a9 9 0 1 0 0 18h1a2 2 0 0 0 1-3.7c-1.1-.8-.6-2.8.9-2.8H17a4 4 0 0 0 4-4C21 6.4 17 3 12 3z M7 10h.01 M10 7h.01 M15 7h.01 M18 10h.01",
  security: "M12 3l8 3v6c0 4-4 7-8 9-4-2-8-5-8-9V6z M8.5 11.5l2.5 2.5 4.5-5",
  tokens: "M14 4a5 5 0 0 0-6 7L3 16v5h5v-3h3v-3l2-2a5 5 0 1 0 1-9z M15 7h.01",
  storage: "M3 7l2-4h14l2 4v12H3z M3 13h18 M6 16h.01 M10 16h.01",
  settings: "M3 6h5m4 0h9M3 12h10m4 0h4M3 18h3m4 0h11 M8 4v4m5 2v4m-7 2v4",
  close: "M6 6l12 12M18 6L6 18",
};
function Icon({ name }) {
  return (
    <svg
      aria-hidden="true"
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.7"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d={iconPaths[name]} />
    </svg>
  );
}

function FilePicker({ label, accept, onSelect }) {
  const [filename, setFilename] = useState("");
  return (
    <label>
      {t(label)}
      <span className="file-picker">
        <span className="file-button" aria-hidden="true">
          {t("select_file")}
        </span>
        <span className="file-name" aria-hidden="true">
          {filename || accept}
        </span>
        <input
          type="file"
          aria-label={t(label)}
          accept={accept}
          onChange={(event) => {
            const file = event.target.files[0];
            if (!file) return;
            setFilename(file.name);
            onSelect(file);
          }}
        />
      </span>
    </label>
  );
}

export default function Settings({ previewSection } = {}) {
  const [savedSection] = useState(
    () => previewSection ?? history.state?.savedSettingsSection,
  );
  const [open, show] = useState(sections.includes(savedSection)),
    [tab, setTab] = useState(
      sections.includes(savedSection) ? savedSection : "site",
    ),
    [prefs, setPrefs] = useState(state.site),
    [themes, setThemes] = useState([]),
    [tokens, setTokens] = useState([]),
    [preview, setPreview] = useState(false),
    [draft, setDraft] = useState(null),
    [asOwner, as] = useState(false),
    [sample, setSample] = useState(false),
    [screen, setScreen] = useState("feed"),
    [versions, setVersions] = useState(null),
    [revision, setRevision] = useState(0);
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
  const launcher = useRef(null);
  const dialog = useRef(null);
  useEffect(() => {
    if (savedSection) {
      const next = { ...history.state };
      delete next.savedSettingsSection;
      history.replaceState(next, "");
      a.set(t("saved"));
    }
  }, [savedSection]);
  const content = useRef(null);
  const [loadError, setLoadError] = useState("");
  const [loadAttempt, retry] = useState(0);
  const [loading, setLoading] = useState(false);
  useEffect(() => {
    if (!open) return;
    dialog.current.showModal();
    document.documentElement.classList.add("settings-open");
    return () => document.documentElement.classList.remove("settings-open");
  }, [open]);
  useEffect(() => {
    let current = true;
    setLoadError("");
    const command =
      tab === "appearance"
        ? "list_themes"
        : tab === "tokens"
          ? "list_tokens"
          : null;
    setLoading(Boolean(open && command));
    if (open && command) {
      api(command)
        .then((items) => {
          if (current)
            (tab === "appearance" ? setThemes : setTokens)(items || []);
        })
        .catch((error) => {
          if (current) setLoadError(error.message);
        })
        .finally(() => {
          if (current) setLoading(false);
        });
    }
    return () => {
      current = false;
    };
  }, [open, tab, loadAttempt]);
  function selectSection(next) {
    setTab(next);
    a.set("");
    verify(false);
    if (content.current) content.current.scrollTop = 0;
  }
  function close() {
    dialog.current.close();
  }
  const editable = ["site", "appearance", "comments"].includes(tab);
  const dirty =
    JSON.stringify(prefs) !== JSON.stringify(state.site) ||
    draft?.theme.id === prefs.theme;
  const set = (k, v) => {
    a.set("");
    setPrefs((p) => ({ ...p, [k]: v }));
  };
  const available = draft
    ? [...themes.filter((item) => item.id !== draft.theme.id), draft.theme]
    : themes;
  const active = available.find((item) => item.id === prefs.theme);
  const previewURL = new URLSearchParams({
    preview: prefs.theme,
    screen,
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
        await api("publish_theme", { settings: prefs, source: draft.source });
      } else {
        const options = { ...prefs.options };
        if (draft) {
          const id = draft.theme.id;
          if (state.site.options?.[id]) options[id] = state.site.options[id];
          else delete options[id];
        }
        await api("save_settings", { ...prefs, options });
      }
      history.replaceState({ ...history.state, savedSettingsSection: tab }, "");
      locationReload();
    });
  const locationReload = () => window.location.reload();
  return (
    <>
      <link rel="stylesheet" href="/assets/settings.css" />
      <div className="toolbar settings-launch">
        <button
          className="secondary"
          ref={launcher}
          aria-haspopup="dialog"
          aria-expanded={open}
          onClick={() => show(true)}
        >
          <Icon name="settings" />
          {t("settings")}
        </button>
        <a href={"/?view=public&lang=" + state.lang}>
          {t("reader")} <span aria-hidden="true">↗</span>
        </a>
      </div>
      {open && (
        <dialog
          className="panel settings-dialog"
          ref={dialog}
          open={previewSection ? true : undefined}
          aria-labelledby="settings-heading"
          onClose={() => {
            show(false);
            launcher.current?.focus();
            verify(false);
            issued("");
            codes("");
            setSecret("");
            setOTP("");
            a.set("");
          }}
        >
          <header className="settings-heading">
            <h2 id="settings-heading">{t("settings")}</h2>
            <button
              className="close-settings"
              aria-label={t("close")}
              onClick={close}
            >
              <Icon name="close" />
            </button>
          </header>
          <div className="settings-layout">
            <nav className="settings-nav" aria-label={t("settings_section")}>
              {sections.map((k) => (
                <button
                  key={k}
                  aria-pressed={tab === k}
                  disabled={a.busy}
                  onClick={() => selectSection(k)}
                >
                  <Icon name={k} />
                  {t(k)}
                </button>
              ))}
            </nav>
            <label className="section-select">
              <Icon name={tab} />
              <span className="visually-hidden">{t("settings_section")}</span>
              <select
                aria-label={t("settings_section")}
                value={tab}
                disabled={a.busy}
                onChange={(e) => selectSection(e.target.value)}
              >
                {sections.map((k) => (
                  <option value={k} key={k}>
                    {t(k)}
                  </option>
                ))}
              </select>
            </label>
            <section
              className="settings-content"
              aria-labelledby="section-heading"
              ref={content}
            >
              <h3 id="section-heading">{t(tab)}</h3>
              {loading && <p role="status">{t("working")}</p>}
              {loadError && (
                <p role="alert">
                  {loadError}{" "}
                  <button
                    className="secondary"
                    onClick={() => retry((n) => n + 1)}
                  >
                    {t("retry")}
                  </button>
                </p>
              )}
              {tab === "site" && (
                <>
                  <fieldset>
                    <legend>{t("public_profile")}</legend>
                    <div className="field-pair">
                      {["name", "author"].map((k) => (
                        <Field
                          key={k}
                          label={k}
                          value={prefs[k]}
                          onChange={(v) => set(k, v)}
                        />
                      ))}
                    </div>
                    <label>
                      {t("description")}
                      <textarea
                        rows={2}
                        value={prefs.description || ""}
                        onChange={(e) => set("description", e.target.value)}
                      />
                    </label>
                  </fieldset>
                  <fieldset>
                    <legend>{t("site_details")}</legend>
                    <Field
                      label="url"
                      value={prefs.url}
                      onChange={(v) => set("url", v)}
                    />
                    <Field
                      label="cover"
                      value={prefs.cover}
                      onChange={(v) => set("cover", v)}
                    />
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
                  </fieldset>
                </>
              )}
              {tab === "appearance" && (
                <>
                  <label>
                    {t("active_theme")}
                    <select
                      value={prefs.theme}
                      disabled={a.busy}
                      onChange={(e) => {
                        setVersions(null);
                        set("theme", e.target.value);
                      }}
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
                  {active?.options?.length > 0 && (
                    <fieldset>
                      <legend>{t("theme_options")}</legend>
                      <div className="option-grid">
                        {active.options.map((o) => {
                          const v =
                            prefs.options?.[prefs.theme]?.[o.key] ?? o.default;
                          const change = (value) =>
                            set("options", {
                              ...prefs.options,
                              [prefs.theme]: {
                                ...prefs.options?.[prefs.theme],
                                [o.key]: value,
                              },
                            });
                          const label = (labels) =>
                            labels[state.lang] ||
                            labels.en ||
                            Object.values(labels)[0];
                          return (
                            <label key={o.key}>
                              <span>
                                {label(o.labels)}
                                {["color", "range"].includes(
                                  o.type || "color",
                                ) && (
                                  <small aria-hidden="true">
                                    {v}
                                    {o.type === "range" ? o.unit : ""}
                                  </small>
                                )}
                              </span>
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
                                  onChange={(e) =>
                                    change(e.target.checked ? "1" : "0")
                                  }
                                />
                              ) : (
                                <input
                                  type={o.type === "range" ? "range" : "color"}
                                  min={o.min ?? 0}
                                  max={o.max}
                                  step={o.step ?? "any"}
                                  value={v}
                                  onChange={(e) => change(e.target.value)}
                                />
                              )}
                            </label>
                          );
                        })}
                      </div>
                    </fieldset>
                  )}
                  <button
                    className="secondary"
                    aria-expanded={preview}
                    onClick={() =>
                      a.run(async () => {
                        if (!preview && draft?.theme.id === prefs.theme) {
                          const result = await api("preview_theme", {
                            source: draft.source,
                            options: prefs.options,
                          });
                          setDraft({ ...result, source: draft.source });
                        }
                        setPreview(!preview);
                      }, "")
                    }
                  >
                    {t("preview")}
                  </button>
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
                      <label>
                        {t("preview_screen")}
                        <select
                          value={screen}
                          onChange={(e) => setScreen(e.target.value)}
                        >
                          {[
                            "feed",
                            "short",
                            "article-list",
                            "article",
                            "login",
                            "composer",
                            "settings",
                          ].map((k) => (
                            <option key={k} value={k}>
                              {t("preview_" + k)}
                            </option>
                          ))}
                        </select>
                      </label>
                      <iframe
                        title={t("preview_holder")}
                        sandbox=""
                        src={
                          (screen === "login" ? "/login?" : "/?") +
                          previewURL.toString()
                        }
                      />
                    </>
                  )}
                  <fieldset>
                    <legend>{t("theme_files")}</legend>
                    <FilePicker
                      label="upload_theme"
                      accept=".html"
                      onSelect={(f) => {
                        a.run(async () => {
                          const source = await f.text();
                          const result = await api("preview_theme", {
                            source,
                            options: prefs.options,
                          });
                          setDraft({ ...result, source });
                          setVersions(null);
                          setPrefs((p) => {
                            const options = result.options || {};
                            return {
                              ...p,
                              theme: result.theme.id,
                              options: {
                                ...p.options,
                                [result.theme.id]: options,
                              },
                            };
                          });
                        }, t("theme_ready"));
                      }}
                    />

                    <div className="row">
                      <button
                        className="secondary"
                        disabled={a.busy}
                        onClick={() =>
                          a.run(async () => {
                            const source =
                              draft?.theme.id === prefs.theme
                                ? draft.source
                                : (await api("get_theme", { id: prefs.theme }))
                                    .source;
                            const url = URL.createObjectURL(
                              new Blob([source], { type: "text/html" }),
                            );
                            const link = document.createElement("a");
                            link.href = url;
                            link.download = prefs.theme + ".html";
                            link.click();
                            setTimeout(() => URL.revokeObjectURL(url), 1000);
                          }, "")
                        }
                      >
                        {t("download_theme")}
                      </button>
                      <button
                        className="secondary"
                        disabled={
                          a.busy || !themes.some((x) => x.id === prefs.theme)
                        }
                        onClick={() =>
                          a.run(async () => {
                            const v = await api("get_theme", {
                              id: prefs.theme,
                            });
                            setVersions(
                              [v.previous, ...(v.history || [])].filter(
                                Boolean,
                              ),
                            );
                            setRevision(0);
                          }, "")
                        }
                      >
                        {t("theme_history")}
                      </button>
                      {themes.some((x) => x.id === prefs.theme && !x.builtin) &&
                        prefs.theme !== state.site.theme && (
                          <button
                            className="secondary"
                            onClick={() => {
                              if (confirm(t("confirm_theme_delete")))
                                a.run(async () => {
                                  await api("delete_theme", {
                                    id: prefs.theme,
                                  });
                                  setThemes(await api("list_themes"));
                                  setVersions(null);
                                  setDraft(null);
                                  setPrefs((p) => {
                                    const options = { ...p.options };
                                    delete options[p.theme];
                                    return {
                                      ...p,
                                      theme: state.site.theme,
                                      options,
                                    };
                                  });
                                });
                            }}
                          >
                            {t("delete_theme")}
                          </button>
                        )}
                    </div>
                    {versions &&
                      (versions.length ? (
                        <label>
                          {t("theme_revision")}
                          <select
                            value={revision}
                            onChange={(e) =>
                              setRevision(Number(e.target.value))
                            }
                          >
                            {versions.map((source, i) => (
                              <option key={i} value={i}>
                                {t("previous_version").replace("{n}", i + 1)}
                              </option>
                            ))}
                          </select>
                        </label>
                      ) : (
                        <p>{t("no_revisions")}</p>
                      ))}
                    <div className="row">
                      <button
                        className="secondary"
                        disabled={
                          a.busy ||
                          !themes.find((x) => x.id === prefs.theme)?.revisions
                        }
                        onClick={() =>
                          a.run(async () => {
                            await api(
                              "install_theme",
                              versions?.length
                                ? { source: versions[revision] }
                                : { rollback: prefs.theme },
                            );
                            setVersions(null);
                            setDraft(null);
                            setThemes(await api("list_themes"));
                            if (dirty) {
                              const saved = await api("get_settings");
                              setPrefs((p) => ({
                                ...p,
                                options: {
                                  ...p.options,
                                  [prefs.theme]:
                                    saved.options?.[prefs.theme] || {},
                                },
                              }));
                              return;
                            }
                            history.replaceState(
                              { ...history.state, savedSettingsSection: tab },
                              "",
                            );
                            locationReload();
                          })
                        }
                      >
                        {t(versions?.length ? "restore_version" : "rollback")}
                      </button>
                    </div>
                  </fieldset>
                </>
              )}
              {tab === "comments" && (
                <CommentSettings
                  policy={prefs.comments || {}}
                  change={(v) => set("comments", v)}
                />
              )}
              {tab === "security" && (
                <>
                  <div className="method-row">
                    <div>
                      <h4>{t("passkey_label")}</h4>
                      <p>{t("passkey_help")}</p>
                    </div>
                    <button onClick={() => a.run(() => ceremony(true))}>
                      {t("new_passkey")}
                    </button>
                  </div>
                  <div className="method-row">
                    <div>
                      <h4>{t("auth_app")}</h4>
                      <p>{t("auth_help")}</p>
                    </div>
                    <button
                      className="secondary"
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
                              (
                                await req("/auth/otp-finish", { code: otp })
                              ).join("\n"),
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
                  <div className="field-pair">
                    <Field label="token_name" value={name} onChange={setName} />
                    <Field
                      label="expiry"
                      type="number"
                      value={days}
                      min={1}
                      max={365}
                      onChange={setDays}
                    />
                  </div>
                  <fieldset>
                    <legend>{t("permissions")}</legend>
                    <div className="scope-grid">
                      {[
                        "posts:read",
                        "posts:write",
                        "settings",
                        "themes",
                        "media",
                      ].map((k) => (
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
                      ))}
                    </div>
                  </fieldset>
                  <button
                    disabled={
                      loading ||
                      a.busy ||
                      !name.trim() ||
                      !scopes.length ||
                      !Number.isInteger(Number(days)) ||
                      days < 1 ||
                      days > 365
                    }
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
                  {tokens.length > 0 && (
                    <h4 className="group-heading">{t("issued_tokens")}</h4>
                  )}
                  {tokens.map((x) => (
                    <p className="grant-row" key={x.id}>
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
                  <fieldset className="backup-group">
                    <legend>{t("backups")}</legend>
                    <p>
                      <a href="/api/v1/backup">{t("backup")}</a>
                    </p>
                    <FilePicker
                      label="restore"
                      accept=".zip"
                      onSelect={(f) => {
                        if (confirm(t("confirm_restore")))
                          a.run(async () => {
                            await req("/api/v1/restore", f, true);
                            locationReload();
                          });
                      }}
                    />
                  </fieldset>
                </>
              )}
              {tab === "security" && (
                <fieldset>
                  <legend>{t("session")}</legend>
                  <div className="row">
                    <button
                      className="secondary"
                      onClick={() => verify(!reauth)}
                    >
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
                </fieldset>
              )}
              {["tokens", "storage", "comments"].includes(tab) && (
                <button className="verify-link" onClick={() => verify(!reauth)}>
                  {t("reauth")}
                </button>
              )}
              {tab !== "security" && reauth && (
                <SignIn reauth onDone={() => verify(false)} />
              )}
            </section>
          </div>
          <footer className="settings-footer">
            <Notice action={a} />
            {editable && dirty && !a.message && !a.busy && (
              <span className="dirty-state">{t("unsaved")}</span>
            )}
            {editable && (
              <button disabled={a.busy || loading || !dirty} onClick={save}>
                {t("save")}
              </button>
            )}
          </footer>
        </dialog>
      )}
    </>
  );
}
