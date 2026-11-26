(() => {
    const $ = id => document.getElementById(id);
    const pageSize = 25;
    const categories = [
        { id: "all", label: "All rules", matches: () => true },
        { id: "rename", label: "Metric renames", matches: rule => Boolean(rule.new_metric) },
        { id: "removed", label: "Removals & splits", matches: rule => Boolean(rule.removed || rule.new_metrics?.length) },
        { id: "labels", label: "Labels", matches: rule => Boolean(rule.label_renames || rule.conditional_label_renames?.length || rule.added_labels?.length || rule.removed_labels?.length || rule.label_value_changes?.length) },
        { id: "unit", label: "Units", matches: rule => Boolean(rule.unit_change) },
        { id: "type", label: "Types", matches: rule => Boolean(rule.type_change) }
    ];
    let rules = [], activeCategory = "all", page = 0;

    function node(tag, className, text) {
        const element = document.createElement(tag);
        if (className) element.className = className;
        if (text !== undefined) element.textContent = String(text);
        return element;
    }
    function detail(container, label, value, className = "") {
        const line = node("p", `rules-detail ${className}`);
        line.append(node("strong", "", `${label} `), node("code", "", value));
        container.append(line);
    }
    function mapping(change) { return `${change.from} → ${change.to}`; }
    function renderRule(rule) {
        const row = node("tr");
        const metric = node("td"), changes = node("td"), advice = node("td");
        metric.append(node("code", "rules-metric", rule.metric));
        if (rule.new_metric || rule.new_metrics?.length) {
            metric.append(node("span", "rules-mapping-arrow", "↓"));
            (rule.new_metrics || [rule.new_metric]).forEach(name => metric.append(node("code", "rules-metric rules-target", name)));
        }
        if (rule.metric_aliases?.length) {
            const aliases = node("details", "rules-aliases");
            aliases.append(node("summary", "", `Also matches ${rule.metric_aliases.length} ${rule.metric_aliases.length === 1 ? "alias" : "aliases"}`));
            rule.metric_aliases.forEach(alias => aliases.append(node("code", "", alias)));
            metric.append(aliases);
        }
        if (rule.metric.endsWith("*")) detail(metric, "*", "Metric family");
        if (rule.exclude_metrics?.length) detail(metric, "Except:", rule.exclude_metrics.join(", "));

        const tags = node("div", "rules-tags");
        categories.slice(1).filter(category => category.matches(rule)).forEach(category => {
            const text = category.id === "removed" ? (rule.removed ? "Removed" : "Split") : { rename: "Metric rename", labels: "Label change", unit: "Unit change", type: "Type change" }[category.id];
            const tag = node("span", "rules-tag", text); tag.dataset.kind = category.id; tags.append(tag);
        });
        changes.append(tags);
        if (rule.label_matchers) detail(changes, "Applies only to series with:", Object.entries(rule.label_matchers).map(([key, value]) => `${key}=${JSON.stringify(value)}`).join(", "), "rules-condition");
        if (rule.unit_change) {
            detail(changes, "Unit:", mapping(rule.unit_change));
            if (rule.unit_change.factor != null) detail(changes, "Multiply by:", String(rule.unit_change.factor));
        }
        if (rule.type_change) detail(changes, "Type:", mapping(rule.type_change));
        Object.entries(rule.label_renames || {}).forEach(([from, to]) => detail(changes, "Label:", `${from} → ${to}`));
        (rule.conditional_label_renames || []).forEach(change => {
            detail(changes, "Conditional label:", mapping(change));
            detail(changes, "Only when:", Object.entries(change.when).map(([key, value]) => `${key}=${JSON.stringify(value)}`).join("; "), "rules-condition");
            const outcomes = { preserve: "Keep the original label", review: "Check manually" };
            detail(changes, "Otherwise:", outcomes[change.on_false] || change.on_false);
            detail(changes, "If unknown:", outcomes[change.on_unknown] || change.on_unknown);
        });
        (rule.added_labels || []).forEach(label => detail(changes, "Added label:", label));
        (rule.removed_labels || []).forEach(label => detail(changes, "Removed label:", label));
        (rule.label_value_changes || []).forEach(change => detail(changes, `Label value (${change.label}):`, mapping(change)));
        if (rule.removed) detail(changes, "Availability:", "Removed in the target version");

        advice.append(node("p", "rules-advice", rule.note || "Review the listed changes wherever this metric is used."));
        if (rule.replacement_query) {
            advice.append(node("span", "rules-replacement-label", "Suggested replacement query"), node("pre", "query-code", rule.replacement_query));
        }
        row.append(metric, changes, advice);
        return row;
    }
    function render() {
        const terms = $("ruleSearch").value.trim().toLowerCase().split(/\s+/).filter(Boolean);
        const searched = rules.filter(entry => terms.every(term => entry.search.includes(term)));
        const category = categories.find(item => item.id === activeCategory);
        const filtered = searched.filter(entry => category.matches(entry.rule));
        const pages = Math.max(1, Math.ceil(filtered.length / pageSize));
        page = Math.min(page, pages - 1);
        const start = page * pageSize, end = Math.min(start + pageSize, filtered.length);
        const filters = document.createDocumentFragment();
        categories.forEach(item => {
            const button = node("button", "rules-filter", item.label);
            button.type = "button"; button.dataset.category = item.id;
            button.setAttribute("aria-pressed", String(item.id === activeCategory));
            button.append(node("span", "", searched.filter(entry => item.matches(entry.rule)).length));
            button.addEventListener("click", () => { activeCategory = item.id; page = 0; render(); $("ruleFilters").querySelector(`[data-category="${item.id}"]`).focus(); });
            filters.append(button);
        });
        $("ruleFilters").replaceChildren(filters);
        $("ruleRows").replaceChildren(...filtered.slice(start, end).map(entry => renderRule(entry.rule)));
        const ruleLabel = filtered.length === 1 ? "rule" : "rules";
        $("rulesStatus").textContent = filtered.length ? `Showing ${start + 1}–${end} of ${filtered.length} matching ${ruleLabel} · ${rules.length} total` : `No matching rules · ${rules.length} total`;
        $("rulesTableWrap").hidden = filtered.length === 0;
        $("rulesEmpty").hidden = filtered.length !== 0;
        $("rulesPagination").hidden = filtered.length <= pageSize;
        $("rulesPageLabel").textContent = `Page ${page + 1} of ${pages} · ${pageSize} rules per page`;
        $("previousRules").disabled = page === 0; $("nextRules").disabled = page >= pages - 1;
        $("clearRules").disabled = terms.length === 0 && activeCategory === "all";
    }
    async function load() {
        $("rulesWorkspace").setAttribute("aria-busy", "true");
        $("rulesError").hidden = true; $("retryRules").disabled = true;
        $("rulesStatus").textContent = "Loading migration rules…";
        try {
            const response = await fetch("/api/metrics-analyzer/rules", { cache: "no-cache" });
            if (!response.ok) throw new Error("Rules request failed");
            const data = await response.json();
            rules = data.rules.map(rule => ({ rule, search: JSON.stringify(rule).toLowerCase() }));
            $("rulesVersion").textContent = `${data.source} → ${data.target}`;
            $("officialGuide").href = data.reference;
            $("ruleSearch").disabled = false;
            render();
        } catch (_) {
            $("rulesStatus").textContent = "Rules unavailable. Please try again.";
            $("rulesVersion").textContent = "Version unavailable";
            $("rulesError").hidden = false;
        } finally {
            $("rulesWorkspace").setAttribute("aria-busy", "false"); $("retryRules").disabled = false;
        }
    }
    $("ruleSearch").addEventListener("input", () => { page = 0; render(); });
    $("clearRules").addEventListener("click", () => { $("ruleSearch").value = ""; activeCategory = "all"; page = 0; render(); $("ruleSearch").focus(); });
    function turnPage(delta) {
        page += delta; render();
        $("rulesWorkspace").scrollIntoView({ block: "start" });
        $("ruleSearch").focus({ preventScroll: true });
    }
    $("previousRules").addEventListener("click", () => turnPage(-1));
    $("nextRules").addEventListener("click", () => turnPage(1));
    $("retryRules").addEventListener("click", load);
    load();
})();
