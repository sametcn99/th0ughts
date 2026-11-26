import assert from "node:assert/strict";
import fs from "node:fs";
import { parse } from "@babel/parser";
const json = (p) => JSON.parse(fs.readFileSync(p, "utf8"));
const en = json("server/locales/en.json"),
  tr = json("server/locales/tr.json");
assert.deepEqual(
  Object.keys(en).sort(),
  Object.keys(tr).sort(),
  "Catalog keys differ",
);
const placeholders = (s) =>
  [...s.matchAll(/\{(\w+)\}/g)].map((x) => x[1]).sort();
for (const key of Object.keys(en)) {
  assert.ok(en[key].trim() && tr[key].trim(), key);
  assert.deepEqual(placeholders(en[key]), placeholders(tr[key]), key);
}
for (const file of fs.readdirSync("web").filter((f) => f.endsWith(".jsx"))) {
  const src = parse(fs.readFileSync("web/" + file, "utf8"), {
    sourceType: "module",
    plugins: ["jsx"],
  });
  function visit(n) {
    if (!n || typeof n !== "object") return;
    if (
      n.type === "CallExpression" &&
      n.callee.name === "t" &&
      n.arguments[0]?.type === "StringLiteral"
    )
      assert.ok(en[n.arguments[0].value], file + ": " + n.arguments[0].value);
    if (n.type === "JSXText" && /[a-zA-Z]/.test(n.value))
      assert.ok(
        ["S3"].includes(n.value.trim()),
        file + ": untranslated JSX text " + n.value,
      );
    if (
      n.type === "JSXAttribute" &&
      ["label", "aria-label", "placeholder", "title"].includes(n.name.name) &&
      n.value?.type === "StringLiteral"
    )
      assert.ok(en[n.value.value], file + ": " + n.value.value);
    for (const [key, v] of Object.entries(n)) if (key !== "loc") visit(v);
  }
  visit(src);
}
console.log(
  Object.keys(en).length + " message keys verified in English and Turkish.",
);
