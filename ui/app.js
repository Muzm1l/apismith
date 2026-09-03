const state = {
  catalog: null,
  environments: [],
  envId: "",
  endpoint: null,
  bodyMode: "form",
  formValues: {},
  lastResponse: null,
  lastRaw: "",
  responseTab: "body",
};

const $ = (id) => document.getElementById(id);

async function api(path, opts = {}) {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
    ...opts,
  });
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = { raw: text }; }
  if (!res.ok) {
    const msg = (data && (data.error || data.message)) || res.statusText;
    throw new Error(msg);
  }
  return data;
}

function methodClass(m) { return `method ${m}`; }

function currentEnv() {
  return state.environments.find((e) => e.id === state.envId) || state.environments[0];
}

function selectedAuthMode() {
  const el = document.querySelector("input[name=auth-mode]:checked");
  return el ? el.value : "jwt";
}

function renderEnvironments(payload) {
  state.environments = payload.environments || [];
  state.envId = payload.default || (state.environments[0] && state.environments[0].id) || "";
  const sel = $("environment");
  sel.innerHTML = state.environments.map((e) =>
    `<option value="${e.id}" ${e.id === state.envId ? "selected" : ""}>${e.name}</option>`
  ).join("");
  applyEnv();
}

function applyEnv() {
  const env = currentEnv();
  if (!env) return;
  $("prod-badge").classList.toggle("hidden", !env.production);
  $("base-url").value = env.base_url || "";
  if (env.client_id && !$("client-id").value) $("client-id").value = env.client_id;
  $("username").placeholder = env.has_stored_credentials ? "using server credentials" : "Cognito username / email";
  $("password").placeholder = env.has_stored_credentials ? "using server credentials" : "Cognito password";
}

function renderCatalog(catalog) {
  state.catalog = catalog;
  $("spec-title").textContent = `${catalog.title || "OpenAPI"} · ${catalog.endpoints.length} endpoints`;
  renderEndpointList();
}

function renderEndpointList() {
  const q = ($("api-search").value || "").toLowerCase().trim();
  const groups = new Map();
  for (const ep of state.catalog.endpoints) {
    const hay = `${ep.method} ${ep.path} ${ep.summary || ""} ${ep.operation_id || ""} ${(ep.tags || []).join(" ")}`.toLowerCase();
    if (q && !hay.includes(q)) continue;
    const tag = (ep.tags && ep.tags[0]) || "untagged";
    if (!groups.has(tag)) groups.set(tag, []);
    groups.get(tag).push(ep);
  }
  const nav = $("endpoint-list");
  if (!groups.size) {
    nav.innerHTML = `<p class="muted">No matching APIs.</p>`;
    return;
  }
  nav.innerHTML = [...groups.entries()].map(([tag, eps]) => `
    <div class="tag-group">
      <h4>${tag}</h4>
      ${eps.map((ep) => `
        <button type="button" class="ep ${state.endpoint && state.endpoint.id === ep.id ? "active" : ""}" data-id="${encodeURIComponent(ep.id)}">
          <span class="${methodClass(ep.method)}">${ep.method}</span>
          <span class="path">${ep.path}</span>
        </button>
      `).join("")}
    </div>
  `).join("");
  nav.querySelectorAll(".ep").forEach((btn) => {
    btn.addEventListener("click", () => {
      const id = decodeURIComponent(btn.dataset.id);
      selectEndpoint(state.catalog.endpoints.find((e) => e.id === id));
    });
  });
}

function selectEndpoint(ep) {
  state.endpoint = ep;
  state.formValues = {};
  $("btn-send").disabled = !ep;
  $("btn-copy-request").disabled = !ep;
  renderEndpointList();
  if (!ep) return;

  $("op-title").textContent = ep.summary || ep.operation_id || ep.id;
  $("op-desc").textContent = ep.description || "";
  $("op-method").textContent = ep.method;
  $("op-method").className = methodClass(ep.method);
  $("op-path").textContent = ep.path;
  $("op-auth").classList.remove("hidden");
  $("op-auth").textContent = ep.auth_required ? "Auth required" : "Public";
  $("op-auth").className = "chip " + (ep.auth_required ? "warn" : "ok");

  renderParams(ep);
  renderBody(ep);
}

