const state = {
    Mode: "quick-test",

    FluentBitLogFile: "",
    StoragePath: "",
    LogPaths: [],
    DBPath: "",
    ReadFromHead: true,
    Tag: "",

    Output: "stdout",

    EnableParser: false,
    ParserInput: {
        Format: "json",
        TimeKey: "",
        TimeValue: "",
        Timezone: "GMT",
        ParserName: "",
        Snippet: "",
        RegexFields: [],
        EnableMultiline: false,
        MultilineParserName: "",
        Match: "",
        KeyName: "log"
    },

    LokiHost: "",
    TenantID: "",
    TLSCAFile: "",
    TLSCrtFile: "",
    TLSKeyFile: "",
    LokiLabels: []
};

const $ = (id) => document.getElementById(id);

const els = {
    layout: $("layout"),
    resizeHandle: $("resizeHandle"),

    themeToggleBtn: $("themeToggleBtn"),
    mode: $("mode"),
    modeDescription: $("modeDescription"),

    serviceCard: $("serviceCard"),
    flbLogFileBlock: $("flbLogFileBlock"),
    flbLogFile: $("flbLogFile"),
    storagePathBlock: $("storagePathBlock"),
    storagePath: $("storagePath"),

    logPaths: $("logPaths"),
    dbPathBlock: $("dbPathBlock"),
    dbPath: $("dbPath"),
    tag: $("tag"),

    enableParser: $("enableParser"),
    parserBlock: $("parserBlock"),
    parserFormat: $("parserFormat"),
    timezone: $("timezone"),
    jsonParserBlock: $("jsonParserBlock"),
    regexParserBlock: $("regexParserBlock"),
    jsonTimeKey: $("jsonTimeKey"),
    jsonTimeValue: $("jsonTimeValue"),
    snippet: $("snippet"),
    regexFields: $("regexFields"),
    addRegexFieldBtn: $("addRegexFieldBtn"),
    regexTimeField: $("regexTimeField"),
    enableMultiline: $("enableMultiline"),
    multilineBlock: $("multilineBlock"),
    multilineNameBlock: $("multilineNameBlock"),
    multilineParserName: $("multilineParserName"),
    parserName: $("parserName"),
    keyName: $("keyName"),

    output: $("output"),
    lokiBlock: $("lokiBlock"),
    lokiLabelBlock: $("lokiLabelBlock"),
    enableLokiLabels: $("enableLokiLabels"),
    lokiLabelConfigBlock: $("lokiLabelConfigBlock"),
    lokiLabelHint: $("lokiLabelHint"),
    lokiLabels: $("lokiLabels"),
    addLokiLabelBtn: $("addLokiLabelBtn"),
    lokiHost: $("lokiHost"),
    tenantID: $("tenantID"),
    tlsCAFile: $("tlsCAFile"),
    tlsCrtFile: $("tlsCrtFile"),
    tlsKeyFile: $("tlsKeyFile"),

    yamlPreview: $("yamlPreview"),
    statusBadge: $("statusBadge"),
    warnings: $("warnings"),
    copyYamlBtn: $("copyYamlBtn"),
    downloadYamlBtn: $("downloadYamlBtn")
};

let latestYaml = "";
let debounceTimer = null;

function setVisible(el, visible) {
    if (!el) {
        return;
    }

    el.classList.toggle("hidden", !visible);
}

function parseLogPaths(raw) {
    return raw
        .split(/[\n,]/)
        .map((v) => v.trim())
        .filter(Boolean);
}

