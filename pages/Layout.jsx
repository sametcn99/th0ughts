import React from "react";
import { repo, text } from "./content.js";

function Brand() {
  return (
    <span className="brand">
      th<span>0</span>ughts
    </span>
  );
}
export function Layout({ lang, prefix, route = "", children }) {
  const t = text[lang];
  const home = route ? prefix + (lang === "tr" ? "tr/" : "") : "";
  const links = [
    ["top", t.quickStart],
    ["themes", t.themeTitle],
    ["installation", t.install],
    ["gallery", t.gallery],
    ["handbook", t.docs],
  ];
  return (
    <>
      <a className="skip" href="#main">
        {t.skip}
      </a>
      <div className="manual">
        <header className="sidebar">
          <a className="home" href={home + "#top"} aria-label="th0ughts">
            <Brand />
          </a>
          <nav className="contents" aria-label={t.docs}>
            {links.map(([id, name]) => (
              <a key={id} href={home + "#" + id}>
                {name}
              </a>
            ))}
            <a
              href={home + "docs/"}
              aria-current={route === "docs/" ? "page" : undefined}
            >
              {t.documents}
            </a>
          </nav>
          <div className="resources">
            <a href={repo}>
              GitHub <span aria-hidden="true">↗</span>
            </a>
            <nav aria-label={t.language}>
              <a
                href={prefix + route}
                lang="en"
                hrefLang="en"
                aria-current={lang === "en" ? "page" : undefined}
              >
                English
              </a>
              <span aria-hidden="true"> / </span>
              <a
                href={prefix + "tr/" + route}
                lang="tr"
                hrefLang="tr"
                aria-current={lang === "tr" ? "page" : undefined}
              >
                Türkçe
              </a>
            </nav>
          </div>
        </header>
        <main id="main">
          {children}
          <footer>
            <a href={repo}>GitHub ↗</a>
            <a href={route ? "#main" : "#top"}>{t.back} ↑</a>
          </footer>
        </main>
      </div>
    </>
  );
}
