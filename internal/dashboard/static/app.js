// Vanilla JS polling client for the Livestock Health Monitor dashboard.
// No frameworks, no build step — this file is served as-is.

const POLL_INTERVAL_MS = 3000;

function severityClass(alerts) {
  if (alerts.some(a => a.severity === "critical")) return "critical";
  if (alerts.some(a => a.severity === "warning")) return "warning";
  return "";
}

function getCookie(name) {
  const match = document.cookie.match(new RegExp("(?:^|; )" + name + "=([^;]*)"));
  return match ? decodeURIComponent(match[1]) : "";
}

async function fetchJSON(url, opts = {}) {
  const headers = Object.assign({}, opts.headers || {});
  if (opts.method && opts.method !== "GET") {
    headers["X-CSRF-Token"] = getCookie("csrf_token");
  }
  const res = await fetch(url, Object.assign({}, opts, { headers }));
  if (res.status === 401) {
    window.location.href = "/login";
    throw new Error("not authenticated");
  }
  if (!res.ok) throw new Error(`${url} -> ${res.status}`);
  if (res.status === 204) return null;
  return res.json();
}

// --- Overview page ---

async function refreshOverview() {
  const grid = document.getElementById("animal-grid");
  const banner = document.getElementById("alert-banner");
  if (!grid) return;

  try {
    const [animals, activeAlerts] = await Promise.all([
      fetchJSON("/api/animals"),
      fetchJSON("/api/alerts"),
    ]);

    const alertsByAnimal = {};
    for (const alert of activeAlerts) {
      (alertsByAnimal[alert.animal_id] = alertsByAnimal[alert.animal_id] || []).push(alert);
    }

    if (activeAlerts.length > 0) {
      banner.style.display = "block";
      banner.textContent = `${activeAlerts.length} active alert(s) across the herd`;
    } else {
      banner.style.display = "none";
    }

    grid.innerHTML = "";
    for (const animal of animals) {
      const detail = await fetchJSON(`/api/animals/${animal.id}`).catch(() => null);
      const latest = detail && detail.latest_reading;
      const alerts = alertsByAnimal[animal.id] || [];
      const sevClass = severityClass(alerts);

      const card = document.createElement("a");
      card.className = `animal-card ${sevClass}`;
      card.href = `/animals/${animal.id}`;

      const badge = sevClass
        ? `<span class="badge ${sevClass}">${sevClass.toUpperCase()}</span>`
        : `<span class="badge ok">OK</span>`;

      card.innerHTML = `
        <h3>${animal.name} <span class="muted">${animal.tag}</span></h3>
        <div class="muted">${animal.species}</div>
        <div class="metric-row">
          <span>🌡️ ${latest ? latest.temperature.toFixed(1) + "°C" : "—"}</span>
          <span>❤️ ${latest ? latest.heart_rate + " bpm" : "—"}</span>
          <span>🏃 ${latest ? latest.activity : "—"}</span>
        </div>
        ${badge}
      `;
      grid.appendChild(card);
    }
  } catch (err) {
    console.error("overview refresh failed", err);
  }
}

// --- Animal detail page ---

function drawTrend(canvas, readings) {
  const ctx = canvas.getContext("2d");
  const w = canvas.width, h = canvas.height;
  ctx.clearRect(0, 0, w, h);
  if (readings.length < 2) return;

  const temps = readings.map(r => r.temperature).reverse();
  const min = Math.min(...temps) - 0.5;
  const max = Math.max(...temps) + 0.5;
  const stepX = w / (temps.length - 1);

  ctx.strokeStyle = "#22543d";
  ctx.lineWidth = 2;
  ctx.beginPath();
  temps.forEach((t, i) => {
    const x = i * stepX;
    const y = h - ((t - min) / (max - min)) * h;
    if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
  });
  ctx.stroke();
}

