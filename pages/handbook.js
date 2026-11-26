// Detailed guides stay readable on the static site with JavaScript disabled.
export const handbook = {
  "README.md#publishing": {
    en: [
      "Sign in at /login and write directly in the timeline. Use Post options to add media, tags or an article title. Longer posts open in full from their timeline preview. Save draft keeps it private. Publish makes the original visible to readers.",
      "Use the other language tab to add a translation and publish it separately. Share the post's date link for a permanent URL. Readers see the original if their translation is still a draft.",
    ],
    tr: [
      "/login adresinden giriş yapıp doğrudan akışa yaz. Gönderi seçeneklerinden medya, etiket veya yazı başlığı ekleyebilirsin. Uzun yazıların tamamı akıştaki önizlemeden açılır. Taslağı kaydet gönderiyi gizli tutar. Yayımla özgün gönderiyi okurlara açar.",
      "Diğer dil sekmesinde bir çeviri ekleyip bağımsız olarak yayımla. Kalıcı adresi paylaşmak için gönderinin tarih bağlantısını kullan. Çeviri taslaksa okur özgün gönderiyi görür.",
    ],
  },
  "docs/themes.md": {
    en: [
      "Save the example below as violet.html. In Settings → Appearance, upload the file, open Preview, and switch between sample content and the real timeline. Save when you are ready to publish.",
      "Use the same theme ID to replace a version. Restore previous theme version rolls it back. To contribute a theme, add the same file to the repository’s themes directory.",
    ],
    tr: [
      "Aşağıdaki örneği violet.html olarak kaydet. Ayarlar → Görünüm bölümünde dosyayı yükle. Önizlemede örnek içerik ve gerçek akış arasında geçiş yap. Yeni görünümü yayımlamaya hazır olduğunda kaydet.",
      "Yeni sürümde aynı tema kimliğini kullan. Önceki tema sürümünü geri yükleyebilirsin. Temayı projeye katmak için aynı dosyayı reponun themes dizinine ekle.",
    ],
    code: '<meta name="theme" content="violet" title="Violet">\n<style>.entry { border-color: #7666a1; }</style>',
  },
  "docs/interfaces.md": {
    en: [
      "Read the public timeline at /feed.md or /api/v1/posts. Get the API specification from /api/v1/openapi.json. To publish remotely, create a token with posts:write and store it in a private file.",
      "An MCP client can connect to /mcp over HTTP or launch th0ughts mcp over stdio. Browser agents discover the available tools on the page when the browser supports WebMCP.",
    ],
    tr: [
      "Herkese açık akışı /feed.md veya /api/v1/posts adresinden oku. API tanımı /api/v1/openapi.json adresindedir. Uzaktan yayınlamak için posts:write yetkili bir token oluştur ve özel bir dosyada sakla.",
      "MCP istemcisi /mcp adresine HTTP ile bağlanabilir veya stdio üzerinden th0ughts mcp çalıştırabilir. WebMCP destekleyen tarayıcılardaki ajanlar kullanılabilir araçları sayfadan keşfeder.",
    ],
    code: 'curl -H "Accept: text/markdown" "http://localhost:8080/?lang=en"',
  },
  "docs/deployment.md#backup-migration-and-recovery": {
    en: [
      "After verifying your identity, download a ZIP from Settings → Storage. The archive includes private drafts, translations, attachments and custom themes. Store it somewhere private.",
      "To move between drivers, initialize an empty instance with the target database, choose its media storage, then restore the archive. To preserve credentials and sessions too, keep an infrastructure backup of the database, master key, volumes and S3 objects.",
    ],
    tr: [
      "Kimliğini doğruladıktan sonra Ayarlar → Depolama bölümünden ZIP arşivini indir. Arşiv özel taslakları, çevirileri, ekleri ve özel temaları içerir. Gizli bir yerde sakla.",
      "Veritabanını değiştirmek için yeni sürücüyle boş bir kurulum başlat, medya deposunu seç ve arşivi içe aktar. Kimlik bilgileri ile oturumları da korumak için veritabanının, ana anahtarın, birimlerin ve S3 nesnelerinin altyapı yedeğini al.",
    ],
  },
  "docs/security.md": {
    en: [
      "Set your public HTTPS origin in Settings → Site before registering a passkey. Settings → Security also lets you enable authenticator codes. Store the one-time recovery codes offline.",
      "In Access tokens, grant only the scopes the integration needs, set an expiry and copy the issued value into a private file. Revoke it from the same section when access is no longer needed.",
    ],
    tr: [
      "Geçiş anahtarı kaydetmeden önce Ayarlar → Site bölümünde herkese açık HTTPS adresini tanımla. Ayarlar → Güvenlik bölümünden doğrulayıcı kodlarını da açabilirsin. Tek kullanımlık kurtarma kodlarını çevrimdışı sakla.",
      "Erişim tokenları bölümünde entegrasyonun ihtiyaç duyduğu yetkileri ve son kullanım tarihini seç. Üretilen değeri özel bir dosyada sakla. Gerek kalmadığında tokenı aynı bölümden iptal et.",
    ],
  },
  "docs/performance.md": {
    en: [
      "The reference workload seeds 10,000 posts and 200 images. Each deployment gets two CPU cores and 2 GiB of memory. The client warms up for two minutes and measures 15 minutes at 50 reads per second, with a write every five seconds.",
      "SQLite and PostgreSQL each completed 45,000 measured reads and 180 writes. Timeline p95 was 0.95 ms for SQLite and 1.18 ms for PostgreSQL on loopback. The linked report includes cold search, cache hits and memory samples.",
    ],
    tr: [
      "Referans yük testi 10.000 gönderi ve 200 görselle başlar. Her kuruluma iki çekirdek ve 2 GiB bellek ayrılır. İstemci iki dakika ısınır. Ardından 15 dakika boyunca saniyede 50 okuma, beş saniyede bir yazma gerçekleştirir.",
      "SQLite ve PostgreSQL ayrı ayrı 45.000 okuma ve 180 yazmayı tamamladı. Yerel bağlantıda akışın p95 yanıt süresi SQLite için 0,95 ms, PostgreSQL için 1,18 ms oldu. Ayrıntılı raporda soğuk arama, önbellek ve bellek örnekleri yer alır.",
    ],
  },
};