function syncStateFromDOM() {
    const isQuickTest = els.mode.value === "quick-test";

    state.Mode = els.mode.value;
    state.Output = els.output.value;

    state.FluentBitLogFile = isQuickTest ? "" : els.flbLogFile.value.trim();
    state.StoragePath = isQuickTest ? "" : els.storagePath.value.trim();
    state.LogPaths = parseLogPaths(els.logPaths.value);
    state.DBPath = isQuickTest ? "" : els.dbPath.value.trim();

    state.ReadFromHead = isQuickTest;

    state.Tag = els.tag.value.trim();

    state.EnableParser = els.enableParser.checked;

    state.ParserInput.Format = els.parserFormat.value;
    state.ParserInput.Timezone = els.timezone.value;
    state.ParserInput.ParserName = els.parserName.value.trim();
    state.ParserInput.Match = state.Tag;
    state.ParserInput.KeyName = els.keyName.value.trim() || "log";

    if (state.ParserInput.Format === "json") {
        state.ParserInput.TimeKey = els.jsonTimeKey.value.trim();
        state.ParserInput.TimeValue = els.jsonTimeValue.value.trim();
        state.ParserInput.Snippet = "";
        state.ParserInput.RegexFields = [];
        state.ParserInput.EnableMultiline = false;
        state.ParserInput.MultilineParserName = "";
    } else {
        state.ParserInput.Snippet = els.snippet.value;
        state.ParserInput.RegexFields = collectRegexFields();

        const selectedTimeField = els.regexTimeField.value;
        const timeField = state.ParserInput.RegexFields.find((f) => f.Key === selectedTimeField);

        state.ParserInput.TimeKey = timeField ? timeField.Key : "";
        state.ParserInput.TimeValue = timeField ? timeField.Value : "";
        state.ParserInput.EnableMultiline = els.enableMultiline.checked;
        state.ParserInput.MultilineParserName = els.multilineParserName.value.trim();
    }

    state.LokiHost = els.lokiHost.value.trim();
    state.TenantID = els.tenantID.value.trim();
    state.TLSCAFile = els.tlsCAFile.value.trim();
    state.TLSCrtFile = els.tlsCrtFile.value.trim();
    state.TLSKeyFile = els.tlsKeyFile.value.trim();
    state.LokiLabels = collectLokiLabels();
}

function buildGeneratePayload() {
    syncStateFromDOM();

    const payload = structuredClone(state);
    const frontendWarnings = [];

    if (payload.EnableParser && !isParserReady(payload)) {
        payload.EnableParser = false;

        payload.ParserInput = {
            Format: payload.ParserInput.Format,
            TimeKey: "",
            TimeValue: "",
            Timezone: payload.ParserInput.Timezone,
            ParserName: "",
            Snippet: "",
            RegexFields: [],
            EnableMultiline: false,
            MultilineParserName: "",
            Match: "",
            KeyName: "log"
        };

        payload.LokiLabels = [];

        frontendWarnings.push("Parser: parser is enabled but not complete yet; YAML preview is generated without parser section");
        return { payload, frontendWarnings };
    }

    if (
        payload.EnableParser &&
        payload.ParserInput.Format === "regex" &&
        payload.ParserInput.EnableMultiline &&
        !payload.ParserInput.MultilineParserName.trim()
    ) {
        payload.ParserInput.EnableMultiline = false;
        payload.ParserInput.MultilineParserName = "";

        frontendWarnings.push("Parser: multiline is enabled but parser name is empty; YAML preview is generated without multiline parser section");
    }

    if (!isAdditionalLokiLabelsEnabled()) {
        payload.LokiLabels = [];
    }

    return { payload, frontendWarnings };
}

function isParserReady(payload) {
    const parserInput = payload.ParserInput;

    if (!payload.EnableParser) {
        return true;
    }

    if (!parserInput.ParserName.trim()) {
        return false;
    }

    if (!parserInput.KeyName.trim()) {
        return false;
    }

    if (parserInput.Format === "json") {
        return parserInput.TimeKey.trim() !== "" && parserInput.TimeValue.trim() !== "";
    }

    if (parserInput.Format === "regex") {
        if (!parserInput.Snippet.trim()) {
            return false;
        }

        if (!parserInput.TimeKey.trim() || !parserInput.TimeValue.trim()) {
            return false;
        }

        if (!parserInput.RegexFields || parserInput.RegexFields.length === 0) {
            return false;
        }

        const hasInvalidField = parserInput.RegexFields.some((field) => {
            return !field.Key.trim() || !field.Value.trim();
        });

        if (hasInvalidField) {
            return false;
        }

        return true;
    }

    return false;
}

