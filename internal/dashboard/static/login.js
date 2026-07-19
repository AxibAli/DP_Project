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

document.getElementById("login-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const errorEl = document.getElementById("login-error");
  errorEl.style.display = "none";
  try {
    await postJSON("/api/auth/login", {
      email: document.getElementById("email").value,
      password: document.getElementById("password").value,
    });
    window.location.href = "/";
  } catch (err) {
    errorEl.textContent = err.message;
    errorEl.style.display = "block";
  }
});

document.getElementById("register-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const msgEl = document.getElementById("register-message");
  msgEl.style.display = "block";
  try {
    await postJSON("/api/auth/register", {
      email: document.getElementById("reg-email").value,
      password: document.getElementById("reg-password").value,
    });
    msgEl.textContent = "Account created — you can sign in now.";
    msgEl.className = "success-text";
  } catch (err) {
    msgEl.textContent = err.message;
    msgEl.className = "error-text";
  }
});
