#!/usr/bin/env bash
# Interactive installer. The Pages build pins SOURCE_REF to the published commit.
set -euo pipefail
SOURCE_REF=main
repository=https://github.com/sametcn99/th0ughts.git
language=en

say() { if [[ $language == tr ]]; then printf '%s\n' "$2"; else printf '%s\n' "$1"; fi; }
fail() { say "$1" "$2" >&2; exit 1; }
ask() {
  local label
  label=$(say "$1" "$2")
  printf '%s [%s] > ' "$label" "$3" >&2
  IFS= read -r reply <&3 || fail "Input ended." "Girdi sona erdi."
  reply=${reply:-$3}
}
yes() { [[ $1 == y || $1 == Y || $1 == e || $1 == E ]]; }

install_tools() {
  local -a missing=() admin=()
  local tool
  for tool in git openssl curl; do
    command -v "$tool" >/dev/null || missing+=("$tool")
  done
  if command -v docker >/dev/null && docker compose version >/dev/null 2>&1 && (("${#missing[@]}" == 0)); then return; fi
  # shellcheck disable=SC1091
  source /etc/os-release
  [[ $ID == ubuntu || $ID == debian ]] || fail "Install Git, curl, OpenSSL and Docker with Compose, then rerun." "Git, curl, OpenSSL ve Docker Compose kurup yeniden çalıştırın."
  ask "Install missing prerequisites using apt and Docker's signed repository? y/n" "Eksik araçlar apt ve Docker'ın imzalı deposundan kurulsun mu? e/h" n
  yes "$reply" || exit 0
  if ((EUID != 0)); then admin=(sudo); fi
  "${admin[@]}" apt-get update
  "${admin[@]}" apt-get install -y ca-certificates curl openssl git
  if ! docker compose version >/dev/null 2>&1; then
    if command -v docker >/dev/null; then
      fail "Add the Compose plugin for your existing Docker installation and rerun." "Mevcut Docker kurulumuna Compose eklentisini ekleyip yeniden çalıştırın."
    fi
    local distro_url="https://download.docker.com/linux/$ID"
    local config=/etc/apt/sources.list.d/th0ughts-docker.list
    if ! grep -RqsF "$distro_url" /etc/apt/sources.list.d /etc/apt/sources.list; then
      "${admin[@]}" install -m 0755 -d /etc/apt/keyrings
      curl -fsSL "$distro_url/gpg" -o "$scratch/docker.asc"
      "${admin[@]}" install -m 0644 "$scratch/docker.asc" /etc/apt/keyrings/th0ughts-docker.asc
      printf 'deb [arch=%s signed-by=/etc/apt/keyrings/th0ughts-docker.asc] %s %s stable\n' "$(dpkg --print-architecture)" "$distro_url" "${UBUNTU_CODENAME:-$VERSION_CODENAME}" > "$scratch/docker.list"
      "${admin[@]}" install -m 0644 "$scratch/docker.list" "$config"
    fi
    "${admin[@]}" apt-get update
    "${admin[@]}" apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  fi
}

