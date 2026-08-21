(() => {
  const API_URL = "https://api.github.com/repos/Distortions81/goThoom/releases/latest";
  const RELEASES_URL = "https://github.com/Distortions81/goThoom/releases/latest";

  const platforms = [
    { key: "windows", name: "Windows", detail: "64-bit · Windows 10 or later", icon: "windows-logo.svg", matches: ["windows"] },
    { key: "macos", name: "macOS", detail: "Apple Silicon & Intel", icon: "apple-logo.svg", matches: ["macos", "darwin", "apple"] },
    { key: "linux", name: "Linux", detail: "64-bit · x86_64", icon: "linux-logo.svg", matches: ["linux"] },
  ];

  const grid = document.querySelector("#download-grid");
  const error = document.querySelector("#release-error");
  const version = document.querySelector("#release-version");
  const date = document.querySelector("#release-date");
  const checksums = document.querySelector("#checksums-link");

  function detectedPlatform() {
    const value = `${navigator.userAgent} ${navigator.platform || ""}`.toLowerCase();
    if (value.includes("win")) return "windows";
    if (value.includes("mac")) return "macos";
    if (value.includes("linux")) return "linux";
    return "";
  }

  function fileSize(bytes) {
    if (!Number.isFinite(bytes) || bytes <= 0) return "ZIP archive";
    const units = ["B", "KB", "MB", "GB"];
    const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const value = bytes / (1024 ** index);
    return `${value.toFixed(index > 1 ? 1 : 0)} ${units[index]}`;
  }

  function findAssets(assets, platform) {
    return assets.filter((asset) => {
      const name = asset.name.toLowerCase();
      return name.endsWith(".zip") && platform.matches.some((match) => name.includes(match));
    });
  }

  function architecture(name) {
    const lower = name.toLowerCase();
    if (lower.includes("applesilicon") || lower.includes("arm64") || lower.includes("aarch64")) return "Apple Silicon";
    if (lower.includes("intel") || lower.includes("x86_64") || lower.includes("amd64")) return "Intel / x86_64";
    return "Download";
  }

  function renderCard(platform, assets, recommended) {
    const article = document.createElement("article");
    article.className = `download-card${recommended ? " recommended" : ""}`;
    article.dataset.platform = platform.key;

    const actions = assets.length
      ? assets.map((asset, index) => `
          <a class="asset-button${index === 0 ? " main-asset" : ""}" href="${asset.browser_download_url}">
            <span>${assets.length > 1 ? architecture(asset.name) : "Download .zip"}<small>${fileSize(asset.size)}</small></span>
            <b aria-hidden="true">↓</b>
          </a>`).join("")
      : `<a class="asset-button main-asset" href="${RELEASES_URL}"><span>View on GitHub<small>Build unavailable</small></span><b aria-hidden="true">↗</b></a>`;

    article.innerHTML = `
      ${recommended ? '<span class="recommended-badge">Recommended</span>' : ""}
      <div class="platform-icon platform-${platform.key}"><img src="${platform.icon}" alt="${platform.name} logo"></div>
      <div class="card-title"><h3>${platform.name}</h3><p>${platform.detail}</p></div>
      <div class="asset-list">${actions}</div>`;
    return article;
  }

  async function loadRelease() {
    try {
      const response = await fetch(API_URL, { headers: { Accept: "application/vnd.github+json" } });
      if (!response.ok) throw new Error(`GitHub returned ${response.status}`);
      const release = await response.json();
      const assets = Array.isArray(release.assets) ? release.assets : [];
      const detected = detectedPlatform();

      version.textContent = release.name || release.tag_name || "Latest release";
      date.textContent = release.published_at
        ? `Published ${new Intl.DateTimeFormat(undefined, { dateStyle: "medium" }).format(new Date(release.published_at))}`
        : "Available now";

      grid.replaceChildren(...platforms.map((platform) =>
        renderCard(platform, findAssets(assets, platform), platform.key === detected)
      ));
      grid.setAttribute("aria-busy", "false");

      const checksumAsset = assets.find((asset) => asset.name.toLowerCase() === "sha256sums.txt");
      if (checksumAsset) {
        checksums.href = checksumAsset.browser_download_url;
        checksums.classList.remove("disabled");
        checksums.removeAttribute("aria-disabled");
      }
    } catch (fetchError) {
      version.textContent = "Latest release";
      date.textContent = "Available on GitHub";
      grid.hidden = true;
      grid.setAttribute("aria-busy", "false");
      error.hidden = false;
    }
  }

  loadRelease();
})();
