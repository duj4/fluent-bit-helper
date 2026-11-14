(() => {
    const $ = id => document.getElementById(id);
    const labels = { requires_change: "Needs change", review: "Manual check", no_impact: "No matching impact" };
    let report = null, sequence = 0, controller = null;
    function node(tag, className, text) {
        const element = document.createElement(tag);
        if (className) element.className = className;
        if (text !== undefined) element.textContent = String(text);
        return element;
    }
    function status(value) {
        const badge = node("span", "result-status", labels[value] || value);
        badge.dataset.status = value;
        return badge;
    }
    const changeLabels = {
        metric_rename: "Metric rename", unit_change: "Unit change", type_change: "Type change",
        metric_removed: "Metric removal", metric_split: "Metric split", label_rename: "Label rename",
        conditional_label_rename: "Label rename", label_removed: "Removed label",
        label_added: "Added label", label_value_change: "Label value change"
    };
    const units = { megahertz: "MHz", hertz: "Hz", seconds: "s", milliseconds: "ms", microseconds: "µs", nanoseconds: "ns", bytes: "bytes", percent: "%" };
    function renderFinding(finding) {
        const block = node("article", "metric-finding");
        const title = node("div", "metric-title");
        title.append(node("code", "", finding.metric));
        block.append(title);
        const changes = node("ul", "change-list");
        finding.changes.forEach(change => {
            const item = node("li");
            item.append(node("strong", "", changeLabels[change.kind] || change.kind.replaceAll("_", " ")), status(change.status));
            if (change.label) item.append(node("code", "change-mapping", `Label: ${change.label}`));
            const from = change.kind === "unit_change" ? units[change.from] || change.from : change.from;
            const to = change.kind === "unit_change" ? units[change.to] || change.to : change.kind === "metric_split" ? finding.new_metrics?.join(", ") : change.to;
            if (from || to) {
                const factor = change.factor != null ? ` (×${Number(change.factor).toLocaleString("en-US", { maximumSignificantDigits: 15 })})` : "";
                item.append(node("code", "change-mapping", `${from || ""}${from && to ? " → " : ""}${to || ""}${factor}`));
            }
            if (change.conditions) {
                const conditions = Object.entries(change.conditions).filter(([key]) => key !== "on_false" && key !== "on_unknown");
                item.append(node("div", "change-description", `Applies only when ${conditions.map(([key, value]) => `${key}=${key === "honor_labels" ? value : JSON.stringify(value)}`).join("; ")}`));
                const { on_false, on_unknown } = change.conditions;
                if (on_false || on_unknown) item.append(node("div", "change-description", `on_false=${on_false}; on_unknown=${on_unknown}`));
            }
            if (["metric_removed", "metric_split"].includes(change.kind) && finding.replacement_query) {
                item.append(node("div", "change-description", "Suggested replacement:"), node("pre", "query-code", finding.replacement_query));
            }
            changes.append(item);
        });
        block.append(changes);
        if (finding.notes?.length) {
            const notes = node("ul", "change-description");
            finding.notes.forEach(note => notes.append(node("li", "", note)));
            block.append(notes);
        }
        return block;
    }
    function renderCoverage(notices) {
        const list = node("ul", "issue-list");
        notices.forEach(notice => {
            const item = node("li", "", `${notice.message} (${notice.count} ${notice.count === 1 ? "event" : "events"})`);
            if (notice.uncertain) item.append(node("span", "result-status", "Coverage warning"));
            list.append(item);
        });
        return list;
    }
    function renderResource(container, resource, kind) {
        // Hide resources with no matching rules or input validation warnings.
        const warnings = resource.coverage.filter(notice => notice.uncertain);
        if (!resource.findings.length && !warnings.length) return;
        if (!container.childElementCount) container.append(node("h2", "", kind === "Dashboard" ? "Dashboards" : "Alert rules"));
        const block = node("section", "card resource-block");
        block.append(node("h3", "", resource.title));
        if (warnings.length) block.append(renderCoverage(warnings));
        if (!resource.findings.length) block.append(node("p", "resource-meta", "No matching migration rules for the extracted metrics. The coverage warning still requires attention."));
        resource.findings.forEach(finding => block.append(renderFinding(finding)));
        container.append(block);
    }
    function render(data) {
        report = data; $("analysisResults").hidden = false;
        const summary = $("analysisSummary"); summary.replaceChildren();
        [["Dashboards", data.stats.dashboards], ["Alert rules", data.stats.alert_rules], ["Unique metric names", data.stats.unique_metrics],
            ["Affected dashboards", data.stats.affected_dashboards], ["Affected alerts", data.stats.affected_alerts], ["Matched metric names", data.stats.matched_metrics],
            ["Needs change", data.stats.requires_change], ["Manual checks", data.stats.review]].forEach(([label, value]) => {
            const card = node("div", "summary-stat"); card.append(node("strong", "", value), node("span", "", label)); summary.append(card);
        });
        const issues = node("ul", "issue-list");
        data.issues.forEach(issue => issues.append(node("li", "", `${issue.input}${issue.resource ? ` / ${issue.resource}` : ""}: ${issue.message}`)));
        $("analysisIssues").replaceChildren(issues);
        const dashboards = $("dashboardResults"), alerts = $("alertResults"); dashboards.replaceChildren(); alerts.replaceChildren();
        data.dashboards.forEach(resource => renderResource(dashboards, resource, "Dashboard"));
        data.alerts.forEach(resource => renderResource(alerts, resource, "Alert"));
    }
    function invalidate() {
        sequence++; controller?.abort(); report = null; $("analysisResults").hidden = true; $("analyzeBtn").disabled = false;
        $("analysisStatus").textContent = "Files changed. Analyze to check the current selection.";
    }
    $("dashboardsFile").addEventListener("change", invalidate); $("alertsFile").addEventListener("change", invalidate);
    $("analyzeForm").addEventListener("submit", async event => {
        event.preventDefault(); const body = new FormData(); let count = 0;
        for (const [id, name] of [["dashboardsFile", "dashboards"], ["alertsFile", "alert_rules"]]) {
            const file = $(id).files[0]; if (!file) continue;
            if (!file.name.toLowerCase().endsWith(".json") || file.size === 0 || file.size > 8 * 1024 * 1024) { $("analysisStatus").textContent = "Select non-empty .json files, at most 8 MiB each."; return; }
            body.append(name, file); count++;
        }
        if (!count) { $("analysisStatus").textContent = "Choose at least one JSON file."; return; }
        const current = ++sequence; controller?.abort(); controller = new AbortController(); $("analyzeBtn").disabled = true;
        $("analysisStatus").textContent = "Analyzing exports…"; $("analysisResults").hidden = true;
        try {
            const response = await fetch("/api/metrics-analyzer/analyze", { method: "POST", body, signal: controller.signal });
            const data = await response.json(); if (current !== sequence) return;
            if (!response.ok) throw new Error(data.error || `Analysis request failed (${response.status}).`);
            render(data); $("analysisStatus").textContent = data.complete ? "" : "Partial inventory — check input issues.";
        } catch (error) { if (current === sequence && error.name !== "AbortError") $("analysisStatus").textContent = error.message || "Could not complete analysis."; }
        finally { if (current === sequence) $("analyzeBtn").disabled = false; }
    });
    $("exportResultsBtn").addEventListener("click", () => {
        if (!report) return; const url = URL.createObjectURL(new Blob([JSON.stringify(report, null, 2)], { type: "application/json" }));
        const link = document.createElement("a"); link.href = url; link.download = "windows-exporter-upgrade-analysis.json"; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
    });
})();
