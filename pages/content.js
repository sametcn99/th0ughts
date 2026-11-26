export const repo = "https://github.com/sametcn99/th0ughts";
export const site = "https://sametcn99.github.io/th0ughts/";
export const install = "curl -fsSL " + site + "install.sh | bash";
export const drivers = {
  sqlite: "SQLite",
  postgres: "PostgreSQL",
  mysql: "MySQL",
  mariadb: "MariaDB",
  mongodb: "MongoDB",
  json: "JSON",
  "json-dir": "JSON directory",
};
export const text = {
  en: {
    documents: "Documentation",
    onPage: "On this page",
    contentLanguage: "",
    skip: "Skip to content",
    quickStart: "Quick start",
    quickNote:
      "The installer asks for a database, port and installation directory.",
    themeTitle: "Themes",
    themeIntro:
      "The same feed in three themes. Each theme is one HTML file with its own layout and CSS.",
    themeGuide: "Write a theme",
    language: "Language",
    fullReference: "Full reference",
    install: "Installation",
    gallery: "Gallery",
    docs: "Handbook",
    lead: "th0ughts is a self-hosted microblog for one author. To publish your first post, connect to your Linux server over SSH and run the installer.",
    terminal: "Install on your VPS",
    copy: "Copy",
    copied: "Copied",
    copyFail: "Select and copy the command",
    inspect: "Read the installer",
    screen: "screenshots",
    all: "All",
    previous: "Previous",
    next: "Next",
    close: "Close",
    groups: {
      reader: "Reading",
      publishing: "Publishing",
      appearance: "Appearance",
      access: "Access",
      storage: "Storage",
    },
    enlarge: "Open screenshot",
    installTitle: "Installation",
    methods: [
      "VPS wizard",
      "Docker Compose",
      "Coolify",
      "Dokploy",
      "From source",
    ],
    wizard: "Install on a Linux VPS",
    wizardIntro:
      "Connect to your server over SSH and run the command in an interactive terminal. Choose English or Turkish to get started.",
    steps: [
      "Choose a database. SQLite stores the feed in one local database. The wizard can also provision PostgreSQL, MySQL, MariaDB, MongoDB and two JSON layouts.",
      "Pick a host port, binding address, a new installation directory and a unique Compose project name. Loopback is the default for an HTTPS proxy or SSH tunnel.",
      "Review and confirm. On Ubuntu and Debian the wizard can install missing tools with your permission. On other distributions, install Git, curl, OpenSSL and Docker with the Compose plugin first.",
      "After the health check, open /login through HTTPS or the displayed SSH tunnel. Read the generated master key from .env, choose an owner password and set the public URL on the timeline.",
    ],
    compose: "Run a standalone Compose recipe",
    composeIntro:
      "Clone the repository, choose a database and run these commands from its root. Generate the key once per installation. Keep the data volumes and key when upgrading.",
    choose: "Database",
    jsondir: "JSON directory",
    coolify: "Deploy with Coolify",
    coolifyDetail:
      "Create an application from the Git repository and choose Docker Compose. Set Base Directory to /deploy and Docker Compose Location to /compose.coolify.yaml. Add TH0UGHTS_MASTER_KEY in Environment. Assign an HTTPS domain to app on container port 8080.",
    dokploy: "Deploy with Dokploy",
    dokployDetail:
      "Create a Docker Compose service from the Git repository. Set the Compose path to deploy/compose.dokploy.yaml and add TH0UGHTS_MASTER_KEY in Environment. Under Domains, select app, enter your hostname, choose port 8080 and enable HTTPS.",
    platforms:
      "Both platform recipes use SQLite with a persistent /data volume. After setup, save the public HTTPS origin in Settings → Site.",
    source: "Build the Go server",
    sourceDetail:
      "Install the Go toolchain from go.mod. Go embeds the included React bundle. To change the frontend, install the version of Bun in package.json, then run bun install --frozen-lockfile and bun run build before compiling.",
    full: "Deployment reference",
    back: "Back to top",
    guides: [
      [
        "Publishing",
        "Read the timeline without JavaScript. Open /login directly to publish Markdown, upload media, pin a post or edit an English or Turkish translation.",
        "README.md#publishing",
      ],
      [
        "Single-file themes",
        "Start with one HTML file. Arrange slots, add CSS and expose appearance options. Preview on your timeline and roll back a saved version.",
        "docs/themes.md",
      ],
      [
        "CLI, API and agents",
        "Use scoped tokens with REST or the CLI. Agents can read Markdown and discover tools through WebMCP, MCP over HTTP or stdio.",
        "docs/interfaces.md",
      ],
      [
        "Backups and recovery",
        "Export a portable archive from Settings → Storage. Keep a separate backup of the master key and persistent volumes for a full recovery.",
        "docs/deployment.md#backup-migration-and-recovery",
      ],
      [
        "Owner access",
        "Use a password, a passkey or an authenticator. Revoke individual access tokens from the timeline.",
        "docs/security.md",
      ],
      [
        "Performance",
        "Inspect the workload, latency and memory measurements for SQLite and PostgreSQL, including the reproducible mobile reader test.",
        "docs/performance.md",
      ],
    ],
  },
  tr: {
    documents: "Belgeler",
    onPage: "Bu sayfada",
    contentLanguage: "Belgeler İngilizce yazılmıştır.",
    skip: "İçeriğe geç",
    quickStart: "Hızlı kurulum",
    quickNote: "Sihirbaz veritabanını, portu ve kurulum dizinini sorar.",
    themeTitle: "Temalar",
    themeIntro:
      "Aynı akışın üç farklı temadaki görünümü. Her tema, kendi yerleşimini ve CSS kurallarını içeren tek bir HTML dosyasıdır.",
    themeGuide: "Tema yazma rehberi",
    language: "Dil",
    fullReference: "Ayrıntılı kaynak (İngilizce)",
    install: "Kurulum",
    gallery: "Galeri",
    docs: "Rehber",
    lead: "th0ughts kendi sunucunda çalıştırabileceğin tek yazarlı bir mikroblog. İlk gönderini yayımlamak için Linux sunucuna SSH ile bağlanıp kurulum komutunu çalıştır.",
    terminal: "VPS’ine kur",
    copy: "Kopyala",
    copied: "Kopyalandı",
    copyFail: "Komutu seçip kopyala",
    inspect: "Kurulum betiğini oku",
    screen: "ekran görüntüsü",
    all: "Tümü",
    previous: "Önceki",
    next: "Sonraki",
    close: "Kapat",
    groups: {
      reader: "Okuma",
      publishing: "Yayın",
      appearance: "Görünüm",
      access: "Erişim",
      storage: "Depolama",
    },
    enlarge: "Ekran görüntüsünü aç",
    installTitle: "Kurulum",
    methods: [
      "VPS sihirbazı",
      "Docker Compose",
      "Coolify",
      "Dokploy",
      "Kaynaktan",
    ],
    wizard: "Linux VPS’e kurulum",
    wizardIntro:
      "SSH ile sunucuna bağlan ve komutu etkileşimli bir terminalde çalıştır. Başlamak için İngilizce veya Türkçeyi seç.",
    steps: [
      "Veritabanını seç. SQLite akışı tek bir yerel veritabanında tutar. Sihirbaz ayrıca PostgreSQL, MySQL, MariaDB, MongoDB ve iki JSON düzenini kurabilir.",
      "Portu, dinleme adresini, yeni kurulum dizinini ve benzersiz bir Compose proje adını belirle. Varsayılan yerel bağlantı HTTPS ters proxy veya SSH tüneli içindir.",
      "Seçimlerini inceleyip onayla. Ubuntu ve Debian’da sihirbaz izninle eksik araçları kurabilir. Diğer dağıtımlarda önce Git, curl, OpenSSL, Docker ve Compose eklentisini kur.",
      "Sağlık kontrolünden sonra HTTPS veya verilen SSH tüneliyle /login adresini aç. .env dosyasındaki anahtarla kurulumu başlat, parolanı seç ve akış ayarlarına sitenin açık adresini gir.",
    ],
    compose: "Bağımsız Compose dosyasıyla kurulum",
    composeIntro:
      "Repoyu klonla, veritabanını seç ve bu komutları repo kökünde çalıştır. Anahtarı her kurulum için bir kez üret. Güncellerken veri birimlerini ve anahtarı koru.",
    choose: "Veritabanı",
    jsondir: "JSON dizini",
    coolify: "Coolify ile dağıtım",
    coolifyDetail:
      "Git reposundan bir uygulama oluştur ve Docker Compose seç. Base Directory alanına /deploy, Docker Compose Location alanına /compose.coolify.yaml yaz. Environment bölümüne TH0UGHTS_MASTER_KEY ekle. HTTPS alan adını app servisinin 8080 portuna yönlendir.",
    dokploy: "Dokploy ile dağıtım",
    dokployDetail:
      "Git reposundan Docker Compose servisi oluştur. Compose yolu olarak deploy/compose.dokploy.yaml gir ve Environment bölümüne TH0UGHTS_MASTER_KEY ekle. Domains altında app servisini seç, alan adını gir, 8080 portunu seç ve HTTPS’i aç.",
    platforms:
      "İki platform da SQLite ve kalıcı /data birimi kullanır. Kurulumun ardından herkese açık HTTPS adresini Ayarlar → Site bölümüne kaydet.",
    source: "Go sunucusunu derle",
    sourceDetail:
      "go.mod içinde belirtilen Go sürümünü kur. Go, repodaki React derlemesini içine alır. Ön yüzü değiştirmek için package.json içindeki Bun sürümünü kur, ardından derlemeden önce bun install --frozen-lockfile ve bun run build çalıştır.",
    full: "Dağıtım rehberi",
    back: "Başa dön",
    guides: [
      [
        "Yayınlama",
        "Akışı JavaScript olmadan oku. Markdown yazmak, medya yüklemek, gönderi sabitlemek veya İngilizce ve Türkçe çevirileri düzenlemek için /login adresini aç.",
        "README.md#publishing",
      ],
      [
        "Tek dosyalık temalar",
        "Bir HTML dosyasıyla başla. Yerleşimi belirle, CSS ekle ve görünüm seçeneklerini tanımla. Akışta önizle veya kaydedilmiş önceki sürüme dön.",
        "docs/themes.md",
      ],
      [
        "CLI, API ve ajanlar",
        "REST ve CLI için yetkileri sınırlı tokenlar kullan. Ajanlar akışı Markdown olarak okuyabilir, WebMCP, HTTP üzerinden MCP veya stdio araçlarını keşfedebilir.",
        "docs/interfaces.md",
      ],
      [
        "Yedekleme ve kurtarma",
        "Ayarlar → Depolama bölümünden taşınabilir bir arşiv al. Tam kurtarma için ana anahtarı ve kalıcı birimleri ayrıca yedekle.",
        "docs/deployment.md#backup-migration-and-recovery",
      ],
      [
        "Sahip erişimi",
        "Parola, geçiş anahtarı veya doğrulama uygulamasıyla giriş yap. Erişim tokenlarını akıştan tek tek iptal et.",
        "docs/security.md",
      ],
      [
        "Performans",
        "SQLite ve PostgreSQL için yük, gecikme ve bellek ölçümlerini ve tekrarlanabilir mobil okuma testini incele.",
        "docs/performance.md",
      ],
    ],
  },
};