async function refreshDetail(animalId) {
  try {
    const [detail, readings, alerts, vaccinations] = await Promise.all([
      fetchJSON(`/api/animals/${animalId}`),
      fetchJSON(`/api/animals/${animalId}/readings?limit=20`),
      fetchJSON(`/api/animals/${animalId}/alerts`),
      fetchJSON(`/api/animals/${animalId}/vaccinations`),
    ]);

    const latest = detail.latest_reading;
    document.getElementById("current-reading").innerHTML = latest
      ? `<div class="metric-row">
           <span>🌡️ ${latest.temperature.toFixed(1)}°C</span>
           <span>❤️ ${latest.heart_rate} bpm</span>
           <span>🏃 activity ${latest.activity}</span>
         </div>
         <div class="muted">as of ${new Date(latest.created_at).toLocaleTimeString()}</div>`
      : "No readings yet.";

    const canvas = document.getElementById("trend-canvas");
    if (canvas) drawTrend(canvas, readings);

    const alertsEl = document.getElementById("animal-alerts");
    alertsEl.innerHTML = alerts.length
      ? `<table><tr><th>Seq</th><th>Reason</th><th>Severity</th><th>Resolved</th></tr>` +
        alerts.map(a => `<tr>
            <td>${a.sequence_no}</td><td>${a.reason}</td>
            <td>${a.severity}</td><td>${a.resolved ? "yes" : "no"}</td>
          </tr>`).join("") + `</table>`
      : "No alerts.";

    const vacEl = document.getElementById("vaccinations");
    vacEl.innerHTML = vaccinations.length
      ? `<table><tr><th>Name</th><th>Given</th><th>Next due</th></tr>` +
        vaccinations.map(v => `<tr>
            <td>${v.name}</td>
            <td>${new Date(v.date_given).toLocaleDateString()}</td>
            <td>${v.next_due ? new Date(v.next_due).toLocaleDateString() : "—"}</td>
          </tr>`).join("") + `</table>`
      : "No vaccination records.";
  } catch (err) {
    console.error("detail refresh failed", err);
  }
}

function initDetailForm(animalId) {
  const form = document.getElementById("vaccination-form");
  if (!form) return;
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const nameInput = document.getElementById("vac-name");
    const name = nameInput.value.trim();
    if (!name) return;
    await fetchJSON(`/api/animals/${animalId}/vaccinations`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, date_given: new Date().toISOString() }),
    });
    nameInput.value = "";
    refreshDetail(animalId);
  });
}

function initLogout() {
  const btn = document.getElementById("logout-btn");
  if (!btn) return;
  btn.addEventListener("click", async () => {
    await fetchJSON("/api/auth/logout", { method: "POST" }).catch(() => {});
    window.location.href = "/login";
  });
}

// --- Admin users page ---

async function loadUsers() {
  const table = document.getElementById("users-table");
  if (!table) return;

  const users = await fetchJSON("/api/users");
  table.innerHTML = "<tr><th>Email</th><th>Role</th><th>Created</th><th></th></tr>";
  for (const u of users) {
    const row = document.createElement("tr");
    const canDelete = u.id !== window.CURRENT_USER_ID;
    row.innerHTML = `
      <td>${u.email}</td>
      <td>
        <select data-id="${u.id}" class="role-select">
          <option value="USER" ${u.role === "USER" ? "selected" : ""}>USER</option>
          <option value="ADMIN" ${u.role === "ADMIN" ? "selected" : ""}>ADMIN</option>
        </select>
      </td>
      <td>${new Date(u.created_at).toLocaleDateString()}</td>
      <td>${canDelete ? `<button data-id="${u.id}" class="delete-btn">Delete</button>` : ""}</td>
    `;
    table.appendChild(row);
  }

  table.querySelectorAll(".role-select").forEach((sel) => {
    sel.addEventListener("change", async () => {
      await fetchJSON(`/api/users/${sel.dataset.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ role: sel.value }),
      });
      loadUsers();
    });
  });
  table.querySelectorAll(".delete-btn").forEach((btn) => {
    btn.addEventListener("click", async () => {
      await fetchJSON(`/api/users/${btn.dataset.id}`, { method: "DELETE" });
      loadUsers();
    });
  });
}

function initUsersPage() {
  const form = document.getElementById("create-user-form");
  if (!form) return;
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    await fetchJSON("/api/users", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email: document.getElementById("new-email").value,
        password: document.getElementById("new-password").value,
        role: document.getElementById("new-role").value,
      }),
    });
    form.reset();
    loadUsers();
  });
  loadUsers();
}

document.addEventListener("DOMContentLoaded", () => {
  initLogout();
  initUsersPage();
  if (document.getElementById("animal-grid")) {
    refreshOverview();
    setInterval(refreshOverview, POLL_INTERVAL_MS);
  }
  if (typeof window.ANIMAL_ID !== "undefined") {
    initDetailForm(window.ANIMAL_ID);
    refreshDetail(window.ANIMAL_ID);
    setInterval(() => refreshDetail(window.ANIMAL_ID), POLL_INTERVAL_MS);
  }
});
