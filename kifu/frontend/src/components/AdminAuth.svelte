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
    <div class="nm-card">
      <div class="card-content" style="padding: 2.5rem 2rem;">
        <span class="card-title center-align" style="font-weight: 600; font-size: 1.5rem; margin-bottom: 2.5rem; display: flex; align-items: center; justify-content: center; gap: 8px; font-family: 'Shippori Mincho B1', serif; color: var(--wc-accent);">
          <i class="material-icons" style="font-size: 1.8rem;">security</i> 管理者ログイン
        </span>

        {#if error}
          <div style="display: flex; align-items: center; gap: 8px; background: rgba(160,50,40,0.08); border: 1px solid rgba(160,50,40,0.2); border-radius: 10px; padding: 12px 16px; margin-bottom: 1.5rem; color: #8B2020; font-size: 0.88rem; font-family: 'DM Sans', sans-serif;">
            <i class="material-icons" style="font-size: 1.2rem;">error_outline</i>
            <span>{error}</span>
          </div>
        {/if}

        <form onsubmit={handleLogin}>
          <div class="input-field" style="margin-bottom: 1.5rem; position: relative;">
            <i class="material-icons prefix" style="color: var(--wc-text-muted); font-size: 1.3rem; margin-top: 10px;">person</i>
            <input id="admin_username" type="text" class="nm-input" bind:value={username} required style="padding-left: 3rem !important;" />
            <label for="admin_username" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text-muted);">ユーザー名 (Admin)</label>
          </div>

          <div class="input-field" style="margin-bottom: 2rem; position: relative;">
            <i class="material-icons prefix" style="color: var(--wc-text-muted); font-size: 1.3rem; margin-top: 10px;">lock</i>
            <input id="admin_password" type="password" class="nm-input" bind:value={password} required style="padding-left: 3rem !important;" />
            <label for="admin_password" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text-muted);">パスワード</label>
          </div>

          <button class="nm-btn-primary" type="submit" disabled={loading} style="width: 100%;">
            {loading ? '認証中...' : 'ログイン'}
          </button>
        </form>
      </div>
    </div>
  </div>
</div>

<style>
  .animate-fade-in {
    animation: fadeIn 0.35s ease-out;
  }
  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(15px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>