function renderParams(ep) {
  const wrap = $("params-wrap");
  const box = $("params");
  const params = ep.parameters || [];
  if (!params.length) {
    wrap.classList.add("hidden");
    box.innerHTML = "";
    return;
  }
  wrap.classList.remove("hidden");
  box.innerHTML = params.map((p) => {
    const ex = p.example != null ? String(p.example) : "";
    return `<label class="param">
      <span class="name">${p.name}${p.required ? ' <span class="req">*</span>' : ""} <small class="muted">${p.in}</small></span>
      <input data-param-in="${p.in}" data-param-name="${p.name}" placeholder="${ex || p.description || p.name}" />
    </label>`;
  }).join("");
}

function schemaFields(schema, prefix = "") {
  if (!schema) return [];
  if (schema.type === "object" || schema.properties) {
    const req = new Set(schema.required || []);
    return Object.entries(schema.properties || {}).map(([name, prop]) => ({
      name: prefix ? `${prefix}.${name}` : name,
      key: name,
      required: req.has(name),
      schema: prop,
    }));
  }
  return [];
}

function renderBody(ep) {
  const wrap = $("body-wrap");
  if (!ep.request_body) {
    wrap.classList.add("hidden");
    $("body-form").innerHTML = "";
    $("body-json").value = "";
    return;
  }
  wrap.classList.remove("hidden");
  const example = ep.request_body.example_json || (ep.request_body.example
    ? JSON.stringify(ep.request_body.example, null, 2)
    : "{\n  \n}");
  $("body-json").value = example;
  const fields = schemaFields(ep.request_body.schema);
  if (!fields.length) {
    state.bodyMode = "json";
  }
  $("body-form").innerHTML = fields.map((f) => {
    const t = (f.schema && f.schema.type) || "string";
    const inputType = t === "integer" || t === "number" ? "number" : (f.schema && f.schema.format === "password" ? "password" : "text");
    const ex = f.schema && f.schema.example != null ? String(f.schema.example) : "";
    return `<label class="param">
      <span class="name">${f.key}${f.required ? ' <span class="req">*</span>' : ""}</span>
      <input data-form-key="${f.key}" type="${inputType}" placeholder="${ex || t}" />
    </label>`;
  }).join("");
  setBodyMode(state.bodyMode);
}

function setBodyMode(mode) {
  state.bodyMode = mode;
  document.querySelectorAll("[data-body-mode]").forEach((b) => {
    b.classList.toggle("active", b.dataset.bodyMode === mode);
  });
  $("body-form").classList.toggle("hidden", mode !== "form");
  $("body-json-wrap").classList.toggle("hidden", mode !== "json");
}

function bodyFromForm() {
  const obj = {};
  $("body-form").querySelectorAll("[data-form-key]").forEach((el) => {
    if (el.value === "") return;
    obj[el.dataset.formKey] = el.type === "number" ? Number(el.value) : el.value;
  });
  return Object.keys(obj).length ? JSON.stringify(obj, null, 2) : ($("body-json").value || "");
}

function collectRequest() {
  const ep = state.endpoint;
  const pathParams = {};
  const query = {};
  const headers = {};
  document.querySelectorAll("[data-param-in]").forEach((el) => {
    if (!el.value) return;
    if (el.dataset.paramIn === "path") pathParams[el.dataset.paramName] = el.value;
    else if (el.dataset.paramIn === "query") query[el.dataset.paramName] = el.value;
    else if (el.dataset.paramIn === "header") headers[el.dataset.paramName] = el.value;
  });
  let body = "";
  if (ep.request_body) {
    body = state.bodyMode === "form" ? bodyFromForm() : $("body-json").value;
  }
  return {
    environment: state.envId,
    base_url: $("base-url").value.trim(),
    method: ep.method,
    path: ep.path,
    path_params: pathParams,
    query,
    headers,
    body,
    auth_mode: selectedAuthMode(),
    jwt: $("jwt").value.trim(),
    custom_authorization: $("custom-auth").value.trim(),
  };
}