function isAdditionalLokiLabelsEnabled() {
    return Boolean(
        els.enableLokiLabels &&
        els.enableLokiLabels.checked &&
        els.output.value === "loki" &&
        els.enableParser.checked &&
        els.parserFormat.value === "regex"
    );
}

function renderVisibility() {
    const isQuickTest = els.mode.value === "quick-test";
    const parserEnabled = els.enableParser.checked;
    const isRegexParser = els.parserFormat.value === "regex";
    const isLokiOutput = els.output.value === "loki";

    setVisible(els.serviceCard, !isQuickTest);
    setVisible(els.flbLogFileBlock, !isQuickTest);
    setVisible(els.storagePathBlock, !isQuickTest);

    setVisible(els.dbPathBlock, !isQuickTest);

    setVisible(els.parserBlock, parserEnabled);
    setVisible(els.jsonParserBlock, parserEnabled && els.parserFormat.value === "json");
    setVisible(els.regexParserBlock, parserEnabled && isRegexParser);

    setVisible(els.multilineBlock, parserEnabled && isRegexParser);
    setVisible(els.multilineNameBlock, parserEnabled && isRegexParser && els.enableMultiline.checked);

    setVisible(els.lokiBlock, isLokiOutput);
    setVisible(els.lokiLabelBlock, isLokiOutput && parserEnabled && isRegexParser);
    setVisible(els.lokiLabelConfigBlock, isAdditionalLokiLabelsEnabled());

    renderModeDescription();
}

function renderModeDescription() {
    if (!els.modeDescription) {
        return;
    }

    if (els.mode.value === "quick-test") {
        els.modeDescription.textContent =
            "Quick test generates a lightweight config without filesystem buffering and tail offset database.";
        return;
    }

    els.modeDescription.textContent =
        "Production enables filesystem buffering and tail offset database for long-running log collection.";
}

function collectRegexFields() {
    return [...els.regexFields.querySelectorAll(".regex-field-row")]
        .map((row) => ({
            Key: row.querySelector(".regex-key").value.trim(),
            Value: row.querySelector(".regex-value").value.trim()
        }))
        .filter((f) => f.Key !== "" || f.Value !== "");
}

function renderRegexFields() {
    els.regexFields.innerHTML = "";

    state.ParserInput.RegexFields.forEach((field, index) => {
        const row = document.createElement("div");
        row.className = "row regex-field-row";
        row.innerHTML = `
      <label>
        Field name
        <input class="regex-key" value="${escapeAttr(field.Key)}" placeholder="${escapeAttr(suggestRegexFieldName(index))}" />
      </label>
      <label>
        Exact value
        <input class="regex-value" value="${escapeAttr(field.Value)}" placeholder="${escapeAttr(suggestRegexFieldValue(index))}" />
      </label>
      <button type="button" class="remove-btn">Remove</button>
    `;

        row.querySelector(".remove-btn").addEventListener("click", () => {
            state.ParserInput.RegexFields.splice(index, 1);
            renderRegexFields();
            renderLokiLabels();
            scheduleGenerate();
        });

        row.querySelectorAll("input").forEach((input) => {
            input.addEventListener("input", () => {
                state.ParserInput.RegexFields = collectRegexFields();
                renderRegexTimeOptions();
                renderLokiLabels();
                scheduleGenerate();
            });

            input.addEventListener("blur", () => {
                const trimmed = input.value.trim();
                if (input.value !== trimmed) {
                    input.value = trimmed;
                    state.ParserInput.RegexFields = collectRegexFields();
                    renderRegexTimeOptions();
                    renderLokiLabels();
                    scheduleGenerate();
                }
            });
        });

        els.regexFields.appendChild(row);
    });

    renderRegexTimeOptions();
}

function suggestRegexFieldName(index) {
    const suggestions = ["timestamp", "thread", "level", "user", "message"];
    return suggestions[index] || "field";
}

function suggestRegexFieldValue(index) {
    const suggestions = [
        "2026-05-27 12:00:00,126+0800",
        "Caesium-1-3",
        "ERROR",
        "itsm_jsm_qa",
        "Error when importing objects"
    ];

    return suggestions[index] || "value";
}

