// Ship the upstream browser distribution with the Go binary, already compressed.
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { gzipSync } from "node:zlib";

const dist = new URL("../server/vendor/", import.meta.url);
const bundle = await readFile(
  new URL("mermaid.min.js", import.meta.resolve("mermaid")),
);
const license = await readFile("node_modules/mermaid/LICENSE", "utf8");
await mkdir(dist, { recursive: true });
await writeFile(
  new URL("mermaid.js.gz", dist),
  gzipSync(Buffer.concat([Buffer.from("/*!\n" + license + "\n*/\n"), bundle]), {
    level: 9,
  }),
);
