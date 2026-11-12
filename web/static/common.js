(() => {
    const key = "flbctl.theme";
    function apply(theme) {
        theme = theme === "light" ? "light" : "dark";
        document.documentElement.dataset.theme = theme;
        try { localStorage.setItem(key, theme); } catch (_) { /* Storage may be disabled. */ }
        const button = document.getElementById("themeToggleBtn");
        if (button) { button.textContent = theme === "dark" ? "Light mode" : "Dark mode"; button.setAttribute("aria-label", `Switch to ${theme === "dark" ? "light" : "dark"} mode`); }
    }
    let saved = "dark";
    try { saved = localStorage.getItem(key) || "dark"; } catch (_) { /* Use default. */ }
    apply(saved);
    document.addEventListener("DOMContentLoaded", () => {
        apply(document.documentElement.dataset.theme);
        document.getElementById("themeToggleBtn")?.addEventListener("click", () => apply(document.documentElement.dataset.theme === "dark" ? "light" : "dark"));
    });
})();
