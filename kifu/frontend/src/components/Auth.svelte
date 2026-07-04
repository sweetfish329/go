<script lang="ts">
  import { auth } from '../lib/auth';

  let { onLoginSuccess } = $props<{ onLoginSuccess: () => void }>();

  let isLogin = $state(true);
  let username = $state("");
  let password = $state("");
  let error = $state<string | null>(null);
  let loading = $state(false);

  const getM = () => (window as any).M;

  async function handleSubmit(e: Event) {
    e.preventDefault();
    if (!username.trim() || !password.trim()) {
      error = "ユーザー名とパスワードを入力してください。";
      return;
    }

    loading = true;
    error = null;

    const endpoint = isLogin ? '/api/auth/login' : '/api/auth/register';

    try {
      const res = await fetch(endpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          username: username.trim(),
          password: password.trim()
        })
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || "認証に失敗しました。");
      }

      auth.setLogin(data.token, data.user.username, data.user.id);
      
      const M = getM();
      if (M) {
        M.toast({ html: isLogin ? 'ログインしました！' : 'アカウントが作成されました！', classes: 'green darken-1' });
      }

      onLoginSuccess();
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }
</script>

<div class="auth-container animate-fade-in">
  <div class="row">
    <div class="col s12 m8 offset-m2 l6 offset-l3">
      <div class="card glass-card">
        <div class="card-content">
          <div class="center-align">
            <i class="material-icons large brown-text text-darken-2">grid_on</i>
            <h4 class="brown-text text-darken-3 font-weight-500" style="margin-top: 10px; margin-bottom: 5px;">{isLogin ? 'ログイン' : '新規アカウント作成'}</h4>
            <p class="grey-text text-darken-1">棋譜のアップロードと管理を開始しましょう</p>
          </div>

          {#if error}
            <div class="card-panel red lighten-4 red-text text-darken-4 valign-wrapper" style="padding: 10px; margin-top: 20px;">
              <i class="material-icons left">error</i>
              <span>{error}</span>
            </div>
          {/if}

          <form onsubmit={handleSubmit} style="margin-top: 20px;">
            <div class="input-field">
              <i class="material-icons prefix">account_circle</i>
              <input id="username" type="text" bind:value={username} required class="validate" />
              <label for="username" class={username ? 'active' : ''}>ユーザー名</label>
            </div>

            <div class="input-field" style="margin-top: 20px;">
              <i class="material-icons prefix">lock</i>
              <input id="password" type="password" bind:value={password} required class="validate" />
              <label for="password" class={password ? 'active' : ''}>パスワード</label>
            </div>

            <div class="center-align" style="margin-top: 30px;">
              <button class="btn btn-large waves-effect waves-light brown darken-2 w-100" type="submit" disabled={loading} style="width: 100%; border-radius: 6px;">
                {#if loading}
                  処理中...
                {:else}
                  {isLogin ? 'ログイン' : '登録する'}
                {/if}
              </button>
            </div>
          </form>
        </div>

        <div class="card-action center-align" style="background-color: rgba(250, 250, 250, 0.8);">
          <!-- svelte-ignore a11y-invalid-attribute -->
          <!-- svelte-ignore a11y-missing-attribute -->
          <a class="cursor-pointer brown-text text-darken-2" onclick={() => { isLogin = !isLogin; error = null; }} style="cursor: pointer;">
            {isLogin ? 'アカウントをお持ちでないですか？ 新規作成' : 'すでにアカウントをお持ちですか？ ログイン'}
          </a>
        </div>
      </div>
    </div>
  </div>
</div>

<style>
  .auth-container {
    margin-top: 4rem;
  }
  .glass-card {
    background: rgba(255, 255, 255, 0.95);
    border-radius: 12px;
    box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.08);
    border: 1px solid rgba(0, 0, 0, 0.05);
  }
  .font-weight-500 {
    font-weight: 500;
  }
  .cursor-pointer {
    cursor: pointer;
  }
  .animate-fade-in {
    animation: fadeIn 0.4s ease-out;
  }
  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(20px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>
