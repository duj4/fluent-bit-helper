(() => {
    const $ = id => document.getElementById(id);
    const labels = { requires_change: "Needs change", review: "Needs review", no_impact: "No impact found" };
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
    function issues(messages) {
        const list = node("ul", "issue-list");
        messages.forEach(message => {
            const item = node("li", "", message);
            item.append(status("review")); list.append(item);
        });
        return list;
    }
    // Hide only a successful, definite no-impact result. Keep the report itself
    // intact so downloads and scan totals include all inspected resources.
    function needsAttention(query) {
        return query.status !== "no_impact" || query.complete !== true || query.issues?.length > 0 || query.findings?.length > 0;
    }
    const changeLabels = {
        metric_rename: "Metric rename", unit_change: "Unit change", type_change: "Type change",
        metric_removed: "Removed metric", metric_split: "Split metric", label_rename: "Label rename",
        conditional_label_rename: "Conditional label rename", label_removed: "Removed label",
        label_added: "Added label", label_value_change: "Label value change", alert_threshold: "Alert threshold"
    };
    const units = { megahertz: "MHz", hertz: "Hz", seconds: "s", milliseconds: "ms", microseconds: "µs", nanoseconds: "ns", bytes: "bytes", percent: "%" };
    function renderFinding(finding) {
        const block = node("article", "metric-finding");
        const title = node("div", "metric-title");
        title.append(node("code", "", finding.metric)); block.append(title);
        const changes = node("ul", "change-list");
        finding.changes.forEach(change => {
            const item = node("li");
            item.append(node("strong", "", changeLabels[change.kind] || change.kind.replaceAll("_", " ")));
            if (change.status === "review") item.append(status("review"));
            const from = change.kind === "unit_change" ? units[change.from] || change.from : change.from;
            const to = change.kind === "unit_change" ? units[change.to] || change.to : change.kind === "metric_split" ? finding.new_metrics?.join(", ") : change.to;
            if (from || to) {
                const factor = change.factor != null ? ` (×${Number(change.factor).toLocaleString("en-US", { maximumSignificantDigits: 15 })})` : "";
                item.append(node("code", "change-mapping", `${from || ""}${from && to ? " → " : ""}${to || ""}${factor}`));
            }
            if (change.description) {
                const description = change.kind === "alert_threshold" ? "Check the alert threshold after the unit change." : change.description;
                const attributed = /^(Rule detail:|Analyzer advice:)/.test(description) ? description : `Analyzer advice: ${description}`;
                item.append(node("div", "change-description", attributed));
            }
            if (["metric_removed", "metric_split"].includes(change.kind) && finding.replacement_query) {
                item.append(node("div", "change-description", "Replacement query (review the caveat above):"), node("pre", "query-code", finding.replacement_query));
            }
            changes.append(item);
        });
        block.append(changes);
        return block;
    }
    function renderQuery(query) {
        const details = node("details", "query-result"); details.open = true;
        const summary = node("summary", "", `Query ${query.ref_id || "(no RefID)"}`);
        const changes = query.findings.flatMap(finding => finding.changes);
        const needsChange = query.status === "requires_change" || query.findings.some(finding => finding.status === "requires_change") || changes.some(change => change.status === "requires_change");
        const needsReview = query.status === "review" || !query.complete || query.issues?.length > 0 || changes.some(change => change.status === "review");
        if (needsChange) summary.append(status("requires_change"));
        if (needsReview) summary.append(status("review"));
        if (!needsChange && !needsReview) summary.append(status(query.status));
        if (!query.complete) summary.append(node("span", "result-status", "Incomplete"));
        details.append(summary);
        const source = node("details", "query-source"); source.append(node("summary", "", "Original PromQL"), node("pre", "query-code", query.expr)); details.append(source);
        if (query.issues?.length) {
            const coverage = node("div", "query-issues");
            coverage.append(node("strong", "", "Query analysis"), status("review"));
            const list = node("ul", "issue-list");
            query.issues.forEach(message => list.append(node("li", "", message)));
            coverage.append(list); details.append(coverage);
        }
        query.findings.forEach(finding => details.append(renderFinding(finding)));
        return details;
    }
    function render(data) {
        report = data; $("analysisResults").hidden = false;
        const summary = $("analysisSummary"); summary.replaceChildren();
        [["Dashboards", data.stats.dashboards], ["Panels", data.stats.panels], ["Queries", data.stats.queries], ["Alert rules", data.stats.alert_rules], ["Needs change", data.stats.requires_change], ["Needs review", data.stats.review]].forEach(([label, value]) => {
            const card = node("div", "summary-stat"); card.append(node("strong", "", value), node("span", "", label)); summary.append(card);
        });
        const banner = $("analysisBanner"); banner.replaceChildren(); banner.classList.toggle("partial", !data.complete);
        banner.append(node("strong", "", !data.complete ? "Partial analysis — check the issues below" : data.status === "no_impact" ? "No impact found" : "Analysis complete"));
        const description = !data.complete ? "Some inputs or queries could not be fully checked. Findings apply only to the checked scope; this is not a complete no-impact result." : data.status === "no_impact" ? "No applicable changes found in the successfully checked exports." : "Review the affected metrics below. These are read-only suggestions; no uploaded resource has been changed.";
        banner.append(node("p", "", description));
        banner.append(node("small", "", `${data.stats.queries} PromQL queries checked · ${data.stats.skipped_queries} non-PromQL/data-source queries skipped. Needs change counts metric findings. Needs review counts metric findings with review items, incomplete queries, and input/resource issues. Multiple review items in one metric finding count once; a query can contribute to both totals.`));
        $("analysisIssues").replaceChildren(issues(data.issues.map(issue => `${issue.input}${issue.resource ? ` / ${issue.resource}` : ""}: ${issue.message}`)));
        const dashboards = $("dashboardResults"), alerts = $("alertResults"); dashboards.replaceChildren(); alerts.replaceChildren();
        data.dashboards.forEach(dashboard => {
            const resourceIssues = data.issues.filter(issue => issue.input === "dashboards" && issue.resource === dashboard.uid);
            const panels = dashboard.panels.filter(panel => panel.queries.some(needsAttention) || resourceIssues.some(issue => issue.message.startsWith(`Panel ${panel.id} `)));
            // Some unsupported targets have an Issue but no QueryResult/PanelResult.
            // Keep their known panel location visible using the existing issue text.
            resourceIssues.forEach(issue => {
                const match = /^Panel (-?\d+)\b/.exec(issue.message);
                if (match && !panels.some(panel => panel.id === Number(match[1]))) panels.push({ id: Number(match[1]), title: "Analysis issue", queries: [] });
            });
            if (!panels.length && !resourceIssues.length) return;
            if (!dashboards.childElementCount) dashboards.append(node("h2", "", "Dashboards"));
            const resource = node("section", "card resource-block"); resource.append(node("h3", "", dashboard.title), node("div", "resource-meta", `Dashboard UID: ${dashboard.uid}`));
            if (resourceIssues.length) resource.append(node("p", "resource-meta", "Unresolved analysis issues are listed above."));
            panels.forEach(panel => {
                const block = node("div", "panel-result"); block.append(node("h4", "", `${panel.title || "Untitled panel"} · Panel ${panel.id}`));
                panel.queries.filter(needsAttention).forEach(query => block.append(renderQuery(query))); resource.append(block);
            }); dashboards.append(resource);
        });
        data.alerts.forEach(alert => {
            const queries = alert.queries.filter(needsAttention);
            if (!queries.length && !data.issues.some(issue => issue.input === "alert_rules" && issue.resource === alert.uid)) return;
            if (!alerts.childElementCount) alerts.append(node("h2", "", "Alert rules"));
            const resource = node("section", "card resource-block"); resource.append(node("h3", "", alert.title), node("div", "resource-meta", `Alert UID: ${alert.uid} · Folder: ${alert.folder_uid || "—"} · Group: ${alert.rule_group || "—"} · Condition: ${alert.condition || "—"}`));
            queries.forEach(query => resource.append(renderQuery(query)));
            alerts.append(resource);
        });
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
            render(data); $("analysisStatus").textContent = data.complete ? "Analysis finished." : "Analysis finished with incomplete inputs. Check the reported issues.";
        } catch (error) { if (current === sequence && error.name !== "AbortError") $("analysisStatus").textContent = error.message || "Could not complete analysis."; }
        finally { if (current === sequence) $("analyzeBtn").disabled = false; }
    });
    $("exportResultsBtn").addEventListener("click", () => {
        if (!report) return; const url = URL.createObjectURL(new Blob([JSON.stringify(report, null, 2)], { type: "application/json" }));
        const link = document.createElement("a"); link.href = url; link.download = "windows-exporter-upgrade-analysis.json"; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
    });
})();
