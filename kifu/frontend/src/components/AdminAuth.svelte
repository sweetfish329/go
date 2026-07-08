<script lang="ts">
  let { onLoginSuccess } = $props<{ onLoginSuccess: () => void }>();

  let username = $state("");
  let password = $state("");
  let loading = $state(false);
  let error = $state<string | null>(null);

  async function handleLogin(e: SubmitEvent) {
    e.preventDefault();
    if (!username.trim() || !password.trim()) {
      error = "ユーザー名とパスワードを入力してください。";
      return;
    }

    loading = true;
    error = null;

    try {
      const res = await fetch("/api/admin/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password })
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || "ログインに失敗しました。");
      }

      localStorage.setItem("admin_token", data.token);
      onLoginSuccess();
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }
</script>

<div class="row animate-fade-in" style="margin-top: 4rem;">
  <div class="col s12 m8 l6 offset-m2 offset-l3">
    <!-- Vogue style sharp editorial card -->
    <div class="nm-card admin-auth-card">
      <div class="card-content" style="padding: 3rem 2.5rem;">
        
        <!-- Header -->
        <span class="card-title center-align font-mincho">
          <i class="material-icons title-icon">security</i>
          管理者ログイン
        </span>

        <!-- Harmonized Error Box -->
        {#if error}
          <div class="error-panel font-sans">
            <i class="material-icons error-icon">error_outline</i>
            <span>{error}</span>
          </div>
        {/if}

        <form onsubmit={handleLogin}>
          <!-- Username Input -->
          <div class="input-field-wrapper">
            <div class="input-field" style="margin-bottom: 0;">
              <i class="material-icons prefix">person</i>
              <input 
                id="admin_username" 
                type="text" 
                class="nm-input" 
                bind:value={username} 
                required 
                style="padding-left: 3rem !important;" 
              />
              <label for="admin_username" class="active font-sans">ユーザー名 (Admin)</label>
            </div>
          </div>

          <!-- Password Input -->
          <div class="input-field-wrapper" style="margin-top: 24px;">
            <div class="input-field" style="margin-bottom: 0;">
              <i class="material-icons prefix">lock</i>
              <input 
                id="admin_password" 
                type="password" 
                class="nm-input" 
                bind:value={password} 
                required 
                style="padding-left: 3rem !important;" 
              />
              <label for="admin_password" class="active font-sans">パスワード</label>
            </div>
          </div>

          <!-- Login Button -->
          <div style="margin-top: 36px;">
            <button class="login-btn font-sans" type="submit" disabled={loading}>
              {loading ? '認証中...' : 'ログイン'}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</div>

<style>
  .admin-auth-card {
    border-radius: 0px !important; /* Sharp Vogue corners */
    border: 1.5px solid var(--wc-text) !important;
    background: var(--wc-surface) !important;
    box-shadow: 8px 8px 0px var(--wc-shadow-dark) !important;
  }

  .card-title {
    font-weight: 800;
    font-size: 1.5rem;
    margin-bottom: 2.5rem;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    color: var(--wc-text);
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .title-icon {
    font-size: 1.8rem;
    color: var(--wc-accent);
  }

  /* Harmonized Error Box */
  .error-panel {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(160, 50, 40, 0.06);
    border: 1.5px solid #9d2f2f;
    border-radius: 0px !important;
    padding: 12px 16px;
    margin-bottom: 24px;
    color: #9d2f2f;
    font-size: 0.88rem;
    line-height: 1.4;
    text-align: left;
  }

  .error-icon {
    font-size: 1.2rem;
    flex-shrink: 0;
  }

  .input-field-wrapper {
    position: relative;
  }

  .input-field label.active {
    transform: translateY(-12px) scale(0.8);
    left: 0.75rem;
    color: var(--wc-text-muted) !important;
  }

  .input-field .prefix {
    color: var(--wc-text-muted) !important;
    font-size: 1.3rem;
    margin-top: 10px;
  }

  /* Input Style - Vogue sharp borders */
  .nm-input {
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-border) !important;
    background: rgba(245, 240, 232, 0.5) !important;
    color: var(--wc-text) !important;
    padding: 10px 14px !important;
    font-size: 0.95rem !important;
    box-shadow: none !important;
    width: 100%;
    box-sizing: border-box;
  }

  .nm-input:focus {
    border-color: var(--wc-accent) !important;
    outline: none !important;
  }

  /* Vogue sharp solid button */
  .login-btn {
    width: 100%;
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-text) !important;
    background: var(--wc-accent) !important;
    color: #FFFFFF !important;
    box-shadow: 4px 4px 0px var(--wc-text) !important;
    padding: 12px 24px !important;
    font-weight: 700;
    font-size: 0.95rem;
    letter-spacing: 0.05em;
    cursor: pointer;
    transition: var(--wc-transition-fast);
  }

  .login-btn:hover:not(:disabled) {
    transform: translate(-1px, -1px);
    box-shadow: 5px 5px 0px var(--wc-text) !important;
    background: var(--wc-accent-hover) !important;
  }

  .login-btn:active:not(:disabled) {
    transform: translate(1px, 1px);
    box-shadow: 2px 2px 0px var(--wc-text) !important;
  }

  .login-btn:disabled {
    opacity: 0.55;
    cursor: not-allowed;
    box-shadow: none !important;
    transform: none !important;
  }

  .font-mincho {
    font-family: 'Shippori Mincho B1', serif;
  }

  .font-sans {
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
  }

  .animate-fade-in {
    animation: fadeIn 0.35s ease-out;
  }

  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(15px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>
