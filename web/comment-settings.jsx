import React, { useEffect, useState } from "react";
import { api, t, Field, Notice, useAction, state } from "./shared.jsx";
import { CommentBody } from "./comments.jsx";

import CommentWebhook from "./comment-webhook.jsx";

function MailSettings() {
  const [relay, setRelay] = useState(null),
    [reload, refresh] = useState(0),
    [error, setError] = useState("");
  const action = useAction();
  useEffect(() => {
    let active = true;
    setError("");
    api("get_comment_mail")
      .then((m) => {
        if (active)
          setRelay({
            ...m,
            port: m.port || 587,
            tlsMode: m.tlsMode || "starttls",
            password: "",
          });
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [reload]);
  const set = (k, v) => setRelay((m) => ({ ...m, [k]: v }));
  return (
    <details className="mail-settings">
      <summary>{t("smtp_settings")}</summary>
      {error && (
        <p role="alert">
          {error}{" "}
          <button onClick={() => refresh((n) => n + 1)}>{t("retry")}</button>
        </p>
      )}
      {relay && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            action.run(async () => {
              await api("configure_comment_mail", relay);
              set("password", "");
            });
          }}
        >
          <div className="field-pair">
            <Field
              label="smtp_host"
              value={relay.host}
              onChange={(v) => set("host", v)}
              required
            />
            <Field
              label="smtp_port"
              min={1}
              max={65535}
              type="number"
              value={relay.port}
              onChange={(v) => set("port", Number(v))}
              required
            />
          </div>
          <label>
            {t("smtp_transport")}
            <select
              value={relay.tlsMode}
              onChange={(e) => set("tlsMode", e.target.value)}
            >
              <option value="starttls">{t("starttls")}</option>
              <option value="tls">{t("implicit_tls")}</option>
            </select>
          </label>
          <Field
            label="smtp_from"
            type="email"
            value={relay.from}
            onChange={(v) => set("from", v)}
            required
          />
          <div className="field-pair">
            <Field
              label="smtp_user"
              value={relay.user}
              onChange={(v) => set("user", v)}
              autoComplete="off"
            />
            <Field
              label="smtp_password"
              type="password"
              value={relay.password}
              onChange={(v) => set("password", v)}
              autoComplete="new-password"
            />
          </div>
          <button disabled={action.busy}>{t("save_mail")}</button>
          <Notice action={action} />
        </form>
      )}
    </details>
  );
}
function Moderation() {
  const [status, filter] = useState("pending"),
    [page, setPage] = useState(null),
    [attempt, retry] = useState(0),
    [error, setError] = useState(""),
    [busy, lock] = useState(false);
  useEffect(() => {
    let live = true;
    setPage(null);
    setError("");
    api("pending_comments", { status })
      .then((p) => {
        if (live) setPage(p);
      })
      .catch((e) => {
        if (live) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, [status, attempt]);
  async function moderate(id, action) {
    if (action === "delete" && !confirm(t("confirm_comment_delete"))) return;
    lock(true);
    setError("");
    try {
      await api("moderate_comment", { id, action });
      retry((n) => n + 1);
    } catch (e) {
      setError(e.message);
    } finally {
      lock(false);
    }
  }
  return (
    <fieldset className="moderation">
      <legend>{t("moderation_queue")}</legend>
      <label>
        {t("comment_filter")}
        <select
          disabled={busy}
          value={status}
          onChange={(e) => filter(e.target.value)}
        >
          <option value="pending">{t("awaiting_review")}</option>
          <option value="published">{t("published_comments")}</option>
        </select>
      </label>
      {error && (
        <p role="alert">
          {error}{" "}
          <button onClick={() => retry((n) => n + 1)}>{t("retry")}</button>
        </p>
      )}
      {!page && !error && <p role="status">{t("working")}</p>}
      {page?.items.length === 0 && <p>{t("empty_queue")}</p>}
      {page?.items.map((c) => (
        <article className="moderation-entry" key={c.id}>
          <header>
            <strong>{c.name || t("unknown_user")}</strong> ·{" "}
            <time>{new Date(c.created).toLocaleString(state.lang)}</time>
          </header>
          <a
            href={
              "/?post=" +
              encodeURIComponent(c.post) +
              "&lang=" +
              state.lang +
              (c.status === "pending" ? "" : "#comment-" + c.id)
            }
          >
            {t("open_thread")}
          </a>
          <CommentBody item={c} />
          <div className="row">
            {c.status === "pending" && (
              <button disabled={busy} onClick={() => moderate(c.id, "approve")}>
                {t("approve_comment")}
              </button>
            )}
            {c.status !== "deleted" && (
              <button
                className="secondary"
                disabled={busy}
                onClick={() => moderate(c.id, "delete")}
              >
                {t("delete")}
              </button>
            )}
          </div>
        </article>
      ))}
      {page?.next && (
        <button
          disabled={busy}
          onClick={async () => {
            lock(true);
            try {
              const p = await api("pending_comments", {
                status,
                cursor: page.next,
              });
              setPage({ items: [...page.items, ...p.items], next: p.next });
            } catch (e) {
              setError(e.message);
            } finally {
              lock(false);
            }
          }}
        >
          {t("more_comments")}
        </button>
      )}
    </fieldset>
  );
}
function PostLocks() {
  const [isOpen, open] = useState(false),
    [posts, show] = useState([]),
    [next, cursor] = useState(""),
    [query, search] = useState(""),
    [selected, select] = useState(""),
    [err, setError] = useState(""),
    [working, busy] = useState(false);
  const current = posts.find((p) => p.id === selected);
  async function load(append = false) {
    busy(true);
    setError("");
    try {
      const p = await api("list_posts", {
        search: query,
        lang: state.lang,
        limit: 20,
        cursor: append ? next : "",
      });
      const items = append ? [...posts, ...p.posts] : p.posts || [];
      show(items);
      select(
        items.some((x) => x.id === selected) ? selected : items[0]?.id || "",
      );
      cursor(p.next || "");
    } catch (e) {
      setError(e.message);
    } finally {
      busy(false);
    }
  }
  return (
    <details
      onToggle={(e) => {
        const shown = e.currentTarget.open;
        if (shown && !isOpen) load();
        open(shown);
      }}
    >
      <summary>{t("post_locks")}</summary>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          load();
        }}
      >
        <Field label="search" value={query} onChange={search} />
        <button disabled={working}>{t("search")}</button>
      </form>
      {posts.length > 0 && (
        <label>
          {t("choose_post")}
          <select value={selected} onChange={(e) => select(e.target.value)}>
            {posts.map((p) => {
              const text = p.text[state.lang] || p.text[p.original];
              return (
                <option key={p.id} value={p.id}>
                  {(text.title || text.body || p.id).slice(0, 90)}
                </option>
              );
            })}
          </select>
        </label>
      )}
      {next && (
        <button
          className="secondary"
          disabled={working}
          onClick={() => load(true)}
        >
          {t("older")}
        </button>
      )}
      {current && (
        <button
          className="secondary"
          disabled={working}
          onClick={async () => {
            busy(true);
            try {
              const closed = !current.commentsClosed;
              await api("set_thread_lock", { id: current.id, closed });
              show(
                posts.map((p) =>
                  p.id === current.id ? { ...p, commentsClosed: closed } : p,
                ),
              );
            } catch (e) {
              setError(e.message);
            } finally {
              busy(false);
            }
          }}
        >
          {t(current.commentsClosed ? "unlock_post" : "lock_thread")}
        </button>
      )}
      {err && <p role="alert">{err}</p>}
    </details>
  );
}
export default function CommentSettings({ policy, change }) {
  const update = (k, v) => change({ ...policy, [k]: v });
  return (
    <>
      <fieldset>
        <legend>{t("comment_policy")}</legend>
        <label>
          {t("comment_mode")}
          <select
            value={policy.mode || "off"}
            onChange={(e) => update("mode", e.target.value)}
          >
            <option value="off">{t("comments_off")}</option>
            <option value="review">{t("comments_review")}</option>
            <option value="open">{t("comments_open")}</option>
          </select>
        </label>
        <label className="check">
          <input
            type="checkbox"
            checked={!!policy.markdown}
            onChange={(e) => update("markdown", e.target.checked)}
          />
          {t("allow_markdown")}
        </label>
        <label>
          {t("max_images")}
          <select
            value={policy.images || 0}
            onChange={(e) => update("images", Number(e.target.value))}
          >
            {[0, 1, 2, 3, 4].map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
        </label>
        <p>{t("comment_setup_hint")}</p>
      </fieldset>
      <MailSettings />
      <CommentWebhook />
      <PostLocks />
      <Moderation />
    </>
  );
}
