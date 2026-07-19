async function postJSON(url, body) {
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `request failed (${res.status})`);
  return data;
}

function showMessage(el, text, kind) {
  el.textContent = text;
  el.className = `form-message ${kind}-text`;
  el.hidden = false;
}

function setLoading(form, loading) {
  const btn = form.querySelector(".auth-submit");
  btn.disabled = loading;
  btn.classList.toggle("is-loading", loading);
}

// --- Tab switching between "Sign in" and "Create account" ---

function initTabs() {
  const tabs = Array.from(document.querySelectorAll(".auth-tab"));
  const indicator = document.querySelector(".auth-tab-indicator");
  const panels = Array.from(document.querySelectorAll(".auth-form"));

  function moveIndicator(tab) {
    indicator.style.width = `${tab.offsetWidth}px`;
    indicator.style.transform = `translateX(${tab.offsetLeft}px)`;
  }

  function activate(name) {
    for (const tab of tabs) {
      const isActive = tab.dataset.tab === name;
      tab.classList.toggle("is-active", isActive);
      tab.setAttribute("aria-selected", String(isActive));
      if (isActive) moveIndicator(tab);
    }
    for (const panel of panels) {
      const isActive = panel.dataset.panel === name;
      panel.hidden = !isActive;
      // The stylesheet's ".auth-form { display: flex }" has the same
      // specificity as the browser's default "[hidden] { display: none }"
      // rule, and author styles win that tie — so the `hidden` attribute
      // alone doesn't actually hide it. An inline style always outranks a
      // stylesheet class, so set/clear it here instead of touching CSS.
      panel.style.display = isActive ? "" : "none";
      panel.classList.toggle("is-active", isActive);
    }
  }

  tabs.forEach((tab) => tab.addEventListener("click", () => activate(tab.dataset.tab)));
  window.addEventListener("resize", () => {
    const current = tabs.find((t) => t.classList.contains("is-active"));
    if (current) moveIndicator(current);
  });

  // Reading offsetLeft/offsetWidth forces layout synchronously, so this
  // doesn't need to wait for a paint frame — and rAF would never fire if
  // this page loads in a background tab, leaving the indicator at 0x0.
  activate("login");
}

// --- Password show/hide toggle ---

function initPasswordToggles() {
  document.querySelectorAll("[data-toggle-for]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const input = document.getElementById(btn.dataset.toggleFor);
      const showing = input.type === "text";
      input.type = showing ? "password" : "text";
      btn.textContent = showing ? "👁️" : "🙈";
      btn.setAttribute("aria-label", showing ? "Show password" : "Hide password");
    });
  });
}

initTabs();
initPasswordToggles();

document.getElementById("login-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const form = e.currentTarget;
  const errorEl = document.getElementById("login-error");
  errorEl.hidden = true;
  setLoading(form, true);
  try {
    await postJSON("/api/auth/login", {
      email: document.getElementById("email").value,
      password: document.getElementById("password").value,
    });
    window.location.href = "/";
  } catch (err) {
    showMessage(errorEl, err.message, "error");
    setLoading(form, false);
  }
});

document.getElementById("register-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const form = e.currentTarget;
  const msgEl = document.getElementById("register-message");
  msgEl.hidden = true;
  setLoading(form, true);
  try {
    await postJSON("/api/auth/register", {
      email: document.getElementById("reg-email").value,
      password: document.getElementById("reg-password").value,
    });
    showMessage(msgEl, "Account created — you can sign in now.", "success");
    form.reset();
  } catch (err) {
    showMessage(msgEl, err.message, "error");
  } finally {
    setLoading(form, false);
  }
});
