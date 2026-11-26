import React, { useEffect, useState } from "react";
import {
  state,
  t,
  api,
  req,
  mount,
  Field,
  Notice,
  useAction,
  SignIn,
  ceremony,
} from "./shared.jsx";
import { createRoot } from "react-dom/client";
import { Thread } from "./comments.jsx";
import Settings from "./settings.jsx";

function Setup() {
  const [password, p] = useState(""),
    [driver, d] = useState(state.driver),
    [address, setAddress] = useState("");
  const a = useAction();
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        a.run(async () => {
          await req("/auth/setup", { password, driver, address });
          location.reload();
        });
      }}
    >
      <h1>{t("setup")}</h1>
      <p>{t("setup_hint")}</p>
      <Field
        label="password"
        value={password}
        type="password"
        minLength={12}
        required
        onChange={p}
        autoComplete="new-password"
      />
      <label>
        {t("driver")}
        <select value={driver} onChange={(e) => d(e.target.value)}>
          {[
            "sqlite",
            "postgres",
            "mysql",
            "mariadb",
            "mongodb",
            "json",
            "json-dir",
          ].map((k) => (
            <option key={k}>{k}</option>
          ))}
        </select>
      </label>
      {!["sqlite", "json", "json-dir"].includes(driver) && (
        <Field
          label="address"
          value={address}
          type="password"
          onChange={setAddress}
        />
      )}
      <button disabled={a.busy}>{t("setup")}</button>
      <Notice action={a} />
    </form>
  );
}
const translation = () => ({
  body: "",
  title: "",
  description: "",
  published: false,
  alt: {},
});
function blank() {
  return {
    id: "",
    original: state.lang,
    text: { [state.lang]: translation() },
    tags: [],
    media: [],
    pinned: false,
    revision: 0,
  };
}
function savedDraft() {
  try {
    return JSON.parse(sessionStorage.getItem("editing"));
  } catch {
    return null;
  }
}
export function Composer() {
  const [post, set] = useState(() => savedDraft() || blank()),
    [lang, langSet] = useState(post?.original || state.lang);
  const a = useAction();
  const hasDraft = Boolean(
    post.id ||
    post.pinned ||
    post.tags?.length ||
    Object.values(post.text).some(
      (tr) => tr.body || tr.title || tr.description || tr.media?.length,
    ),
  );
  useEffect(() => {
    if (hasDraft) sessionStorage.setItem("editing", JSON.stringify(post));
    else sessionStorage.removeItem("editing");
  }, [post, hasDraft]);
  useEffect(() => {
    const click = (e) => {
      const id = e.target.closest?.("[data-edit]")?.dataset.edit;
      if (id)
        a.run(async () => {
          const p = await api("get_post", { id, private: true });
          set(p);
          langSet(p.original);
          document.querySelector("#composer").scrollIntoView();
        });
    };
    document.addEventListener("click", click);
    return () => document.removeEventListener("click", click);
  }, []);
  const edit = (key, val) => set((p) => ({ ...p, [key]: val }));
  const tr = post?.text[lang] || translation();
  const shownMedia = [...(post?.media || []), ...(tr.media || [])];
  const setTr = (key, val) =>
    set((p) => ({
      ...p,
      text: {
        ...p.text,
        [lang]: { ...(p.text[lang] || translation()), [key]: val },
      },
    }));
  function save(published) {
    a.run(async () => {
      const body = {
        ...post,
        tags: (post.tags || []).filter(Boolean),
        text: {
          ...post.text,
          [lang]: {
            ...tr,
            cover: shownMedia.includes(tr.cover) ? tr.cover : "",
            published,
          },
        },
      };
      await api("save_post", body);
      sessionStorage.removeItem("editing");
      location.reload();
    });
  }
  const hasContent = Boolean(tr.body?.trim() || shownMedia.length);
  return (
    <section
      className="panel compose-panel"
      aria-label={t(post.id ? "edit" : "write")}
    >
      {post.id && <h2>{t("edit")}</h2>}
      <label className="compose-body">
        <span className="visually-hidden">{t("body")}</span>
        <textarea
          placeholder={t("compose_hint")}
          rows={2}
          value={tr.body}
          onChange={(e) => setTr("body", e.target.value)}
        />
      </label>
      <details className="compose-options" key={post.id || "new"}>
        <summary>{t("post_options")}</summary>
        <div className="tabs">
          {["en", "tr"].map((l) => (
            <button
              key={l}
              lang={l}
              aria-pressed={l === lang}
              onClick={() => langSet(l)}
            >
              {t("lang_" + l)}
            </button>
          ))}
        </div>
        <label>
          {t("original")}
          <select
            value={post.original}
            onChange={(e) => edit("original", e.target.value)}
          >
            <option value="en">{t("lang_en")}</option>
            <option value="tr">{t("lang_tr")}</option>
          </select>
        </label>
        <Field
          label="title"
          value={tr.title}
          onChange={(v) => setTr("title", v)}
        />
        <Field
          label="description"
          value={tr.description}
          onChange={(v) => setTr("description", v)}
        />
        <Field
          label="tags"
          value={(post.tags || []).join(",")}
          onChange={(v) =>
            edit(
              "tags",
              v.split(",").map((s) => s.trim()),
            )
          }
        />
        <label>
          {t("media")}
          <input
            type="file"
            multiple
            accept="image/png,image/jpeg,image/webp,image/gif,video/mp4,video/webm"
            onChange={(e) => {
              const files = [...e.target.files];
              a.run(async () => {
                for (const file of files) {
                  const m = await req("/api/v1/upload", file, true);
                  set((p) => ({
                    ...p,
                    text: {
                      ...p.text,
                      [lang]: {
                        ...(p.text[lang] || translation()),
                        media: [...(p.text[lang]?.media || []), m.id],
                      },
                    },
                  }));
                }
              });
            }}
          />
        </label>
        {shownMedia.length > 0 && (
          <p>
            {t(
              "media_count_" +
                new Intl.PluralRules(state.lang).select(shownMedia.length),
            ).replace(
              "{count}",
              new Intl.NumberFormat(state.lang).format(shownMedia.length),
            )}
          </p>
        )}
        {shownMedia.length > 0 && (
          <label>
            {t("post_cover")}
            <select
              value={tr.cover || ""}
              onChange={(e) => setTr("cover", e.target.value)}
            >
              <option value="">{t("automatic")}</option>
              {shownMedia.map((id, index) => (
                <option key={id} value={id}>
                  {tr.alt?.[id] || t("media") + " " + (index + 1)}
                </option>
              ))}
            </select>
          </label>
        )}
        {shownMedia.map((id) => (
          <div key={id}>
            <Field
              label="alt"
              value={tr.alt?.[id]}
              onChange={(v) => setTr("alt", { ...tr.alt, [id]: v })}
            />
            <button
              className="secondary"
              onClick={() =>
                post.media?.includes(id)
                  ? edit(
                      "media",
                      post.media.filter((m) => m !== id),
                    )
                  : setTr(
                      "media",
                      tr.media.filter((m) => m !== id),
                    )
              }
            >
              {t("remove")}
            </button>
          </div>
        ))}
        {["review", "open"].includes(state.site.comments?.mode) && (
          <label className="check">
            <input
              type="checkbox"
              checked={!!post.commentsClosed}
              onChange={(e) => edit("commentsClosed", e.target.checked)}
            />
            {t("lock_thread")}
          </label>
        )}
        <label className="check">
          <input
            type="checkbox"
            checked={post.pinned}
            onChange={(e) => edit("pinned", e.target.checked)}
          />
          {t("pin")}
        </label>
      </details>
      <div className="row">
        <button disabled={a.busy || !hasContent} onClick={() => save(true)}>
          {t("publish")}
        </button>
        <button
          className="secondary"
          disabled={a.busy || !hasContent}
          onClick={() => save(false)}
        >
          {t(tr.published ? "unpublish" : "save_draft")}
        </button>
        {hasDraft && (
          <button
            className="secondary"
            disabled={a.busy}
            onClick={() => {
              set(blank());
              langSet(state.lang);
            }}
          >
            {t("cancel")}
          </button>
        )}
        {post.id && (
          <button
            className="secondary"
            onClick={() => {
              if (confirm(t("confirm_delete")))
                a.run(async () => {
                  await api("delete_post", { id: post.id });
                  sessionStorage.removeItem("editing");
                  location.reload();
                });
            }}
          >
            {t("delete")}
          </button>
        )}
      </div>
      <Notice action={a} />
    </section>
  );
}
if (typeof document !== "undefined") {
  const thread = document.getElementById("comments");
  if (thread && state.thread) createRoot(thread).render(<Thread />);
  if (location.pathname === "/login") mount("login", <SignIn />);
  else if (state.setup) mount("composer", <Setup />);
  else if (state.owner) {
    mount("composer", <Composer />);
    mount("settings", <Settings />);
  }
}
