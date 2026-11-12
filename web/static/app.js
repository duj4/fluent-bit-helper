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
    ParserInputs: [],

    LokiHost: "",
    TenantID: "",
    TLSCAFile: "",
    TLSCrtFile: "",
    TLSKeyFile: "",
    LokiLineFormat: "json",
    LokiLabels: [],
    LokiStructuredMetadata: []
};

const $ = (id) => document.getElementById(id);

const els = {
    layout: $("layout"),
    resizeHandle: $("resizeHandle"),

    generateViewBtn: $("generateViewBtn"),
    doctorViewBtn: $("doctorViewBtn"),
    generateForm: $("generateForm"),
    doctorForm: $("doctorForm"),

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
    parserFormatsBlock: $("parserFormatsBlock"),
    parserFormats: $("parserFormats"),
    addParserFormatBtn: $("addParserFormatBtn"),
    removeParserFormatBtn: $("removeParserFormatBtn"),
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

    structuredMetadataBlock: $("structuredMetadataBlock"),
    enableStructuredMetadata: $("enableStructuredMetadata"),
    structuredMetadataConfigBlock: $("structuredMetadataConfigBlock"),
    structuredMetadataHint: $("structuredMetadataHint"),
    structuredMetadata: $("structuredMetadata"),
    addStructuredMetadataBtn: $("addStructuredMetadataBtn"),

    lokiHost: $("lokiHost"),
    tenantID: $("tenantID"),
    tlsCAFile: $("tlsCAFile"),
    tlsCrtFile: $("tlsCrtFile"),
    tlsKeyFile: $("tlsKeyFile"),
    lokiLineFormat: $("lokiLineFormat"),

    doctorYamlInput: $("doctorYamlInput"),
    doctorBaseline: $("doctorBaseline"),
    doctorHint: $("doctorHint"),

    previewTitle: $("previewTitle"),
    yamlPreview: $("yamlPreview"),
    doctorResult: $("doctorResult"),
    statusBadge: $("statusBadge"),
    warnings: $("warnings"),
    copyYamlBtn: $("copyYamlBtn"),
    downloadYamlBtn: $("downloadYamlBtn")
};

let latestYaml = "";
let generateIsValid = false;
let activeView = "generate";
let debounceTimer = null;
let doctorDebounceTimer = null;
let doctorRequestSeq = 0;
let activeParserInputIndex = 0;

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

function normalizeLokiInstance(value) {
    return value.trim().replace(/^https?:\/\/(.+)$/i, "$1");
}

function defaultParserInput(index = 0) {
    return {
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
    };
}

function cloneParserInput(parserInput) {
    return structuredClone(parserInput || defaultParserInput());
}

function ensureParserInputs() {
    if (!state.ParserInputs || state.ParserInputs.length === 0) {
        state.ParserInputs = [cloneParserInput(state.ParserInput)];
    }

    if (activeParserInputIndex >= state.ParserInputs.length) {
        activeParserInputIndex = state.ParserInputs.length - 1;
    }

    state.ParserInput = state.ParserInputs[activeParserInputIndex];
}

function sharedMultilineParserName() {
    ensureParserInputs();

    const input = state.ParserInputs.find((parserInput) => parserInput.MultilineParserName && parserInput.MultilineParserName.trim());
    return input ? input.MultilineParserName.trim() : "";
}