function renderRegexTimeOptions() {
    const current = els.regexTimeField.value;
    const fields = collectRegexFields().filter((f) => f.Key.trim() !== "");

    els.regexTimeField.innerHTML = "";

    if (fields.length === 0) {
        const option = document.createElement("option");
        option.value = "";
        option.textContent = "Add extracted fields first";
        els.regexTimeField.appendChild(option);

        els.regexTimeField.value = "";
        els.regexTimeField.disabled = true;
        return;
    }

    els.regexTimeField.disabled = false;

    fields.forEach((field) => {
        const option = document.createElement("option");
        option.value = field.Key;
        option.textContent = field.Key;
        els.regexTimeField.appendChild(option);
    });

    if (fields.some((f) => f.Key === current)) {
        els.regexTimeField.value = current;
    }
}

function collectLokiLabels() {
    if (!els.lokiLabels) {
        return [];
    }

    return [...els.lokiLabels.querySelectorAll(".loki-label-row")]
        .map((row) => ({
            Field: row.querySelector(".loki-field").value.trim(),
            Name: row.querySelector(".loki-name").value.trim()
        }))
        .filter((label) => label.Field !== "" || label.Name !== "");
}

function getUsedLokiFields(exceptIndex) {
    const labels = collectLokiLabels();
    const used = new Set();

    labels.forEach((label, index) => {
        if (index !== exceptIndex && label.Field) {
            used.add(label.Field);
        }
    });

    return used;
}

function renderLokiLabels() {
    clearLokiLabelHint();

    const labels = state.LokiLabels;
    els.lokiLabels.innerHTML = "";

    labels.forEach((label, index) => {
        const row = document.createElement("div");
        row.className = "row loki-label-row";

        row.innerHTML = `
      <label>
        Parser field
        <select class="loki-field">${buildLokiFieldOptions(label.Field, index)}</select>
      </label>
      <label>
        Loki label name
        <input class="loki-name" value="${escapeAttr(label.Name)}" placeholder="severity" />
      </label>
      <button type="button" class="remove-btn">Remove</button>
    `;

        row.querySelector(".remove-btn").addEventListener("click", () => {
            state.LokiLabels.splice(index, 1);
            renderLokiLabels();
            scheduleGenerate();
        });

        const nameInput = row.querySelector(".loki-name");
        const fieldSelect = row.querySelector(".loki-field");

        nameInput.addEventListener("input", () => {
            state.LokiLabels = collectLokiLabels();
            scheduleGenerate();
        });

        fieldSelect.addEventListener("change", () => {
            state.LokiLabels = collectLokiLabels();
            renderLokiLabels();
            scheduleGenerate();
        });

        els.lokiLabels.appendChild(row);
    });
}

function buildLokiFieldOptions(current, index) {
    const fields = collectRegexFields()
        .map((f) => f.Key.trim())
        .filter(Boolean);

    const used = getUsedLokiFields(index);

    return fields
        .filter((field) => field === current || !used.has(field))
        .map((field) => {
            const selected = field === current ? "selected" : "";
            return `<option value="${escapeAttr(field)}" ${selected}>${escapeHTML(field)}</option>`;
        })
        .join("");
}

function addRegexField() {
    state.ParserInput.RegexFields = collectRegexFields();
    state.ParserInput.RegexFields.push({ Key: "", Value: "" });
    renderRegexFields();
    scheduleGenerate();
}

function addLokiLabel() {
    clearLokiLabelHint();

    state.LokiLabels = collectLokiLabels();

    const fields = collectRegexFields()
        .map((field) => field.Key.trim())
        .filter(Boolean);

    if (fields.length === 0) {
        showLokiLabelHint("Add extracted fields first, then choose which fields should become Loki labels.");
        return;
    }

    const used = new Set(state.LokiLabels.map((label) => label.Field));
    const firstAvailable = fields.find((key) => !used.has(key));

    if (!firstAvailable) {
        showLokiLabelHint("All extracted fields are already added as Loki labels.");
        return;
    }

    state.LokiLabels.push({
        Field: firstAvailable,
        Name: firstAvailable
    });

    renderLokiLabels();
    scheduleGenerate();
}