main() {
  local source_dir= choice driver port bind destination project scratch
  case "${1:-}" in
    --source) [[ $# == 2 && -d $2 ]] || exit 2; source_dir=$(cd "$2" && pwd) ;;
    --help) echo 'Usage: bash install.sh [--source CHECKOUT]'; return ;;
    '') ;;
    *) exit 2 ;;
  esac
  [[ $(uname -s) == Linux ]] || fail "Run this wizard on a Linux VPS." "Bu sihirbazı bir Linux VPS üzerinde çalıştırın."
  if [[ -f ${BASH_SOURCE[0]:-} ]]; then exec 3<&0; else exec 3</dev/tty; fi
  printf '\nth0ughts / VPS\n\n'
  ask "Language / Dil (en, tr)" "Dil" en
  [[ $reply == en || $reply == tr ]] || exit 2
  language=$reply
  say "Choose a database. Database servers run in their own private containers." "Veritabanını seçin. Veritabanı sunucuları kendi özel konteynerlerinde çalışır."
  say "1 SQLite (single local database)   2 PostgreSQL   3 MySQL   4 MariaDB   5 MongoDB" "1 SQLite (tek yerel veritabanı)   2 PostgreSQL   3 MySQL   4 MariaDB   5 MongoDB"
  say "6 Single JSON file   7 JSON directory (one file per record)" "6 Tek JSON dosyası   7 JSON dizini (kayıt başına bir dosya)"
  local drivers=(sqlite postgres mysql mariadb mongodb json json-dir)
  while :; do ask "Database (1–7)" "Veritabanı (1–7)" 1; [[ $reply =~ ^[1-7]$ ]] && break; done
  driver=${drivers[reply-1]}
  while :; do ask "Web port (1–65535)" "Web portu (1–65535)" 8080; [[ $reply =~ ^[1-9][0-9]{0,4}$ ]] && ((reply <= 65535)) && break; done
  port=$reply
  say "Loopback works with an HTTPS reverse proxy or SSH tunnel. Public binding exposes HTTP on every IPv4 interface." "Yerel bağlantı HTTPS ters proxy veya SSH tüneli içindir. Genel bağlantı HTTP portunu tüm IPv4 arayüzlerinde açar."
  while :; do ask "Bind address (127.0.0.1 or 0.0.0.0)" "Dinleme adresi (127.0.0.1 veya 0.0.0.0)" 127.0.0.1; [[ $reply == 127.0.0.1 || $reply == 0.0.0.0 ]] && break; done
  bind=$reply
  ask "New installation directory (absolute path)" "Yeni kurulum dizini (mutlak yol)" "$HOME/th0ughts"
  destination=$reply
  [[ $destination == /* && ! -e $destination && ! -L $destination ]] || fail "Choose a new absolute directory. Existing installations can be managed with ./manage." "Yeni bir mutlak dizin seçin. Mevcut kurulumu ./manage ile yönetebilirsiniz."
  while :; do ask "Compose project (unique volume namespace)" "Compose projesi (birimlere verilen benzersiz ad)" th0ughts; [[ $reply =~ ^[a-z0-9][a-z0-9_-]{0,47}$ ]] && break; done
  project=$reply
  printf '\n%s  |  %s:%s  |  %s  |  %s\n' "$driver" "$bind" "$port" "$project" "$destination"
  say "A random master key will be saved in .env with owner-only permissions." "Rastgele bir ana anahtar .env dosyasına yalnızca sahibinin okuyabileceği şekilde kaydedilecek."
  ask "Install with these settings? y/n" "Bu ayarlarla kurulsun mu? e/h" n
  yes "$reply" || return 0
  umask 077
  scratch=$(mktemp -d)
  trap "$(printf 'rm -rf -- %q' "$scratch")" EXIT
  install_tools
  local -a docker_cmd=(docker)
  if ! docker info >/dev/null 2>&1; then
    if ((EUID != 0)) && command -v sudo >/dev/null && sudo docker info >/dev/null; then docker_cmd=(sudo docker)
    else fail "Start Docker and check daemon access." "Docker hizmetini başlatıp erişimi kontrol edin."; fi
  fi
  local kind existing
  for kind in container volume network; do
    local -a opts=()
    [[ $kind != container ]] || opts=(-a)
    existing=$("${docker_cmd[@]}" "$kind" ls "${opts[@]}" -q --filter "label=com.docker.compose.project=$project")
    [[ -z $existing ]] || fail "That Compose project already has resources. Choose another name." "Bu Compose projesinin kaynakları var. Başka bir ad seçin."
  done
  if [[ -n $source_dir ]]; then
    (umask 022; git clone --quiet --no-local --depth 1 "$source_dir" "$scratch/source")
  else
    (umask 022
     git init --quiet "$scratch/source"
     git -C "$scratch/source" remote add origin "$repository"
     git -C "$scratch/source" fetch --quiet --depth 1 origin "$SOURCE_REF"
     git -C "$scratch/source" checkout --quiet --detach FETCH_HEAD)
  fi
  mkdir -p -- "$(dirname "$destination")"
  mkdir -m 700 -- "$destination"
  mv -- "$scratch/source" "$destination/source"
  cd "$destination"
  local key
  key=$(openssl rand -hex 32)
  printf 'TH0UGHTS_MASTER_KEY=%s\n' "$key" > .env
  unset key
  {
    printf '#!/usr/bin/env bash\nset -eu\ncd -- "$(dirname -- "$0")"\n'
    printf 'privilege=()\nif ! docker info >/dev/null 2>&1 && ((EUID != 0)); then privilege=(sudo); fi\n'
    printf 'exec "${privilege[@]}" env -u TH0UGHTS_MASTER_KEY TH0UGHTS_LISTEN=%q docker compose --env-file "$PWD/.env" -p %q -f "source/deploy/compose.%s.yaml" "$@"\n' "$bind:$port" "$project" "$driver"
  } > manage
  chmod 700 manage
  say "Building the app. To inspect or retry, use ./manage logs or ./manage up -d --build --wait in the installation directory." "Uygulama derleniyor. Kayıtlar için kurulum dizininde ./manage logs, yeniden denemek için ./manage up -d --build --wait kullanın."
  ./manage config --quiet
  ./manage up -d --build --wait --wait-timeout 300
  curl --fail --silent --show-error --retry 5 --retry-connrefused "http://127.0.0.1:$port/healthz" >/dev/null
  say "Healthy. Open /login through your HTTPS proxy or the SSH tunnel below. Use the master key for first setup, then choose your password." "Hazır. HTTPS proxy veya aşağıdaki SSH tüneli üzerinden /login adresini açın. İlk kurulumda ana anahtarı kullanıp parolanızı seçin."
  printf 'ssh -L %s:127.0.0.1:%s USER@VPS\nhttp://localhost:%s/login\n' "$port" "$port" "$port"
  say "Master key and backups: keep the .env file private and back it up with your data volumes." "Ana anahtar ve yedekler: .env dosyasını gizli tutun, veri birimleri ile birlikte yedekleyin."
  printf 'cd %q\nsed -n "s/^TH0UGHTS_MASTER_KEY=//p" .env\n./manage ps\n' "$destination"
}
main "$@"
