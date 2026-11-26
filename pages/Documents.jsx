import React from "react";
import { Layout } from "./Layout.jsx";
import { text } from "./content.js";

export function Documents({ lang, prefix, route, documents, article, html }) {
  const t = text[lang];
  const index = prefix + (lang === "tr" ? "tr/" : "") + "docs/";
  return (
    <Layout {...{ lang, prefix, route }}>
      {article ? (
        <>
          <a href={index}>← {t.documents}</a>
          {t.contentLanguage && (
            <p className="document-language">{t.contentLanguage}</p>
          )}
          {article.sections.length > 0 && (
            <details className="document-contents">
              <summary>{t.onPage}</summary>
              <ul lang="en">
                {article.sections.map(({ id, title }) => (
                  <li key={id}>
                    <a href={"#" + id}>{title}</a>
                  </li>
                ))}
              </ul>
            </details>
          )}
          {!article.hasTitle && <h1 lang="en">{article.title}</h1>}
          <article
            className="markdown"
            lang="en"
            dangerouslySetInnerHTML={{ __html: html }}
          />
        </>
      ) : (
        <>
          <h1>{t.documents}</h1>
          {t.contentLanguage && <p>{t.contentLanguage}</p>}
          <ul className="document-list" lang="en">
            {documents.map((doc) => (
              <li key={doc.route}>
                <a href={index + doc.route}>{doc.title}</a>
              </li>
            ))}
          </ul>
        </>
      )}
    </Layout>
  );
}
