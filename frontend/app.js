"use strict";

// Endpoint Agent Dashboard: loads hosts, CIS results and packages from the API.
// Everything from the API is shown with textContent, never as HTML, because the
// values come from the machines being scanned.

const API_URL = window.APP_CONFIG.apiUrl.replace(/\/$/, "");
const KEY_NAME = "agent-api-key";

const statusEl = document.getElementById("status");
const keyForm = document.getElementById("key-form");
const keyFormElement = document.getElementById("key-form-element");
const keyInput = document.getElementById("key-input");
const refreshButton = document.getElementById("refresh");
const forgetButton = document.getElementById("forget");
const hostsSection = document.getElementById("hosts-section");
const hostsTitle = document.getElementById("hosts-title");
const hostsEl = document.getElementById("hosts");

// ---------- the saved API key (browser storage can be blocked, so never rely on it) ----------

function loadKey() {
  try { return localStorage.getItem(KEY_NAME) || ""; } catch (e) { return ""; }
}
function saveKey(key) {
  try { localStorage.setItem(KEY_NAME, key); } catch (e) { /* kept in memory only */ }
}
function clearKey() {
  try { localStorage.removeItem(KEY_NAME); } catch (e) { /* nothing to clear */ }
}

let apiKey = loadKey();

// ---------- small helpers ----------

// el("div", {class: "x"}, "text", childElement) builds an element safely.
function el(tag, attrs, ...children) {
  const node = document.createElement(tag);
  for (const [name, value] of Object.entries(attrs || {})) {
    if (value === true) node.setAttribute(name, "");
    else if (value !== false && value != null) node.setAttribute(name, value);
  }
  for (const child of children) {
    node.append(child instanceof Node ? child : document.createTextNode(String(child)));
  }
  return node;
}

function setStatus(message) { statusEl.textContent = message; }

function formatTime(iso) {
  return iso ? iso.replace("T", " ").slice(0, 16) : "";
}

class ApiError extends Error {
  constructor(message, status) { super(message); this.status = status; }
}

async function apiGet(path) {
  let response;
  try {
    response = await fetch(API_URL + path, { headers: { "x-api-key": apiKey } });
  } catch (e) {
    throw new ApiError("Could not reach the API. Check your connection and try again.", 0);
  }
  if (response.status === 403) throw new ApiError("The API key was not accepted.", 403);
  if (!response.ok) throw new ApiError("The API returned an error (HTTP " + response.status + ").", response.status);
  return response.json();
}

// ---------- status words ----------

const STATUS_CLASS = { PASS: "pass", FAIL: "fail", NOT_APPLICABLE: "na", ERROR: "error" };
const STATUS_LABEL = { PASS: "PASS", FAIL: "FAIL", NOT_APPLICABLE: "N/A", ERROR: "ERROR" };

function summaryCounts(summary) {
  const wrapper = el("span");
  const parts = [
    ["PASS", "pass", "pass"],
    ["FAIL", "fail", "fail"],
    ["NOT_APPLICABLE", "na", "n/a"],
    ["ERROR", "error", "error"],
  ];
  let first = true;
  for (const [key, cssClass, label] of parts) {
    const count = (summary || {})[key];
    if (!count) continue;
    if (!first) wrapper.append(", ");
    wrapper.append(el("span", { class: cssClass }, count + " " + label));
    first = false;
  }
  if (first) wrapper.append("no checks");
  return wrapper;
}

// ---------- the hosts list ----------

function renderHosts(hosts) {
  hostsEl.replaceChildren();
  hostsTitle.textContent = "Hosts: " + hosts.length;

  if (hosts.length === 0) {
    hostsEl.append(el("p", { class: "empty" }, "No hosts have reported yet. Run the agent on a machine (go-agent send)."));
    return;
  }

  hosts.sort((a, b) => (a.hostname || "").localeCompare(b.hostname || ""));
  hosts.forEach((host, index) => hostsEl.append(renderHost(host, index)));
}

function renderHost(host, index) {
  const body = el("div", { class: "host-body" });
  const summary = el("summary", { class: "cols" },
    el("span", { class: "arrow" }, "▶"),
    el("strong", {}, host.hostname || host.machine_id),
    el("span", {}, host.os_pretty_name || ""),
    el("span", { class: "hide-small" }, formatTime(host.last_seen)),
    el("span", {}, host.package_count ?? ""),
    summaryCounts(host.check_summary),
  );
  const details = el("details", { class: "host" }, summary, body);

  // Load the details the first time the row is opened.
  let loaded = false;
  details.addEventListener("toggle", () => {
    if (!details.open || loaded) return;
    loaded = true;
    loadHostDetails(host, index, body);
  });
  return details;
}