function syncActiveParserInputFromDOM() {
    ensureParserInputs();

    const parserInput = state.ParserInputs[activeParserInputIndex];
    const sharedMultilineParserName = els.multilineParserName.value.trim();
    parserInput.Format = els.parserFormat.value;
    parserInput.Timezone = els.timezone.value;
    parserInput.ParserName = els.parserName.value.trim();
    parserInput.Match = state.Tag;
    parserInput.KeyName = els.keyName.value.trim() || "log";

    if (parserInput.Format === "json") {
        parserInput.TimeKey = els.jsonTimeKey.value.trim();
        parserInput.TimeValue = els.jsonTimeValue.value.trim();
        parserInput.Snippet = "";
        parserInput.RegexFields = [];
        parserInput.EnableMultiline = false;
        parserInput.MultilineParserName = "";
    } else {
        parserInput.Snippet = els.snippet.value;
        parserInput.RegexFields = collectRegexFields();

        const selectedTimeField = els.regexTimeField.value;
        const timeField = parserInput.RegexFields.find((f) => f.Key === selectedTimeField);

        parserInput.TimeKey = timeField ? timeField.Key : "";
        parserInput.TimeValue = timeField ? timeField.Value : "";
        parserInput.EnableMultiline = els.enableMultiline.checked;
        parserInput.MultilineParserName = sharedMultilineParserName;
    }

    state.ParserInputs.forEach((item) => {
        if (item.Format === "regex" && sharedMultilineParserName) {
            item.MultilineParserName = sharedMultilineParserName;
        }
    });

    if (parserInput.EnableMultiline && sharedMultilineParserName) {
        state.ParserInputs.forEach((item) => {
            if (item.Format === "regex") {
                item.EnableMultiline = true;
            }
        });
    }

    state.ParserInput = parserInput;
}

function loadActiveParserInputIntoDOM() {
    ensureParserInputs();

    const parserInput = state.ParserInputs[activeParserInputIndex];
    state.ParserInput = parserInput;

    els.parserFormat.value = parserInput.Format || "json";
    els.timezone.value = parserInput.Timezone || "GMT";
    els.parserName.value = parserInput.ParserName || "";
    els.keyName.value = parserInput.KeyName || "log";
    els.jsonTimeKey.value = parserInput.TimeKey || "";
    els.jsonTimeValue.value = parserInput.TimeValue || "";
    els.snippet.value = parserInput.Snippet || "";
    els.enableMultiline.checked = Boolean(parserInput.EnableMultiline);
    els.multilineParserName.value = sharedMultilineParserName() || parserInput.MultilineParserName || "";

    renderRegexFields();
    if (parserInput.TimeKey) {
        els.regexTimeField.value = parserInput.TimeKey;
    }
    renderParserFormats();
    renderVisibility();
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

    syncActiveParserInputFromDOM();

    state.LokiHost = normalizeLokiInstance(els.lokiHost.value);
    if (els.lokiHost.value !== state.LokiHost) {
        els.lokiHost.value = state.LokiHost;
    }

    state.TenantID = els.tenantID.value.trim();
    state.TLSCAFile = els.tlsCAFile.value.trim();
    state.TLSCrtFile = els.tlsCrtFile.value.trim();
    state.TLSKeyFile = els.tlsKeyFile.value.trim();
    state.LokiLineFormat = normalizeLokiLineFormat(els.lokiLineFormat ? els.lokiLineFormat.value : "json");
    state.LokiLabels = collectLokiLabels();
    state.LokiStructuredMetadata = collectStructuredMetadata();
    removeStructuredMetadataFieldsAlreadyUsedAsLabels();
}

function buildGeneratePayload() {
    syncStateFromDOM();

    const payload = structuredClone(state);
    const frontendWarnings = [];
    payload.ParserInputs = normalizePayloadParserInputs(payload);
    payload.ParserInput = payload.ParserInputs[0] || payload.ParserInput;

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
        payload.ParserInputs = [];

        payload.LokiLabels = [];
        payload.LokiStructuredMetadata = [];

        frontendWarnings.push("Parser: parser is enabled but not complete yet; YAML preview is generated without parser section");
        return { payload, frontendWarnings };
    }

    if (
        payload.EnableParser &&
        payload.ParserInputs.some((parserInput) => parserInput.Format === "regex" && parserInput.EnableMultiline) &&
        !payload.ParserInputs.some((parserInput) => parserInput.EnableMultiline && parserInput.MultilineParserName.trim())
    ) {
        payload.ParserInputs.forEach((parserInput) => {
            parserInput.EnableMultiline = false;
            parserInput.MultilineParserName = "";
        });
        payload.ParserInput = payload.ParserInputs[0] || payload.ParserInput;

        frontendWarnings.push("Parser: multiline is enabled but parser name is empty; YAML preview is generated without multiline parser section");
    }

    if (!isAdditionalLokiLabelsEnabled()) {
        payload.LokiLabels = [];
    }

    if (!isStructuredMetadataEnabled()) {
        payload.LokiStructuredMetadata = [];
    }

    return { payload, frontendWarnings };
}

