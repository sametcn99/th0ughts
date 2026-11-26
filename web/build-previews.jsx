// Render the shipped controls without event handlers, scripts or credentials.
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { mkdir, writeFile } from "node:fs/promises";
import { state, SignIn } from "./shared.jsx";
import { Composer } from "./owner.jsx";
import Settings from "./settings.jsx";

await mkdir("server/preview", { recursive: true });
for (const lang of ["en", "tr"]) {
  const messages = await Bun.file(`server/locales/${lang}.json`).json();
  Object.assign(state, {
    lang,
    messages,
    site: { name: "th0ughts", author: "", theme: "paper", lang },
  });
  const controls = {
    login: <SignIn />,
    composer: <Composer />,
    settings: <Settings previewSection="site" />,
    launcher: <Settings previewSection="" />,
  };
  for (const [name, component] of Object.entries(controls)) {
    await writeFile(
      `server/preview/${lang}-${name}.html`,
      renderToStaticMarkup(component) + "\n",
    );
  }
}
