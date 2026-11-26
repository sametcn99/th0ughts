import React, { useEffect, useRef, useState } from "react";
import { state, t, api, req } from "./shared.jsx";

export function CommentBody({ item }) {
  if (item.status === "deleted")
    return <p className="comment-muted">{t("deleted_comment")}</p>;
  return (
    <>
      {item.format === "markdown" ? (
        <div
          className="comment-text"
          dangerouslySetInnerHTML={{ __html: item.html }}
        />
      ) : (
        <p className="comment-plain">{item.body}</p>
      )}
      {!!item.images?.length && (
        <div className="comment-images">
          {item.images.map((id, i) => (
            <a href={"/media/" + id} key={id}>
              <img
                loading="lazy"
                src={"/media/" + id + "?w=480"}
                alt={t("comment_image_n").replace("{n}", i + 1)}
              />
            </a>
          ))}
        </div>
      )}
    </>
  );
}

function GuestForm({ parent = "", cancel, submitted }) {
  const policy = state.site.comments;
  const [format, setFormat] = useState("text"),
    [files, setFiles] = useState([]),
    [address, email] = useState(""),
    [challenge, setChallenge] = useState(""),
    [code, setCode] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const fileInput = useRef(null),
    codeInput = useRef(null);
  useEffect(() => {
    if (challenge) codeInput.current?.focus();
  }, [challenge]);
  async function submit(event) {
    event.preventDefault();
    if (busy) return;
    const form = event.currentTarget;
    const data = new FormData(form);
    setBusy(true);
    setError("");
    try {
      if (!challenge) {
        const result = await req("/api/v1/comment-code", {
          email: address,
          post: state.threadPost,
          lang: state.lang,
        });
        setChallenge(result.challenge);
        return;
      }
      const payload = new FormData();
      for (const k of ["name", "body"]) payload.set(k, data.get(k));
      payload.set("parent", parent);
      payload.set("format", format);
      for (const f of files) payload.append("image", f);
      const response = await fetch(
        "/api/v1/comments/" + encodeURIComponent(state.threadPost),
        {
          method: "POST",
          headers: {
            "X-Th0ughts-Request": "1",
            "X-Comment-Challenge": challenge,
            "X-Comment-Code": code,
          },
          body: payload,
        },
      );
      const result = await response.json();
      if (!response.ok) throw Error(t(result.error));
      submitted(result);
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <form className="guest-comment" onSubmit={submit}>
      <label>
        {t("comment_body")}
        <textarea
          name="body"
          maxLength={6000}
          required={!files.length}
          rows={3}
          autoFocus={!!parent}
        />
      </label>
      {policy.markdown && (
        <label>
          {t("comment_format")}
          <select
            aria-label={t("comment_format")}
            value={format}
            onChange={(e) => setFormat(e.target.value)}
          >
            <option value="text">{t("plain_text")}</option>
            <option value="markdown">{t("markdown_format")}</option>
          </select>
        </label>
      )}
      <div className="comment-fields">
        <label>
          {t("comment_name")}
          <input
            name="name"
            autoComplete="nickname"
            maxLength={80}
            placeholder={t("unknown_user")}
          />
        </label>
        <label>
          {t("comment_email")}
          <input
            type="email"
            required
            maxLength={254}
            autoComplete="email"
            value={address}
            readOnly={!!challenge}
            onChange={(e) => email(e.target.value)}
          />
        </label>
      </div>
      {policy.images > 0 && (
        <label>
          {t("comment_images").replace("{n}", policy.images)}
          <input
            type="file"
            ref={fileInput}
            accept="image/jpeg,image/png,image/webp"
            multiple
            onChange={(e) => {
              const next = Array.from(e.target.files || []);
              if (
                next.length > policy.images ||
                next.some((f) => f.size > 4 * 1024 * 1024)
              ) {
                setError(t("image_limits"));
                setFiles([]);
                e.target.value = "";
              } else {
                setFiles(next);
                setError("");
              }
            }}
          />
        </label>
      )}
      {challenge && (
        <>
          <p role="status">{t("code_sent")}</p>
          <label>
            {t("email_code")}
            <input
              ref={codeInput}
              inputMode="numeric"
              autoComplete="one-time-code"
              pattern="[0-9]{6}"
              maxLength={6}
              required
              value={code}
              onChange={(e) => setCode(e.target.value)}
            />
          </label>
          <button
            type="button"
            onClick={() => {
              setChallenge("");
              setCode("");
              setError("");
            }}
          >
            {t("change_email")}
          </button>
        </>
      )}
      {error && <p role="alert">{error}</p>}
      <div className="comment-actions">
        <button className="comment-primary" disabled={busy}>
          {t(
            busy
              ? "working"
              : challenge
                ? "submit_comment"
                : "send_comment_code",
          )}
        </button>
        <button type="button" onClick={cancel} disabled={busy}>
          {t("cancel")}
        </button>
      </div>
    </form>
  );
}

function Branch({ parent = "", initial, depth = 0 }) {
  const [items, setItems] = useState(initial?.items || []),
    [next, setNext] = useState(initial?.next || ""),
    [busy, loading] = useState(!initial),
    [error, setError] = useState(""),
    [attempt, retry] = useState(0);
  useEffect(() => {
    if (initial) {
      setItems(initial.items);
      setNext(initial.next);
      loading(false);
      return;
    }
    let active = true;
    loading(true);
    setError("");
    api("list_comments", { post: state.threadPost, parent })
      .then((p) => {
        if (active) {
          setItems(p.items);
          setNext(p.next);
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      })
      .finally(() => {
        if (active) loading(false);
      });
    return () => {
      active = false;
    };
  }, [parent, initial, attempt]);
  async function more() {
    loading(true);
    setError("");
    try {
      const p = await api("list_comments", {
        post: state.threadPost,
        parent,
        cursor: next,
      });
      setItems((v) => [...v, ...p.items]);
      setNext(p.next);
    } catch (e) {
      setError(e.message);
    } finally {
      loading(false);
    }
  }
  return (
    <div className={parent ? "comment-branch" : "comment-roots"}>
      {items.map((item) => (
        <ThreadNode key={item.id} item={item} depth={depth} />
      ))}
      {!items.length && !parent && !busy && !error && (
        <p className="comment-muted">{t("no_comments")}</p>
      )}
      {busy && <p role="status">{t("working")}</p>}
      {error && (
        <p role="alert">
          {error}{" "}
          <button onClick={() => (next ? more() : retry((n) => n + 1))}>
            {t("retry")}
          </button>
        </p>
      )}
      {next && !error && (
        <button disabled={busy} onClick={more}>
          {t("more_comments")}
        </button>
      )}
    </div>
  );
}

function ThreadNode({ item, depth }) {
  const [open, show] = useState(false),
    [reply, compose] = useState(false),
    [removed, remove] = useState(false),
    [revision, refresh] = useState(0),
    [message, notify] = useState(""),
    [collapsed, collapse] = useState(false);
  const deleted = removed || item.status === "deleted";
  if (removed && !item.hasReplies && !revision) return null;
  return (
    <article
      className={"thread-node" + (depth < 3 ? " thread-indent" : "")}
      id={"comment-" + item.id}
    >
      <header className="comment-meta">
        <button
          className="branch-toggle"
          onClick={() => collapse(!collapsed)}
          aria-expanded={!collapsed}
          aria-label={t(collapsed ? "expand_comment" : "collapse_comment")}
        >
          {collapsed ? "+" : "−"}
        </button>
        <strong>
          {deleted ? t("deleted_comment") : item.name || t("unknown_user")}
        </strong>
        <a href={"#comment-" + item.id}>
          <time dateTime={new Date(item.created).toISOString()}>
            {new Date(item.created).toLocaleString(state.lang, {
              dateStyle: "medium",
              timeStyle: "short",
            })}
          </time>
        </a>
        {item.parent && (
          <a href={"#comment-" + item.parent}>{t("parent_comment")}</a>
        )}
      </header>
      {!collapsed && (
        <>
          {!deleted && <CommentBody item={item} />}
          <div className="comment-actions">
            {(item.hasReplies || revision > 0) && (
              <button aria-expanded={open} onClick={() => show(!open)}>
                {t(open ? "hide_replies" : "show_replies")}
              </button>
            )}
            {!deleted && !state.threadReadonly && item.depth < 12 && (
              <button aria-expanded={reply} onClick={() => compose(!reply)}>
                {t("reply")}
              </button>
            )}
            {!deleted && state.owner && (
              <button
                onClick={async () => {
                  if (!confirm(t("confirm_comment_delete"))) return;
                  try {
                    await api("moderate_comment", {
                      id: item.id,
                      action: "delete",
                    });
                    remove(true);
                    compose(false);
                  } catch (e) {
                    notify(e.message);
                  }
                }}
              >
                {t("delete")}
              </button>
            )}
          </div>
          {reply && (
            <GuestForm
              parent={item.id}
              cancel={() => compose(false)}
              submitted={(c) => {
                compose(false);
                notify(
                  t(
                    c.status === "pending"
                      ? "comment_queued"
                      : "comment_published",
                  ),
                );
                if (c.status === "approved") {
                  refresh((n) => n + 1);
                  show(true);
                }
              }}
            />
          )}
          {message && <p role="status">{message}</p>}
          {open && <Branch key={revision} parent={item.id} depth={depth + 1} />}
        </>
      )}
    </article>
  );
}
export function Thread() {
  const [compose, show] = useState(false),
    [revision, refresh] = useState(0),
    [message, notify] = useState("");
  const [trail, setTrail] = useState(null);
  useEffect(() => {
    let token = 0;
    async function locate() {
      const current = ++token;
      const id = location.hash.match(/^#comment-([a-z0-9_]+)$/)?.[1];
      if (!id) {
        setTrail(null);
        return;
      }
      try {
        const result = await api("comment_context", { id });
        if (current === token && result.at(-1)?.post === state.threadPost) {
          setTrail(result);
          document.getElementById("comments")?.scrollIntoView();
        }
      } catch (e) {
        if (current === token) {
          setTrail(null);
          notify(e.message);
        }
      }
    }
    locate();
    window.addEventListener("hashchange", locate);
    return () => {
      token++;
      window.removeEventListener("hashchange", locate);
    };
  }, []);
  return (
    <>
      <div className="thread-heading">
        <h2>{t("comments")}</h2>
        {!state.threadReadonly && (
          <button
            className="comment-primary"
            aria-expanded={compose}
            onClick={() => show(!compose)}
          >
            {t("add_comment")}
          </button>
        )}
      </div>
      {state.threadReadonly && (
        <p className="comment-muted">{t("thread_readonly")}</p>
      )}
      {compose && (
        <GuestForm
          cancel={() => show(false)}
          submitted={(c) => {
            show(false);
            notify(
              t(
                c.status === "pending" ? "comment_queued" : "comment_published",
              ),
            );
            if (c.status === "approved") refresh((n) => n + 1);
          }}
        />
      )}
      {message && <p role="status">{message}</p>}
      {trail ? (
        <>
          <nav className="comment-actions" aria-label={t("thread_context")}>
            <a href="#comments">{t("all_comments")}</a>
            {trail.slice(0, -1).map((c) => (
              <a key={c.id} href={"#comment-" + c.id}>
                {c.name || t("unknown_user")}
              </a>
            ))}
          </nav>
          <ThreadNode key={trail.at(-1).id} item={trail.at(-1)} depth={0} />
        </>
      ) : (
        <Branch key={revision} initial={revision ? null : state.thread} />
      )}
    </>
  );
}