function normalizePayloadParserInputs(payload) {
    if (!payload.EnableParser) {
        return [];
    }

    const inputs = (payload.ParserInputs && payload.ParserInputs.length > 0)
        ? payload.ParserInputs
        : [payload.ParserInput];

    return inputs
        .map((parserInput) => ({
            ...parserInput,
            Match: payload.Tag,
            KeyName: parserInput.KeyName && parserInput.KeyName.trim() ? parserInput.KeyName.trim() : "log"
        }))
        .filter((parserInput) => parserInput.ParserName.trim() || parserInput.TimeKey.trim() || parserInput.TimeValue.trim());
}


function normalizeLokiLineFormat(value) {
    if (value === "key_value") {
        return "key_value";
    }

    return "json";
}

function isParserReady(payload) {
    if (!payload.EnableParser) {
        return true;
    }

    const parserInputs = normalizePayloadParserInputs(payload);
    if (parserInputs.length === 0) {
        return false;
    }

    return parserInputs.every(isSingleParserReady);
}

function isSingleParserReady(parserInput) {
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
    ensureParserInputs();

    return Boolean(
        els.enableLokiLabels &&
        els.enableLokiLabels.checked &&
        els.output.value === "loki" &&
        els.enableParser.checked &&
        state.ParserInputs.some((parserInput) => parserInput.Format === "regex")
    );
}

function isStructuredMetadataEnabled() {
    ensureParserInputs();

    return Boolean(
        els.enableStructuredMetadata &&
        els.enableStructuredMetadata.checked &&
        els.output.value === "loki" &&
        els.enableParser.checked &&
        state.ParserInputs.some((parserInput) => parserInput.Format === "regex")
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
    setVisible(els.structuredMetadataBlock, isLokiOutput && parserEnabled && isRegexParser);
    setVisible(els.structuredMetadataConfigBlock, isStructuredMetadataEnabled());

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
            Value: row.querySelector(".regex-value").value.trim(),
            Optional: row.querySelector(".regex-optional")?.checked || false,
            Pattern: row.querySelector(".regex-pattern")?.value.trim() || ""
        }))
        .filter((f) => f.Key !== "" || f.Value !== "" || f.Pattern !== "");
}

function renderParserFormats() {
    ensureParserInputs();

    if (!els.parserFormats) {
        return;
    }

    els.parserFormats.innerHTML = "";

    state.ParserInputs.forEach((parserInput, index) => {
        const button = document.createElement("button");
        button.type = "button";
        button.className = `secondary format-tab${index === activeParserInputIndex ? " active" : ""}`;
        button.textContent = parserInput.ParserName || `Format ${index + 1}`;

        button.addEventListener("click", () => {
            if (index === activeParserInputIndex) {
                return;
            }

            syncStateFromDOM();
            activeParserInputIndex = index;
            loadActiveParserInputIntoDOM();
            renderLokiLabels();
            renderStructuredMetadata();
            scheduleGenerate();
        });

        els.parserFormats.appendChild(button);
    });

    if (els.removeParserFormatBtn) {
        els.removeParserFormatBtn.disabled = state.ParserInputs.length <= 1;
    }
}

function addParserFormat() {
    syncStateFromDOM();
    const parserInput = defaultParserInput(state.ParserInputs.length);
    parserInput.MultilineParserName = sharedMultilineParserName();
    parserInput.EnableMultiline = state.ParserInputs.some((item) => item.EnableMultiline);
    state.ParserInputs.push(parserInput);
    activeParserInputIndex = state.ParserInputs.length - 1;
    loadActiveParserInputIntoDOM();
    renderLokiLabels();
    renderStructuredMetadata();
    scheduleGenerate();
}

function removeParserFormat() {
    ensureParserInputs();

    if (state.ParserInputs.length <= 1) {
        return;
    }

    state.ParserInputs.splice(activeParserInputIndex, 1);
    activeParserInputIndex = Math.max(0, activeParserInputIndex - 1);
    loadActiveParserInputIntoDOM();
    renderLokiLabels();
    renderStructuredMetadata();
    scheduleGenerate();
}