async function loadHostDetails(host, index, body) {
  body.replaceChildren(el("p", { class: "loading" }, "Loading…"));
  const id = encodeURIComponent(host.machine_id);
  try {
    const [cis, apps] = await Promise.all([
      apiGet("/cis-results?machine_id=" + id),
      apiGet("/apps?machine_id=" + id),
    ]);
    body.replaceChildren(...renderHostBody(host, index, cis.checks || [], apps.packages || []));
  } catch (e) {
    body.replaceChildren(el("p", { class: "error-text" }, e.message));
  }
}

function renderHostBody(host, index, checks, packages) {
  const meta = el("p", { class: "meta" },
    "Kernel " + (host.kernel || "?") + " · " + (host.architecture || "?") + " · agent " + (host.agent_version || "?"),
    el("br"),
    "Machine ID " + host.machine_id,
  );

  const name = "tab-" + index;
  const tabbed = el("div", { class: "tabbed" },
    el("input", { class: "tab-radio t-cis", type: "radio", name: name, id: name + "-cis", checked: true }),
    el("input", { class: "tab-radio t-pkg", type: "radio", name: name, id: name + "-pkg" }),
    el("div", { class: "tabs" },
      el("label", { class: "l-cis", for: name + "-cis" }, "CIS Results (" + checks.length + ")"),
      el("label", { class: "l-pkg", for: name + "-pkg" }, "Installed Packages (" + packages.length + ")"),
    ),
    el("section", { class: "panel panel-cis" }, checksTable(checks)),
    el("section", { class: "panel panel-pkg" }, ...packagesPanel(packages)),
  );
  return [meta, tabbed];
}

function checksTable(checks) {
  const rows = checks.map((check) => el("tr", {},
    el("td", { class: STATUS_CLASS[check.status] || "" }, STATUS_LABEL[check.status] || check.status),
    el("td", {}, check.cis_section || ""),
    el("td", {}, check.title || check.id),
    el("td", { class: "evidence" }, check.evidence || ""),
  ));
  return el("div", { class: "table-wrap" },
    el("table", {},
      el("thead", {}, el("tr", {}, el("th", {}, "Status"), el("th", {}, "CIS"), el("th", {}, "Check"), el("th", {}, "Evidence"))),
      el("tbody", {}, ...rows),
    ),
  );
}

function packagesPanel(packages) {
  const sorted = [...packages].sort((a, b) => (a.name || "").localeCompare(b.name || ""));
  const rows = sorted.map((pkg) => {
    const row = el("tr", {}, el("td", {}, pkg.name), el("td", {}, pkg.version), el("td", {}, pkg.architecture));
    row.dataset.search = ((pkg.name || "") + " " + (pkg.version || "")).toLowerCase();
    return row;
  });

  const footnote = el("p", { class: "note" }, "Showing " + rows.length + " of " + rows.length + " packages.");
  const input = el("input", { type: "search", placeholder: "Search packages", "aria-label": "Search packages" });
  input.addEventListener("input", () => {
    const query = input.value.trim().toLowerCase();
    let shown = 0;
    for (const row of rows) {
      const match = row.dataset.search.includes(query);
      row.hidden = !match;
      if (match) shown++;
    }
    footnote.textContent = "Showing " + shown + " of " + rows.length + " packages.";
  });

  const table = el("div", { class: "table-wrap" },
    el("table", {},
      el("thead", {}, el("tr", {}, el("th", {}, "Name"), el("th", {}, "Version"), el("th", {}, "Architecture"))),
      el("tbody", {}, ...rows),
    ),
  );
  return [el("div", { class: "search" }, input), table, footnote];
}

// ---------- page flow ----------

function showKeyForm(message) {
  keyForm.hidden = false;
  hostsSection.hidden = true;
  refreshButton.hidden = true;
  forgetButton.hidden = true;
  setStatus(message || "");
  keyInput.focus();
}

async function loadHosts() {
  keyForm.hidden = true;
  refreshButton.hidden = false;
  forgetButton.hidden = false;
  setStatus("Loading hosts…");
  try {
    const hosts = await apiGet("/hosts");
    setStatus("");
    hostsSection.hidden = false;
    renderHosts(hosts);
  } catch (e) {
    if (e.status === 403) {
      clearKey();
      apiKey = "";
      showKeyForm(e.message + " Enter it again.");
    } else {
      hostsSection.hidden = true;
      setStatus(e.message);
    }
  }
}

keyFormElement.addEventListener("submit", (event) => {
  event.preventDefault();
  apiKey = keyInput.value.trim();
  keyInput.value = "";
  if (!apiKey) return;
  saveKey(apiKey);
  loadHosts();
});

refreshButton.addEventListener("click", loadHosts);

forgetButton.addEventListener("click", () => {
  clearKey();
  apiKey = "";
  showKeyForm("The API key was removed from this browser.");
});

if (apiKey) loadHosts();
else showKeyForm();