function showLokiLabelHint(message) {
    if (!els.lokiLabelHint) {
        return;
    }

    els.lokiLabelHint.textContent = message;
    els.lokiLabelHint.classList.remove("hidden");
}

function clearLokiLabelHint() {
    if (!els.lokiLabelHint) {
        return;
    }

    els.lokiLabelHint.textContent = "";
    els.lokiLabelHint.classList.add("hidden");
}

function scheduleGenerate() {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(generateYAML, 250);
}

async function generateYAML() {
    const { payload, frontendWarnings } = buildGeneratePayload();
    renderVisibility();

    try {
        const response = await fetch("/api/generate", {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: JSON.stringify(payload)
        });

        const data = await response.json();

        if (!response.ok) {
            setStatus("error", "Error");
            latestYaml = "";
            els.yamlPreview.textContent = data.error || "failed to generate YAML";
            renderWarnings([...frontendWarnings]);
            return;
        }

        latestYaml = data.yaml || "";
        els.yamlPreview.textContent = latestYaml || "No YAML generated";

        renderWarnings([
            ...(data.warnings || []),
            ...frontendWarnings
        ]);

        setStatus("ok", "Ready");
    } catch (error) {
        setStatus("error", "Error");
        latestYaml = "";
        els.yamlPreview.textContent = error.message;
        renderWarnings(frontendWarnings);
    }
}

function renderWarnings(warnings) {
    if (!warnings || warnings.length === 0) {
        els.warnings.classList.add("hidden");
        els.warnings.innerHTML = "";
        return;
    }

    const groupedWarnings = groupWarningsBySection(warnings);

    els.warnings.classList.remove("hidden");
    els.warnings.innerHTML = `
    <strong>Warnings</strong>
    <div class="warning-groups">
      ${groupedWarnings.map((group) => `
        <div class="warning-group">
          <div class="warning-section">${escapeHTML(group.section)}</div>
          <ul>
            ${group.messages.map((message) => `<li>${escapeHTML(message)}</li>`).join("")}
          </ul>
        </div>
      `).join("")}
    </div>
  `;
}

function groupWarningsBySection(warnings) {
    const groups = [];
    const groupMap = new Map();

    warnings.forEach((warning) => {
        const parsed = parseWarning(warning);

        if (!groupMap.has(parsed.section)) {
            const group = {
                section: parsed.section,
                messages: []
            };

            groupMap.set(parsed.section, group);
            groups.push(group);
        }

        groupMap.get(parsed.section).messages.push(parsed.message);
    });

    return groups;
}

function parseWarning(warning) {
    const index = warning.indexOf(":");

    if (index === -1) {
        return {
            section: "General",
            message: warning.trim()
        };
    }

    const section = warning.slice(0, index).trim();
    const message = warning.slice(index + 1).trim();

    if (!section || !message) {
        return {
            section: "General",
            message: warning.trim()
        };
    }

    return {
        section,
        message
    };
}

function setStatus(type, text) {
    els.statusBadge.className = `badge ${type}`;
    els.statusBadge.textContent = text;
}

async function copyYAML() {
    if (!latestYaml) {
        showCopyFeedback("No YAML", false);
        return;
    }

    try {
        if (navigator.clipboard && window.isSecureContext) {
            await navigator.clipboard.writeText(latestYaml);
        } else {
            fallbackCopyText(latestYaml);
        }

        showCopyFeedback("Copied!", true);
    } catch (error) {
        console.error("failed to copy YAML:", error);
        showCopyFeedback("Copy failed", false);
    }
}

function fallbackCopyText(text) {
    const textarea = document.createElement("textarea");

    textarea.value = text;
    textarea.setAttribute("readonly", "");
    textarea.style.position = "fixed";
    textarea.style.top = "-9999px";
    textarea.style.left = "-9999px";

    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();

    const ok = document.execCommand("copy");
    document.body.removeChild(textarea);

    if (!ok) {
        throw new Error("document.execCommand copy failed");
    }
}