async function confirmProductionIfNeeded(payload) {
  const env = currentEnv();
  if (!env || !env.production) return payload;
  const dialog = $("prod-dialog");
  $("prod-dialog-detail").textContent = `${payload.method} ${payload.path} → ${payload.base_url || env.base_url || ""}`;
  dialog.showModal();
  return new Promise((resolve) => {
    dialog.addEventListener("close", function onClose() {
      dialog.removeEventListener("close", onClose);
      if (dialog.returnValue === "confirm") {
        resolve({ ...payload, confirm_production: true });
      } else {
        resolve(null);
      }
    });
  });
}

async function sendRequest() {
  if (!state.endpoint) return;
  if (state.bodyMode === "json" && $("body-json").value.trim()) {
    try { JSON.parse($("body-json").value); $("json-error").textContent = ""; }
    catch (err) { $("json-error").textContent = err.message; return; }
  }
  let payload = collectRequest();
  payload = await confirmProductionIfNeeded(payload);
  if (!payload) return;

  $("btn-send").disabled = true;
  $("response-meta").textContent = "Sending…";
  try {
    const data = await api("/api/request", { method: "POST", body: JSON.stringify(payload) });
    showResponse(data.response);
    await loadHistory();
  } catch (err) {
    showResponse({ error: err.message, status: 0, status_text: "client error", duration_ms: 0, body: err.message });
  } finally {
    $("btn-send").disabled = false;
  }
}

function showResponse(resp) {
  state.lastResponse = resp;
  const status = resp.status || 0;
  const ok = status >= 200 && status < 400;
  $("response-meta").innerHTML = resp.error && !status
    ? `<span class="status bad">${escapeHtml(resp.error)}</span>`
    : `<span class="status ${ok ? "ok" : "bad"}">${status} ${escapeHtml(resp.status_text || "")}</span>
       <span>${resp.duration_ms || 0} ms</span>
       <span>${escapeHtml(resp.content_type || "")}</span>`;
  state.lastRaw = resp.body || resp.error || "";
  renderResponseTab();
  $("btn-copy-response").disabled = !state.lastRaw;
}

function renderResponseTab() {
  const resp = state.lastResponse;
  const view = $("response-view");
  if (!resp) { view.textContent = "Awaiting a request."; return; }
  if (state.responseTab === "headers") {
    view.textContent = JSON.stringify(resp.headers || {}, null, 2);
    return;
  }
  if (state.responseTab === "raw") {
    view.textContent = JSON.stringify(resp, null, 2);
    return;
  }
  view.textContent = resp.body || resp.error || "";
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

async function generateJWT() {
  $("btn-generate").disabled = true;
  try {
    const data = await api("/api/auth/jwt", {
      method: "POST",
      body: JSON.stringify({
        environment: state.envId,
        client_id: $("client-id").value.trim(),
        username: $("username").value.trim(),
        password: $("password").value,
      }),
    });
    $("jwt").value = data.access_token || "";
    setAuthStatus(true, data.status);
  } catch (err) {
    setAuthStatus(false, null, err.message);
  } finally {
    $("btn-generate").disabled = false;
  }
}

function setAuthStatus(ok, status, err) {
  const el = $("auth-status");
  if (err) {
    el.className = "status bad";
    el.textContent = err;
    return;
  }
  if (ok || (status && status.authenticated)) {
    el.className = "status ok";
    el.textContent = "Authenticated";
  } else {
    el.className = "status muted";
    el.textContent = "Not authenticated";
  }
}

async function loadHistory() {
  const items = await api("/api/history");
  const box = $("history-list");
  if (!items.length) {
    box.className = "history-list muted";
    box.textContent = "No requests yet.";
    return;
  }
  box.className = "history-list";
  box.innerHTML = items.map((it) => `
    <button type="button" class="hist" data-id="${it.id}">
      <div class="row">
        <span>${it.ok ? "✓" : "✗"}</span>
        <span class="${methodClass(it.method)}">${it.method}</span>
        <span>${it.path}</span>
      </div>
      <div class="meta">${it.status || "—"} · ${it.duration_ms}ms · ${it.environment}</div>
    </button>
  `).join("");
  box.querySelectorAll(".hist").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const item = await api(`/api/history/${btn.dataset.id}`);
      restoreHistory(item);
    });
  });
}

