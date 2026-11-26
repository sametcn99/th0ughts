import { handbook } from "./handbook.js";
import { Layout } from "./Layout.jsx";
import React, { useEffect, useRef, useState } from "react";
import { drivers, install, repo, text } from "./content.js";
import shots from "./public/screenshots/manifest.json";
const shotById = Object.fromEntries(shots.map((s) => [s.id, s]));
const keySetup =
  'umask 077\ntest -e master.key || openssl rand -hex 32 > master.key\nexport TH0UGHTS_MASTER_KEY="$(cat master.key)"';

function Code({ value, label, t }) {
  const [message, setMessage] = useState("");
  const code = useRef(null);
  useEffect(() => {
    if (message) {
      const timer = setTimeout(() => setMessage(""), 2400);
      return () => clearTimeout(timer);
    }
  }, [message]);
  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setMessage(t.copied);
    } catch {
      const range = document.createRange();
      range.selectNodeContents(code.current);
      const sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
      setMessage(t.copyFail);
    }
  }
  return (
    <div className="shell">
      <div className="shell-head">
        <span>{label}</span>
        <button type="button" onClick={copy}>
          <span aria-live="polite">{message || t.copy}</span>
        </button>
      </div>
      <pre>
        <code ref={code}>{value}</code>
      </pre>
    </div>
  );
}
function QuickStart({ prefix, t }) {
  return (
    <section id="top" className="quick-start">
      <h1>{t.quickStart}</h1>
      <p className="lede">{t.lead}</p>
      <Code value={install} label={t.terminal} t={t} />
      <p className="installer-note">
        {t.quickNote} <a href={prefix + "install.sh"}>{t.inspect}</a>
      </p>
    </section>
  );
}
function Themes({ prefix, t, lang }) {
  return (
    <section id="themes">
      <h2>{t.themeTitle}</h2>
      <p>{t.themeIntro}</p>
      <div className="specimens">
        {[
          ["timeline", "Paper"],
          ["theme-index", "Index"],
          ["theme-dusk", "Dusk"],
        ].map(([id, name]) => {
          const shot = shotById[id];
          const src = prefix + "screenshots/" + shot.file;
          return (
            <figure key={id}>
              <a href={src} aria-label={t.enlarge + " · " + name}>
                <img
                  src={src}
                  width={shot.width}
                  height={shot.height}
                  alt={shot.title[lang]}
                />
              </a>
              <figcaption>{name}</figcaption>
            </figure>
          );
        })}
      </div>
      <a
        href="#theme-guide"
        onClick={() => {
          document.getElementById("theme-guide").open = true;
        }}
      >
        {t.themeGuide}
      </a>
    </section>
  );
}
function Installer({ t }) {
  const [method, select] = useState(0);
  const [db, setDB] = useState("sqlite");
  const title = [t.wizard, t.compose, t.coolify, t.dokploy, t.source][method];
  return (
    <section id="installation" className="section">
      <h2>{t.installTitle}</h2>
      <div className="install-grid">
        <nav className="methods" aria-label={t.install}>
          {t.methods.map((name, i) => (
            <button
              key={name}
              aria-pressed={i === method}
              onClick={() => select(i)}
            >
              {name}
            </button>
          ))}
        </nav>
        <div className="guide-panel">
          <h3>{title}</h3>
          {method === 0 && (
            <>
              <p>{t.wizardIntro}</p>
              <a href="#top">{t.quickStart}</a>
              <ol>
                {t.steps.map((step) => (
                  <li key={step}>{step}</li>
                ))}
              </ol>
            </>
          )}
          {method === 1 && (
            <>
              <p>{t.composeIntro}</p>
              <label>
                {t.choose}{" "}
                <select value={db} onChange={(e) => setDB(e.target.value)}>
                  {Object.entries(drivers).map(([k, v]) => (
                    <option key={k} value={k}>
                      {k === "json-dir" ? t.jsondir : v}
                    </option>
                  ))}
                </select>
              </label>
              <Code
                t={t}
                label={drivers[db]}
                value={
                  "git clone " +
                  repo +
                  ".git\ncd th0ughts\n" +
                  keySetup +
                  "\ndocker compose -p thoughts -f deploy/compose." +
                  db +
                  ".yaml up -d --build"
                }
              />
            </>
          )}
          {(method === 2 || method === 3) && (
            <>
              <p>{method === 2 ? t.coolifyDetail : t.dokployDetail}</p>
              <Code
                t={t}
                value="openssl rand -hex 32"
                label="TH0UGHTS_MASTER_KEY"
              />
              <p>{t.platforms}</p>
            </>
          )}
          {method === 4 && (
            <>
              <p>{t.sourceDetail}</p>
              <Code
                value={
                  "git clone " +
                  repo +
                  ".git\ncd th0ughts\n" +
                  keySetup +
                  "\ngo build -o bin/th0ughts ./cmd/th0ughts\nbin/th0ughts -listen 127.0.0.1:8080 -data ./data serve"
                }
                label="Go"
                t={t}
              />
            </>
          )}
          <a className="text-link" href="docs/deployment/">
            {t.full}
          </a>
        </div>
      </div>
    </section>
  );
}
function Gallery({ lang, prefix, t }) {
  const [group, filter] = useState("all"),
    [expanded, expand] = useState(false),
    [selected, setSelected] = useState(null);
  const modal = useRef(null);
  const filtered =
    group === "all" ? shots : shots.filter((s) => s.group === group);
  const shown = expanded ? filtered : filtered.slice(0, 6);
  const current = selected === null ? null : filtered[selected];
  useEffect(() => {
    if (selected !== null && !modal.current.open) modal.current.showModal();
    if (selected === null && modal.current.open) modal.current.close();
  }, [selected]);
  const move = (n) =>
    setSelected((selected + n + filtered.length) % filtered.length);
  return (
    <section id="gallery" className="gallery">
      <div className="wrap">
        <div className="gallery-heading">
          <div>
            <h2>{t.gallery}</h2>
          </div>
          <span>
            {shots.length} {t.screen}
          </span>
        </div>
        <div className="filters" aria-label={t.gallery}>
          {Object.entries({ all: t.all, ...t.groups }).map(([key, label]) => (
            <button
              key={key}
              aria-pressed={key === group}
              onClick={() => {
                filter(key);
                expand(false);
              }}
            >
              {label}
            </button>
          ))}
        </div>
        <div className="cards">
          {shown.map((s, i) => (
            <button
              key={s.id}
              className="card"
              onClick={() => setSelected(i)}
              aria-label={t.enlarge + " · " + s.title[lang]}
            >
              <div
                className={"thumb " + (s.width < s.height ? "portrait" : "")}
              >
                <img
                  loading="lazy"
                  width={s.width}
                  height={s.height}
                  alt=""
                  src={prefix + "screenshots/" + s.file}
                />
              </div>

              <strong>{s.title[lang]}</strong>
            </button>
          ))}
        </div>
        {!expanded && filtered.length > shown.length && (
          <button className="more" onClick={() => expand(true)}>
            {lang === "tr" ? "Tümünü göster" : "Show all"} ({filtered.length})
          </button>
        )}
        <dialog
          ref={modal}
          onClose={() => setSelected(null)}
          aria-label={current?.title[lang] || t.gallery}
          onKeyDown={(e) => {
            if (e.key === "ArrowRight") move(1);
            if (e.key === "ArrowLeft") move(-1);
          }}
          onClick={(e) => {
            if (e.target === modal.current) setSelected(null);
          }}
        >
          {current && (
            <div className="lightbox">
              <div className="lightbox-head">
                <h3>{current.title[lang]}</h3>
                <button onClick={() => setSelected(null)} autoFocus>
                  {t.close}
                </button>
              </div>
              <img
                width={current.width}
                height={current.height}
                alt={current.title[lang]}
                src={prefix + "screenshots/" + current.file}
              />
              <div className="pager">
                <button onClick={() => move(-1)}>{t.previous}</button>
                <span>
                  {selected + 1} / {filtered.length}
                </span>
                <button onClick={() => move(1)}>{t.next}</button>
              </div>
            </div>
          )}
        </dialog>
      </div>
    </section>
  );
}
export function App({ lang }) {
  const t = text[lang];
  const prefix = lang === "en" ? "./" : "../";
  return (
    <Layout {...{ lang, prefix }}>
      <QuickStart {...{ prefix, t }} />
      <Themes {...{ lang, prefix, t }} />
      <Installer t={t} />
      <Gallery {...{ lang, prefix, t }} />
      <section id="handbook">
        <h2>{t.docs}</h2>
        {t.guides.map(([title, body, path]) => (
          <details
            key={path}
            className="reference"
            id={path === "docs/themes.md" ? "theme-guide" : undefined}
          >
            <summary>{title}</summary>
            <p>{body}</p>
            {handbook[path][lang].map((paragraph) => (
              <p key={paragraph}>{paragraph}</p>
            ))}
            {handbook[path].code && (
              <pre>
                <code>{handbook[path].code}</code>
              </pre>
            )}
            {path.startsWith("docs/") ? (
              <a href={path.replace(".md", "/")}>{t.fullReference}</a>
            ) : (
              <a href={repo + "/blob/main/" + path}>README · GitHub</a>
            )}
          </details>
        ))}
      </section>
    </Layout>
  );
}
