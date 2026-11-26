// Local TLS-only inbox for the browser suite and the disposable screenshot instance.
import { execFileSync } from "node:child_process";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createServer } from "node:tls";
import { once } from "node:events";

export async function openCommentInbox() {
  const dir = await mkdtemp(join(tmpdir(), "th0ughts-inbox-"));
  const key = join(dir, "key.pem"),
    cert = join(dir, "cert.pem");
  execFileSync(
    "openssl",
    [
      "req",
      "-x509",
      "-newkey",
      "rsa:2048",
      "-nodes",
      "-days",
      "1",
      "-subj",
      "/CN=localhost",
      "-addext",
      "subjectAltName=IP:127.0.0.1",
      "-keyout",
      key,
      "-out",
      cert,
    ],
    { stdio: "ignore" },
  );
  const codes = new Map();
  const server = createServer(
    { key: await readFile(key), cert: await readFile(cert) },
    (socket) => {
      socket.setEncoding("utf8");
      socket.write("220 localhost test inbox\r\n");
      let buffer = "",
        data = null,
        address = "";
      socket.on("error", () => {});
      socket.on("data", (chunk) => {
        buffer += chunk;
        for (let i; (i = buffer.indexOf("\r\n")) >= 0;) {
          const line = buffer.slice(0, i);
          buffer = buffer.slice(i + 2);
          if (data !== null) {
            if (line !== ".") {
              data.push(line);
              continue;
            }
            const body = data.join("\r\n").split("\r\n\r\n")[1] || "";
            const code = Buffer.from(body, "base64")
              .toString()
              .match(/\b\d{6}\b/)?.[0];
            if (code) codes.set(address, code);
            data = null;
            socket.write("250 queued\r\n");
            continue;
          }
          if (line.startsWith("RCPT TO:"))
            address = line.match(/<([^>]+)>/)?.[1] || "";
          if (line === "DATA") {
            data = [];
            socket.write("354 send data\r\n");
          } else if (line === "QUIT") {
            socket.end("221 goodbye\r\n");
          } else socket.write("250 localhost\r\n");
        }
      });
    },
  );
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return {
    cert,
    codes,
    relay: {
      host: "127.0.0.1",
      port: server.address().port,
      tlsMode: "tls",
      from: "comments@example.com",
      user: "",
    },
    async close() {
      await new Promise((resolve) => server.close(resolve));
      await rm(dir, { recursive: true, force: true });
    },
  };
}
