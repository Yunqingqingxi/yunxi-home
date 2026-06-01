const { chromium } = require("playwright");
const path = require("path");

const OUT = "D:\\code\\dns-updater-go\\web\\screenshots";
const BASE = "http://localhost:9981";
const TOKEN = "fake-token-for-screenshot";

// Set of pages to screenshot
const pages = [
  { name: "login-light", path: "/#/login", theme: "light", needsAuth: false },
  { name: "dashboard-light", path: "/#/", theme: "light", needsAuth: true },
  { name: "dashboard-dark", path: "/#/", theme: "dark", needsAuth: true },
  { name: "domains-light", path: "/#/domains", theme: "light", needsAuth: true },
  { name: "domains-dark", path: "/#/domains", theme: "dark", needsAuth: true },
  { name: "files-light", path: "/#/files", theme: "light", needsAuth: true },
  { name: "files-dark", path: "/#/files", theme: "dark", needsAuth: true },
  { name: "history-light", path: "/#/history", theme: "light", needsAuth: true },
  { name: "history-dark", path: "/#/history", theme: "dark", needsAuth: true },
  { name: "system-light", path: "/#/system", theme: "light", needsAuth: true },
  { name: "system-dark", path: "/#/system", theme: "dark", needsAuth: true },
  { name: "settings-light", path: "/#/settings", theme: "light", needsAuth: true },
  { name: "settings-dark", path: "/#/settings", theme: "dark", needsAuth: true },
  { name: "chat-light", path: "/#/chat", theme: "light", needsAuth: true },
  { name: "chat-dark", path: "/#/chat", theme: "dark", needsAuth: true },
  { name: "terminal-light", path: "/#/terminal", theme: "light", needsAuth: true },
  { name: "terminal-dark", path: "/#/terminal", theme: "dark", needsAuth: true },
];

(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();

  for (const p of pages) {
    console.log(`Taking screenshot: ${p.name}`);

    // Set auth if needed
    if (p.needsAuth) {
      await page.goto(BASE + "/#/login", { waitUntil: "networkidle" });
      await page.evaluate((token) => {
        localStorage.setItem("token", token);
        localStorage.setItem("yunxi-theme", "");
      }, TOKEN);
    }

    await page.goto(BASE + p.path, { waitUntil: "networkidle" });

    // Set theme
    if (p.theme === "dark") {
      await page.evaluate(() => {
        document.documentElement.setAttribute("data-theme", "dark");
        localStorage.setItem("yunxi-theme", "dark");
      });
    } else {
      await page.evaluate(() => {
        document.documentElement.setAttribute("data-theme", "light");
        localStorage.setItem("yunxi-theme", "light");
      });
    }

    await page.waitForTimeout(1500);
    await page.screenshot({
      path: path.join(OUT, `${p.name}.png`),
      fullPage: false,
    });
  }

  await browser.close();
  console.log("All screenshots taken.");
})();
