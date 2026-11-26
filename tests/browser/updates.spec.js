import { test, expect } from "@playwright/test";
import { execFileSync, spawn } from "node:child_process";
import { createHash, randomBytes } from "node:crypto";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import { once } from "node:events";
import { join } from "node:path";
import { tmpdir } from "node:os";

async function listen(server) {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return server.address().port;
}
async function close(server) {
  await new Promise(resolve => server.close(resolve));
}

test("one-click release activation, persistence and failed-start fallback", async ({ browser }) => {
  test.skip(process.platform !== "linux" || !["x64", "arm64"].includes(process.arch));
  test.setTimeout(180000);
  const dir = await mkdtemp(join(tmpdir(), "th0ughts-release-"));
  const binary = process.env.TEST_BINARY || "bin/th0ughts";
  const current = execFileSync(binary, ["version"], {encoding:"utf8"}).trim();
  const nextVersion = v => v.replace(/\d+$/, patch => String(Number(patch) + 1));
  const target = nextVersion(current), broken = nextVersion(target);
  const name = "th0ughts-linux-" + (process.arch === "arm64" ? "arm64" : "amd64");
  const master = randomBytes(32).toString("hex");
  let child, exited, context, upstream, proxy;
  try {
    const update = join(dir, "release");
    execFileSync("go", ["build", "-ldflags", "-X github.com/sametcn99/th0ughts/updates.embeddedVersion="+target, "-o", update, "./cmd/th0ughts"]);
    const content = await readFile(update);
    const digest = "sha256:" + createHash("sha256").update(content).digest("hex");
    const key = join(dir, "tls.key"), cert = join(dir, "tls.crt");
    execFileSync("openssl", ["req","-x509","-newkey","rsa:2048","-nodes","-days","1","-subj","/CN=release-fixture","-addext","subjectAltName=DNS:api.github.com,DNS:github.com","-keyout",key,"-out",cert], {stdio:"ignore"});
    let tag = target;
    upstream = https.createServer({key: await readFile(key), cert: await readFile(cert)}, (req,res) => {
      if (req.url === "/repos/sametcn99/th0ughts/releases/latest") {
        res.setHeader("content-type","application/json");
        res.end(JSON.stringify({tag_name:tag,assets:[{name,size:content.length,digest}]}));
      } else if (req.url === "/sametcn99/th0ughts/releases/download/"+tag+"/"+name) {
        res.setHeader("content-length",content.length);
        res.end(content);
      } else { res.writeHead(404).end(); }
    });
    const tlsPort = await listen(upstream);
    proxy = http.createServer();
    proxy.on("connect", (req, client, head) => {
      if (!["api.github.com:443","github.com:443"].includes(req.url)) {client.destroy();return;}
      const dest = net.connect(tlsPort,"127.0.0.1",() => {
        client.write("HTTP/1.1 200 Connection established\r\n\r\n");
        if (head.length) dest.write(head);
        client.pipe(dest).pipe(client);
      });
      client.on("error",()=>dest.destroy());
      dest.on("error",()=>client.destroy());
      client.on("close",()=>dest.destroy());
    });
    const proxyPort = await listen(proxy);
    const reservation = net.createServer();
    const port = await listen(reservation);
    await close(reservation);
    const baseURL = "http://localhost:"+port;
    const start = () => {
      child = spawn(binary, ["-managed-updates","-data",join(dir,"data"),"-listen","127.0.0.1:"+port,"serve"], {
        env:{...process.env,TH0UGHTS_MASTER_KEY:master,HTTPS_PROXY:"http://127.0.0.1:"+proxyPort,NO_PROXY:"localhost,127.0.0.1",SSL_CERT_FILE:cert},
        stdio:"ignore",
      });
      exited = once(child,"exit");
    };
    const stop = async () => { child.kill("SIGTERM"); await exited; child=null; };
    start();
    context = await browser.newContext({baseURL});
    const api = async (path,data={}) => (await context.request.post(path,{headers:{"X-Th0ughts-Request":"1"},data,failOnStatusCode:true})).json();
    async function healthy() {
      await expect.poll(async () => {
        try {return (await context.request.get("/healthz")).status()}catch{return 0}
      },{timeout:30000}).toBe(200);
    }
    await healthy();
    await api("/auth/bootstrap",{master});
    await api("/auth/setup",{password:"local-fixture-password"});
    const post=await api("/api/v1/save_post",{original:"en",text:{en:{body:"Keep this thought.",published:true}}});
    const page=await context.newPage();
    await page.goto("/");
    await page.getByRole("button",{name:"Settings",exact:true}).click();
    const dialog=page.getByRole("dialog");
    await dialog.getByRole("button",{name:"Updates",exact:true}).click();
    const install = async (version) => {
      await dialog.getByRole("button",{name:"Check for updates"}).click();
      await dialog.getByRole("button",{name:"Install "+version+" and restart"}).click();
    };
    await install(target);
    await expect(dialog.getByText("The new version is running.")).toBeVisible({timeout:30000});
    expect(await api("/api/v1/update_status")).toMatchObject({current:target,managed:true});
    await stop();start();await healthy();
    expect(await api("/api/v1/update_status")).toMatchObject({current:target,managed:true});
    tag=broken; // The digest matches, but the binary announces the wrong version.
    await install(broken);
    await expect(dialog.getByRole("alert")).toContainText("previous version is running",{timeout:30000});
    expect(await api("/api/v1/update_status")).toMatchObject({current:target,failed:broken});
    expect((await api("/api/v1/get_post",{id:post.id})).text.en.body).toBe("Keep this thought.");
  } finally {
    if(child){child.kill("SIGTERM");await exited;}
    await context?.close();
    upstream?.closeAllConnections();
    if(proxy) await close(proxy);
    if(upstream) await close(upstream);
    await rm(dir,{recursive:true,force:true});
  }
});