function showCopyFeedback(text, success) {
    const originalText = "Copy YAML";

    els.copyYamlBtn.textContent = text;
    els.copyYamlBtn.classList.toggle("copied", success);
    els.copyYamlBtn.classList.toggle("copy-failed", !success);

    setTimeout(() => {
        els.copyYamlBtn.textContent = originalText;
        els.copyYamlBtn.classList.remove("copied", "copy-failed");
    }, 1200);
}

function downloadYAML() {
    if (!latestYaml) {
        return;
    }

    const blob = new Blob([latestYaml], { type: "application/x-yaml" });
    const url = URL.createObjectURL(blob);

    const link = document.createElement("a");
    link.href = url;
    link.download = `fluent-bit-${formatDownloadTimestamp(new Date())}.yaml`;
    link.click();

    URL.revokeObjectURL(url);
}

function formatDownloadTimestamp(date) {
    const pad = (value) => String(value).padStart(2, "0");

    const year = date.getFullYear();
    const month = pad(date.getMonth() + 1);
    const day = pad(date.getDate());
    const hour = pad(date.getHours());
    const minute = pad(date.getMinutes());
    const second = pad(date.getSeconds());

    return `${year}${month}${day}${hour}${minute}${second}`;
}

function applyTheme(theme) {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem("flbctl.theme", theme);

    if (els.themeToggleBtn) {
        els.themeToggleBtn.textContent = theme === "dark" ? "Light mode" : "Dark mode";
    }
}

function toggleTheme() {
    const current = document.documentElement.getAttribute("data-theme") || "dark";
    applyTheme(current === "dark" ? "light" : "dark");
}

function bindResizablePanels() {
    if (!els.layout || !els.resizeHandle) {
        return;
    }

    let dragging = false;

    els.resizeHandle.addEventListener("mousedown", () => {
        dragging = true;
        document.body.classList.add("resizing");
    });

    window.addEventListener("mousemove", (event) => {
        if (!dragging) {
            return;
        }

        const rect = els.layout.getBoundingClientRect();
        const leftWidth = event.clientX - rect.left;

        const min = 420;
        const max = Math.min(900, rect.width - 560);
        const nextWidth = Math.max(min, Math.min(max, leftWidth));

        els.layout.style.setProperty("--left-width", `${nextWidth}px`);
        localStorage.setItem("flbctl.leftWidth", String(nextWidth));
    });

    window.addEventListener("mouseup", () => {
        if (!dragging) {
            return;
        }

        dragging = false;
        document.body.classList.remove("resizing");
    });

    const saved = Number(localStorage.getItem("flbctl.leftWidth"));
    if (saved > 0) {
        els.layout.style.setProperty("--left-width", `${saved}px`);
    }
}

function escapeHTML(value) {
    return String(value)
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;");
}

function escapeAttr(value) {
    return escapeHTML(value).replaceAll('"', "&quot;");
}

function bindEvents() {
    document.querySelectorAll("input, select, textarea").forEach((el) => {
        el.addEventListener("input", () => {
            syncStateFromDOM();
            renderVisibility();
            scheduleGenerate();
        });

        el.addEventListener("change", () => {
            syncStateFromDOM();
            renderVisibility();
            scheduleGenerate();
        });
    });

    els.addRegexFieldBtn.addEventListener("click", addRegexField);
    els.addLokiLabelBtn.addEventListener("click", addLokiLabel);
    els.copyYamlBtn.addEventListener("click", copyYAML);
    els.downloadYamlBtn.addEventListener("click", downloadYAML);
    els.themeToggleBtn.addEventListener("click", toggleTheme);

    bindResizablePanels();
}

function initDefaults() {
    const savedTheme = localStorage.getItem("flbctl.theme") || "dark";
    applyTheme(savedTheme);

    els.mode.value = "quick-test";
    els.output.value = "stdout";
    els.timezone.value = "GMT";
    els.parserFormat.value = "json";

    if (els.enableLokiLabels) {
        els.enableLokiLabels.checked = false;
    }

    renderVisibility();
    renderRegexFields();
    renderLokiLabels();
    generateYAML();
}

bindEvents();
initDefaults();