function renderRegexFields() {
    els.regexFields.innerHTML = "";

    state.ParserInput.RegexFields.forEach((field, index) => {
        const row = document.createElement("div");
        row.className = "regex-field-row";
        row.innerHTML = `
      <div class="regex-field-main">
        <label>
          Field name
          <input class="regex-key" value="${escapeAttr(field.Key || "")}" placeholder="${escapeAttr(suggestRegexFieldName(index))}" />
        </label>
        <label>
          Exact value
          <input class="regex-value" value="${escapeAttr(field.Value || "")}" placeholder="${escapeAttr(suggestRegexFieldValue(index))}" />
        </label>
        <button type="button" class="remove-btn">Remove</button>
      </div>
      <div class="regex-field-extra">
        <div class="checkbox-row regex-optional-row">
          <input class="regex-optional" id="regexOptional${index}" type="checkbox" ${field.Optional ? "checked" : ""} />
          <label for="regexOptional${index}">Optional field</label>
        </div>
        <label class="regex-pattern-field">
          Field value pattern
          <input class="regex-pattern" value="${escapeAttr(field.Pattern || "")}" placeholder="Default for optional: [^\s\[]+" />
          <small>Optional. Leave empty to match one token that does not contain whitespace or [. Use a custom pattern for stricter matching.</small>
        </label>
      </div>
    `;

        row.querySelector(".remove-btn").addEventListener("click", () => {
            state.ParserInput.RegexFields.splice(index, 1);
            renderRegexFields();
            renderLokiLabels();
            renderStructuredMetadata();
            scheduleGenerate();
        });

        row.querySelectorAll("input").forEach((input) => {
            input.addEventListener("input", () => {
                state.ParserInput.RegexFields = collectRegexFields();
                renderRegexTimeOptions();
                renderLokiLabels();
                renderStructuredMetadata();
                scheduleGenerate();
            });

            input.addEventListener("change", () => {
                state.ParserInput.RegexFields = collectRegexFields();
                renderRegexTimeOptions();
                renderLokiLabels();
                renderStructuredMetadata();
                scheduleGenerate();
            });

            input.addEventListener("blur", () => {
                if (input.type === "checkbox") {
                    return;
                }

                const trimmed = input.value.trim();
                if (input.value !== trimmed) {
                    input.value = trimmed;
                    state.ParserInput.RegexFields = collectRegexFields();
                    renderRegexTimeOptions();
                    renderLokiLabels();
                    renderStructuredMetadata();
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
    const metadata = collectStructuredMetadata();
    const used = new Set();

    labels.forEach((label, index) => {
        if (index !== exceptIndex && label.Field) {
            used.add(label.Field);
        }
    });

    metadata.forEach((item) => {
        if (item.Field) {
            used.add(item.Field);
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
            renderStructuredMetadata();
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
            removeStructuredMetadataFieldsAlreadyUsedAsLabels();
            renderLokiLabels();
            renderStructuredMetadata();
            scheduleGenerate();
        });

        els.lokiLabels.appendChild(row);
    });
}

function buildLokiFieldOptions(current, index) {
    const fields = getParserFieldNames();

    const used = getUsedLokiFields(index);

    return fields
        .filter((field) => field === current || !used.has(field))
        .map((field) => {
            const selected = field === current ? "selected" : "";
            return `<option value="${escapeAttr(field)}" ${selected}>${escapeHTML(field)}</option>`;
        })
        .join("");
}

function collectStructuredMetadata() {
    if (!els.structuredMetadata) {
        return [];
    }

    return [...els.structuredMetadata.querySelectorAll(".structured-metadata-row")]
        .map((row) => ({
            Field: row.querySelector(".structured-metadata-field").value.trim(),
            Name: row.querySelector(".structured-metadata-name").value.trim()
        }))
        .filter((item) => item.Field !== "" || item.Name !== "");
}

function getUsedStructuredMetadataFields(exceptIndex) {
    const labels = collectLokiLabels();
    const metadata = collectStructuredMetadata();
    const used = new Set();

    labels.forEach((label) => {
        if (label.Field) {
            used.add(label.Field);
        }
    });

    metadata.forEach((item, index) => {
        if (index !== exceptIndex && item.Field) {
            used.add(item.Field);
        }
    });

    return used;
}

function renderStructuredMetadata() {
    clearStructuredMetadataHint();

    if (!els.structuredMetadata) {
        return;
    }

    const metadata = state.LokiStructuredMetadata || [];
    els.structuredMetadata.innerHTML = "";

    metadata.forEach((item, index) => {
        const row = document.createElement("div");
        row.className = "row structured-metadata-row";

        row.innerHTML = `
      <label>
        Parser field
        <select class="structured-metadata-field">${buildStructuredMetadataFieldOptions(item.Field, index)}</select>
      </label>
      <label>
        Metadata name
        <input class="structured-metadata-name" value="${escapeAttr(item.Name)}" placeholder="trace_id" />
      </label>
      <button type="button" class="remove-btn">Remove</button>
    `;

        row.querySelector(".remove-btn").addEventListener("click", () => {
            state.LokiStructuredMetadata.splice(index, 1);
            renderLokiLabels();
            renderStructuredMetadata();
            scheduleGenerate();
        });

        const nameInput = row.querySelector(".structured-metadata-name");
        const fieldSelect = row.querySelector(".structured-metadata-field");

        nameInput.addEventListener("input", () => {
            state.LokiStructuredMetadata = collectStructuredMetadata();
            scheduleGenerate();
        });

        fieldSelect.addEventListener("change", () => {
            state.LokiStructuredMetadata = collectStructuredMetadata();
            renderLokiLabels();
            renderStructuredMetadata();
            scheduleGenerate();
        });

        els.structuredMetadata.appendChild(row);
    });
}

function buildStructuredMetadataFieldOptions(current, index) {
    const fields = getParserFieldNames();

    const used = getUsedStructuredMetadataFields(index);

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
    state.ParserInput.RegexFields.push({ Key: "", Value: "", Optional: false, Pattern: "" });
    renderRegexFields();
    scheduleGenerate();
}

function addLokiLabel() {
    clearLokiLabelHint();

    state.LokiLabels = collectLokiLabels();

    const fields = getParserFieldNames();

    if (fields.length === 0) {
        showLokiLabelHint("Add extracted fields first, then choose which fields should become Loki labels.");
        return;
    }

    const used = new Set([
        ...state.LokiLabels.map((label) => label.Field),
        ...state.LokiStructuredMetadata.map((item) => item.Field)
    ]);
    const firstAvailable = fields.find((key) => !used.has(key));

    if (!firstAvailable) {
        showLokiLabelHint("All extracted fields are already used as Loki labels or structured metadata.");
        return;
    }

    state.LokiLabels.push({
        Field: firstAvailable,
        Name: firstAvailable
    });

    renderLokiLabels();
    renderStructuredMetadata();
    scheduleGenerate();
}

function addStructuredMetadata() {
    clearStructuredMetadataHint();

    state.LokiStructuredMetadata = collectStructuredMetadata();

    const fields = getParserFieldNames();

    if (fields.length === 0) {
        showStructuredMetadataHint("Add extracted fields first, then choose which fields should become structured metadata.");
        return;
    }

    const used = new Set([
        ...state.LokiLabels.map((label) => label.Field),
        ...state.LokiStructuredMetadata.map((item) => item.Field)
    ]);
    const firstAvailable = fields.find((key) => !used.has(key));

    if (!firstAvailable) {
        showStructuredMetadataHint("All extracted fields are already used as Loki labels or structured metadata.");
        return;
    }

    state.LokiStructuredMetadata.push({
        Field: firstAvailable,
        Name: firstAvailable
    });

    renderLokiLabels();
    renderStructuredMetadata();
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

function showStructuredMetadataHint(message) {
    if (!els.structuredMetadataHint) {
        return;
    }

    els.structuredMetadataHint.textContent = message;
    els.structuredMetadataHint.classList.remove("hidden");
}

function clearStructuredMetadataHint() {
    if (!els.structuredMetadataHint) {
        return;
    }

    els.structuredMetadataHint.textContent = "";
    els.structuredMetadataHint.classList.add("hidden");
}

function findDuplicatedLokiNames(labels, metadata) {
    const labelNames = new Set(
        labels
            .map((label) => label.Name.trim())
            .filter(Boolean)
    );

    const duplicated = [];
    const seen = new Set();

    metadata.forEach((item) => {
        const name = item.Name.trim();

        if (!name || !labelNames.has(name) || seen.has(name)) {
            return;
        }

        duplicated.push(name);
        seen.add(name);
    });

    return duplicated;
}

function getParserFieldNames() {
    ensureParserInputs();

    const names = [];
    const seen = new Set();

    state.ParserInputs.forEach((parserInput, index) => {
        const fields = index === activeParserInputIndex ? collectRegexFields() : (parserInput.RegexFields || []);

        fields.forEach((field) => {
            const name = field.Key.trim();
            if (!name || seen.has(name)) {
                return;
            }

            names.push(name);
            seen.add(name);
        });
    });

    return names;
}

function getUsedLokiFieldsFromState() {
    return new Set(
        (state.LokiLabels || [])
            .map((label) => label.Field)
            .filter(Boolean)
    );
}

function getUsedStructuredMetadataFieldsFromState() {
    return new Set(
        (state.LokiStructuredMetadata || [])
            .map((item) => item.Field)
            .filter(Boolean)
    );
}

function removeStructuredMetadataFieldsAlreadyUsedAsLabels() {
    const labelFields = getUsedLokiFieldsFromState();

    state.LokiStructuredMetadata = (state.LokiStructuredMetadata || [])
        .filter((item) => !labelFields.has(item.Field));
}

function switchView(view) {
    activeView = view;

    const isGenerate = view === "generate";

    setVisible(els.generateForm, isGenerate);
    setVisible(els.doctorForm, !isGenerate);

    els.generateViewBtn.classList.toggle("active", isGenerate);
    els.doctorViewBtn.classList.toggle("active", !isGenerate);

    setVisible(els.copyYamlBtn, isGenerate);
    setVisible(els.downloadYamlBtn, isGenerate);

    if (els.previewTitle) {
        els.previewTitle.textContent = isGenerate ? "YAML preview" : "Diagnosis";
    }

    if (isGenerate) {
        setVisible(els.yamlPreview, true);
        setVisible(els.doctorResult, false);
        generateYAML();
        return;
    }

    latestYaml = "";
    renderWarnings([]);
    setVisible(els.yamlPreview, false);
    setVisible(els.doctorResult, true);
    renderDoctorResult(null);
    setStatus("", "waiting");
    scheduleDoctor();
}

function showDoctorHint(message) {
    if (!els.doctorHint) {
        return;
    }

    els.doctorHint.textContent = message;
    els.doctorHint.classList.remove("hidden");
}

function clearDoctorHint() {
    if (!els.doctorHint) {
        return;
    }

    els.doctorHint.textContent = "";
    els.doctorHint.classList.add("hidden");
}

function scheduleDoctor() {
    if (activeView !== "doctor") {
        return;
    }

    clearTimeout(doctorDebounceTimer);
    doctorDebounceTimer = setTimeout(runDoctor, 350);
}

function getDoctorBaselineMode() {
    if (els.doctorBaseline && els.doctorBaseline.value) {
        return els.doctorBaseline.value;
    }

    if (els.mode && els.mode.value) {
        return els.mode.value;
    }

    return "production";
}

async function runDoctor() {
    clearDoctorHint();
    renderWarnings([]);

    const yaml = els.doctorYamlInput.value.trim();
    if (!yaml) {
        latestYaml = "";
        setStatus("", "waiting");
        renderDoctorResult(null);
        return;
    }

    const mode = getDoctorBaselineMode();
    const requestID = ++doctorRequestSeq;
    setStatus("", "checking");
    renderDoctorResult({ loading: true, mode });

    try {
        const response = await fetch("/api/doctor", {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: JSON.stringify({ yaml, mode, baseline: mode })
        });

        const data = await response.json();

        if (requestID !== doctorRequestSeq) {
            return;
        }

        if (!response.ok) {
            setStatus("error", "Error");
            latestYaml = "";
            renderDoctorResult({ error: data.error || "failed to check YAML" });
            return;
        }

        latestYaml = formatDoctorReport(data);
        renderDoctorResult(data);

        if ((data.error_count || 0) > 0) {
            setStatus("error", "Error");
        } else {
            setStatus("ok", "Ready");
        }
    } catch (error) {
        if (requestID !== doctorRequestSeq) {
            return;
        }

        setStatus("error", "Error");
        latestYaml = "";
        renderDoctorResult({ error: error.message });
    }
}

function renderDoctorResult(report) {
    if (!els.doctorResult) {
        return;
    }

    if (!report) {
        els.doctorResult.innerHTML = `<div class="doctor-empty">Paste a Fluent Bit YAML config to run diagnosis automatically.</div>`;
        return;
    }

    if (report.loading) {
        els.doctorResult.innerHTML = `<div class="doctor-empty">Checking config...</div>`;
        return;
    }

    if (report.error) {
        els.doctorResult.innerHTML = `<div class="doctor-error">${escapeHTML(report.error)}</div>`;
        return;
    }

    const checks = report.checks || [];
    if (checks.length === 0) {
        els.doctorResult.innerHTML = `<div class="doctor-empty">No checks returned.</div>`;
        return;
    }

    const groups = groupDoctorChecks(checks);
    const baselineLabel = formatDoctorBaseline(report.mode);

    els.doctorResult.innerHTML = `
        <div class="doctor-baseline-note">Baseline: ${escapeHTML(baselineLabel)}</div>
        <div class="doctor-groups">
            ${groups.map((group) => `
                <section class="doctor-group">
                    <h3>${escapeHTML(group.section)}</h3>
                    <div class="doctor-checks">
                        ${group.checks.map(renderDoctorCheck).join("")}
                    </div>
                </section>
            `).join("")}
        </div>
        <div class="doctor-final-summary">
            Summary: ${Number(report.ok_count || 0)} OK, ${Number(report.warning_count || 0)} WARN, ${Number(report.error_count || 0)} ERROR
        </div>
    `;
}

function groupDoctorChecks(checks) {
    const groups = [];
    const groupMap = new Map();

    checks.forEach((check) => {
        const section = check.section || "General";

        if (!groupMap.has(section)) {
            const group = {
                section,
                checks: []
            };
            groupMap.set(section, group);
            groups.push(group);
        }

        groupMap.get(section).checks.push(check);
    });

    return groups;
}

function renderDoctorCheck(check) {
    const level = String(check.level || "INFO").toLowerCase();
    const displayLevel = String(check.level || "INFO").toUpperCase();
    const field = String(check.field || "").trim();
    const message = buildDoctorDisplayMessage(check);
    const details = buildDoctorDetails(check);

    return `
        <div class="doctor-check doctor-${escapeAttr(level)}">
            <span class="doctor-level">${escapeHTML(displayLevel)}</span>
            <div class="doctor-message">
                ${field ? `<span class="doctor-field">${escapeHTML(field)}</span>` : ""}
                <span class="doctor-text">${escapeHTML(message)}</span>
                ${details}
            </div>
        </div>
    `;
}

function buildDoctorDisplayMessage(check) {
    const level = String(check.level || "INFO").toUpperCase();
    const field = String(check.field || "").trim();
    const message = String(check.message || "").trim();
    const actual = String(check.actual || "").trim();

    if (level === "OK" && actual) {
        return actual;
    }

    if (!field || !message) {
        return message;
    }

    return stripRepeatedFieldName(message, field);
}

function stripRepeatedFieldName(message, field) {
    if (message === field) {
        return "configured";
    }

    const prefixes = [
        `${field} `,
        `${field}: `,
    ];

    for (const prefix of prefixes) {
        if (message.startsWith(prefix)) {
            return message.slice(prefix.length).trim();
        }
    }

    return message;
}

function buildDoctorDetails(check) {
    const level = String(check.level || "INFO").toUpperCase();

    if (level === "OK") {
        return "";
    }

    const details = [];

    if (check.actual) {
        details.push(`<span class="doctor-detail">actual=${escapeHTML(check.actual)}</span>`);
    }

    if (check.expect) {
        details.push(`<span class="doctor-detail">expected=${escapeHTML(check.expect)}</span>`);
    }

    return details.join(" ");
}

function formatDoctorBaseline(mode) {
    if (mode === "quick-test") {
        return "quick test";
    }

    return "production";
}

function formatDoctorReport(report) {
    const checks = report.checks || [];

    if (checks.length === 0) {
        return "No checks returned.";
    }

    const lines = [];
    let currentSection = "";

    checks.forEach((check) => {
        if (check.section !== currentSection) {
            if (lines.length > 0) {
                lines.push("");
            }

            currentSection = check.section || "General";
            lines.push(`== ${currentSection} ==`);
        }

        const level = check.level || "INFO";
        const field = check.field ? `${check.field}: ` : "";
        const message = buildDoctorDisplayMessage(check);
        const details = level === "OK" ? "" : [
            check.actual ? `actual=${check.actual}` : "",
            check.expect ? `expected=${check.expect}` : ""
        ].filter(Boolean).join(" ");

        lines.push(`[${level}] ${field}${message}${details ? ` ${details}` : ""}`);
    });

    lines.push("");
    lines.push(`Summary: ${Number(report.ok_count || 0)} OK, ${Number(report.warning_count || 0)} WARN, ${Number(report.error_count || 0)} ERROR`);

    return lines.join("\n");
}

function scheduleGenerate() {
    if (activeView !== "generate") {
        return;
    }

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
    warnings = uniqueWarnings(warnings || []);

    if (warnings.length === 0) {
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

function uniqueWarnings(warnings) {
    const unique = [];
    const seen = new Set();

    warnings.forEach((warning) => {
        const normalized = String(warning || "").trim();
        if (!normalized || seen.has(normalized)) {
            return;
        }

        seen.add(normalized);
        unique.push(normalized);
    });

    return unique;
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
        showCopyFeedback(activeView === "generate" ? "No YAML" : "No result", false);
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
    if (activeView === "generate") {
        link.download = `fluent-bit-${formatDownloadTimestamp(new Date())}.yaml`;
    } else {
        link.download = `fluent-bit-doctor-${formatDownloadTimestamp(new Date())}.txt`;
    }
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
        const max = Math.max(min, rect.width - 280);
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
            renderParserFormats();
            scheduleGenerate();
        });

        el.addEventListener("change", () => {
            syncStateFromDOM();
            renderVisibility();
            renderParserFormats();
            scheduleGenerate();
        });
    });

    els.generateViewBtn.addEventListener("click", () => switchView("generate"));
    els.doctorViewBtn.addEventListener("click", () => switchView("doctor"));

    els.doctorYamlInput.addEventListener("input", scheduleDoctor);

    if (els.doctorBaseline) {
        els.doctorBaseline.addEventListener("change", scheduleDoctor);
    }

    els.addRegexFieldBtn.addEventListener("click", addRegexField);
    els.addParserFormatBtn.addEventListener("click", addParserFormat);
    els.removeParserFormatBtn.addEventListener("click", removeParserFormat);
    els.addLokiLabelBtn.addEventListener("click", addLokiLabel);
    els.addStructuredMetadataBtn.addEventListener("click", addStructuredMetadata);
    els.copyYamlBtn.addEventListener("click", copyYAML);
    els.downloadYamlBtn.addEventListener("click", downloadYAML);

    bindResizablePanels();
}

function initDefaults() {
    els.mode.value = "quick-test";
    els.output.value = "stdout";
    els.timezone.value = "GMT";
    els.parserFormat.value = "json";
    state.ParserInputs = [cloneParserInput(state.ParserInput)];
    activeParserInputIndex = 0;

    if (els.enableLokiLabels) {
        els.enableLokiLabels.checked = false;
    }

    if (els.enableStructuredMetadata) {
        els.enableStructuredMetadata.checked = false;
    }

    if (els.lokiLineFormat) {
        els.lokiLineFormat.value = "json";
    }

    renderVisibility();
    renderParserFormats();
    renderRegexFields();
    renderLokiLabels();
    renderStructuredMetadata();
    switchView("generate");
}

bindEvents();
initDefaults();
