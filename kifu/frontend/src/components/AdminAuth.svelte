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
    <div class="card white" style="border-radius: 12px; box-shadow: 0 8px 24px rgba(0,0,0,0.08); border: 1px solid rgba(0,0,0,0.04);">
      <div class="card-content" style="padding: 2rem;">
        <span class="card-title center-align brown-text text-darken-3" style="font-weight: 500; font-size: 1.6rem; margin-bottom: 2rem; display: flex; align-items: center; justify-content: center; gap: 8px;">
          <i class="material-icons" style="font-size: 2rem;">security</i> 管理者ログイン
        </span>

        {#if error}
          <div class="card-panel red lighten-4 red-text text-darken-4" style="padding: 10px 15px; border-radius: 6px; margin-bottom: 1.5rem;">
            <i class="material-icons left" style="margin-right: 8px;">error</i>{error}
          </div>
        {/if}

        <form onsubmit={handleLogin}>
          <div class="input-field" style="margin-bottom: 1.5rem;">
            <i class="material-icons prefix">person</i>
            <input id="admin_username" type="text" bind:value={username} required />
            <label for="admin_username">ユーザー名 (Admin)</label>
          </div>

          <div class="input-field" style="margin-bottom: 2rem;">
            <i class="material-icons prefix">lock</i>
            <input id="admin_password" type="password" bind:value={password} required />
            <label for="admin_password">パスワード</label>
          </div>

          <button class="btn waves-effect waves-light brown darken-2 w-100" type="submit" disabled={loading} style="width: 100%; height: 45px; line-height: 45px; border-radius: 6px; font-weight: 500; font-size: 1.05rem;">
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
