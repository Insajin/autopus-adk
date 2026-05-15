package run

func guiPolicyGuardScript() string {
	return `const fs = require("fs");
const Module = require("module");
const policyPath = process.env.AUTOPUS_QAMESH_GUI_POLICY_PATH;
const readyPath = process.env.AUTOPUS_QAMESH_GUI_GUARD_READY_PATH;
let policy = {
  allowed_origins: (process.env.AUTOPUS_QAMESH_GUI_ALLOWED_ORIGINS || "").split(",").filter(Boolean),
  forbidden_actions: (process.env.AUTOPUS_QAMESH_GUI_FORBIDDEN_ACTIONS || "").split(",").filter(Boolean)
};
try {
  if (policyPath) policy = JSON.parse(fs.readFileSync(policyPath, "utf8"));
} catch (_) {}
const allowedOriginValues = (policy.allowed_origins || []).map((value) => String(value).trim()).filter(Boolean);
const allowedOrigins = new Set(allowedOriginValues.map((value) => absoluteOriginOf(value)).filter(Boolean));
const forbiddenActions = new Set((policy.forbidden_actions || []).map((value) => String(value).trim().toLowerCase()).filter(Boolean));
function absoluteOriginOf(value) {
  try { return new URL(String(value)).origin.toLowerCase(); } catch (_) { return ""; }
}
function baseCandidates() {
  const values = [
    process.env.PLAYWRIGHT_BASE_URL,
    process.env.AUTOPUS_DESKTOP_GUI_BASE_URL,
    ...allowedOriginValues
  ];
  return values.map((value) => String(value || "").trim()).filter(Boolean);
}
function originOfTarget(value) {
  const absolute = absoluteOriginOf(value);
  if (absolute) return absolute;
  for (const base of baseCandidates()) {
    try { return new URL(String(value), base).origin.toLowerCase(); } catch (_) {}
  }
  return "";
}
function ensureURLAllowed(target) {
  const origin = originOfTarget(target);
  if (!origin || !allowedOrigins.has(origin)) {
    throw new Error("AUTOPUS_QAMESH_GUI_OFF_ORIGIN:" + origin);
  }
}
function shouldBlockAction(method) {
  method = String(method).toLowerCase();
  return forbiddenActions.has(method) ||
    (forbiddenActions.has("mutation") && ["click", "dblclick", "tap", "fill", "press", "check", "uncheck", "selectoption", "setinputfiles", "dragto"].includes(method));
}
function blockAction(method) {
  if (shouldBlockAction(method)) {
    throw new Error("AUTOPUS_QAMESH_GUI_FORBIDDEN_ACTION:" + method);
  }
}
function patchMethod(target, name, wrapper) {
  if (!target || typeof target[name] !== "function") return;
  const original = target[name];
  if (original.__autopusQameshGuard) return;
  target[name] = wrapper(original);
  target[name].__autopusQameshGuard = true;
}
function patchLocator(locator) {
  for (const name of ["click", "dblclick", "tap", "fill", "press", "check", "uncheck", "selectOption", "setInputFiles", "dragTo"]) {
    patchMethod(locator, name, (original) => function(...args) {
      blockAction(name);
      return original.apply(this, args);
    });
  }
  return locator;
}
function patchPage(page) {
  if (!page || page.__autopusQameshGuarded) return page;
  Object.defineProperty(page, "__autopusQameshGuarded", { value: true, configurable: true });
  patchMethod(page, "goto", (original) => function(target, ...rest) {
    ensureURLAllowed(target);
    return original.call(this, target, ...rest);
  });
  patchMethod(page, "locator", (original) => function(...args) {
    return patchLocator(original.apply(this, args));
  });
  patchMethod(page, "getByRole", (original) => function(...args) {
    return patchLocator(original.apply(this, args));
  });
  for (const name of ["click", "dblclick", "tap", "fill", "press", "check", "uncheck", "selectOption", "setInputFiles", "dragTo"]) {
    patchMethod(page, name, (original) => function(...args) {
      blockAction(name);
      return original.apply(this, args);
    });
  }
  return page;
}
function patchBrowser(browser) {
  if (!browser || browser.__autopusQameshGuarded) return browser;
  Object.defineProperty(browser, "__autopusQameshGuarded", { value: true, configurable: true });
  patchMethod(browser, "newPage", (original) => async function(...args) {
    return patchPage(await original.apply(this, args));
  });
  patchMethod(browser, "newContext", (original) => async function(...args) {
    const context = await original.apply(this, args);
    patchMethod(context, "newPage", (newPage) => async function(...pageArgs) {
      return patchPage(await newPage.apply(this, pageArgs));
    });
    return context;
  });
  return browser;
}
function patchBrowserType(browserType) {
  if (!browserType || browserType.__autopusQameshGuarded) return browserType;
  Object.defineProperty(browserType, "__autopusQameshGuarded", { value: true, configurable: true });
  patchMethod(browserType, "launch", (original) => async function(...args) {
    return patchBrowser(await original.apply(this, args));
  });
  patchMethod(browserType, "launchPersistentContext", (original) => async function(...args) {
    const context = await original.apply(this, args);
    patchMethod(context, "newPage", (newPage) => async function(...pageArgs) {
      return patchPage(await newPage.apply(this, pageArgs));
    });
    return context;
  });
  return browserType;
}
function patchExports(exports) {
  for (const name of ["chromium", "firefox", "webkit"]) {
    if (exports && exports[name]) patchBrowserType(exports[name]);
  }
  return exports;
}
const load = Module._load;
Module._load = function(request, parent, isMain) {
  const exports = load.apply(this, arguments);
  if (String(request).includes("playwright")) return patchExports(exports);
  return exports;
};
if (readyPath) {
  fs.writeFileSync(readyPath, JSON.stringify({
    schema_version: "autopus.qamesh.gui_guard.v1",
    installed: true,
    hooks: ["node_preload", "playwright_module", "page_navigation", "locator_actions"],
    loaded_at: new Date().toISOString()
  }) + "\n");
}
`
}