function restoreHistory(item) {
  const ep = state.catalog.endpoints.find((e) => e.method === item.method && e.path === item.path);
  if (ep) selectEndpoint(ep);
  if (item.environment) {
    state.envId = item.environment;
    $("environment").value = item.environment;
    applyEnv();
  }
  document.querySelectorAll("[data-param-in]").forEach((el) => {
    const bag = el.dataset.paramIn === "path" ? item.path_params
      : el.dataset.paramIn === "query" ? item.query
      : item.headers;
    el.value = (bag && bag[el.dataset.paramName]) || "";
  });
  if (item.body) {
    $("body-json").value = item.body;
    setBodyMode("json");
  }
}

function copy(text) {
  navigator.clipboard.writeText(text || "");
}

function bindUI() {
  $("environment").addEventListener("change", (e) => {
    state.envId = e.target.value;
    applyEnv();
  });
  $("api-search").addEventListener("input", renderEndpointList);
  document.querySelectorAll("input[name=auth-mode]").forEach((el) => {
    el.addEventListener("change", () => {
      const mode = selectedAuthMode();
      $("jwt-fields").classList.toggle("hidden", mode === "custom");
      $("custom-auth-wrap").classList.toggle("hidden", mode !== "custom");
    });
  });
  document.querySelectorAll("[data-body-mode]").forEach((b) => {
    b.addEventListener("click", () => {
      if (state.bodyMode === "form" && b.dataset.bodyMode === "json") {
        const fromForm = bodyFromForm();
        if (fromForm) $("body-json").value = fromForm;
      }
      setBodyMode(b.dataset.bodyMode);
    });
  });
  document.querySelectorAll("[data-tab]").forEach((b) => {
    b.addEventListener("click", () => {
      state.responseTab = b.dataset.tab;
      document.querySelectorAll("[data-tab]").forEach((x) => x.classList.toggle("active", x === b));
      renderResponseTab();
    });
  });
  $("btn-generate").addEventListener("click", generateJWT);
  $("btn-copy-jwt").addEventListener("click", () => copy($("jwt").value));
  $("btn-send").addEventListener("click", sendRequest);
  $("btn-format-json").addEventListener("click", () => {
    try {
      $("body-json").value = JSON.stringify(JSON.parse($("body-json").value || "{}"), null, 2);
      $("json-error").textContent = "";
    } catch (err) {
      $("json-error").textContent = err.message;
    }
  });
  $("body-json").addEventListener("input", () => {
    if (!$("body-json").value.trim()) { $("json-error").textContent = ""; return; }
    try { JSON.parse($("body-json").value); $("json-error").textContent = ""; }
    catch (err) { $("json-error").textContent = err.message; }
  });
  $("btn-copy-response").addEventListener("click", () => copy(state.lastRaw));
  $("btn-copy-request").addEventListener("click", () => {
    const p = collectRequest();
    copy(`${p.method} ${p.path}\n${p.body || ""}`.trim());
  });
  $("btn-clear-history").addEventListener("click", async () => {
    await api("/api/history/clear", { method: "POST" });
    await loadHistory();
  });
}

async function boot() {
  bindUI();
  const [health, envs, catalog] = await Promise.all([
    api("/api/health"),
    api("/api/environments"),
    api("/api/catalog"),
  ]);
  $("spec-title").textContent = `${health.title} · ${health.endpoints} endpoints`;
  renderEnvironments(envs);
  renderCatalog(catalog);
  try {
    const status = await api("/api/auth/status");
    setAuthStatus(status.authenticated, status);
  } catch { /* ignore */ }
  await loadHistory();
}

boot().catch((err) => {
  $("spec-title").textContent = "failed to load: " + err.message;
});
