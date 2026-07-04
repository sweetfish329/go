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

  async function handleOAuth(provider: string) {
    loading = true;
    error = null;

    // In a real application, this would redirect to provider's auth endpoint,
    // but we simulate it by sending a post request with mock provider credentials.
    const providerUserId = `${provider}-mock-${Math.floor(Math.random() * 900000 + 100000)}`;
    const defaultName = `${provider.charAt(0).toUpperCase() + provider.slice(1)}ユーザー`;

    try {
      const res = await fetch('/api/auth/oauth', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          provider: provider,
          provider_user_id: providerUserId,
          default_username: defaultName
        })
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || "外部アカウント認証に失敗しました。");
      }

      auth.setLogin(data.token, data.user.username, data.user.id);
      
      const M = getM();
      if (M) {
        M.toast({ 
          html: `${provider.toUpperCase()}アカウントでログインしました！`, 
          classes: 'green darken-1' 
        });
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

          <!-- Traditional Login Form -->
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

          <!-- Social Login Divider -->
          <div class="divider-container" style="margin: 25px 0; text-align: center; position: relative;">
            <div style="border-top: 1px solid #e0e0e0; position: absolute; width: 100%; top: 50%; z-index: 1;"></div>
            <span style="background: #fff; padding: 0 15px; position: relative; z-index: 2; color: #9e9e9e; font-size: 0.9rem;">または外部サービスでログイン</span>
          </div>

          <!-- Social Login Buttons -->
          <div class="social-login-grid" style="display: flex; flex-direction: column; gap: 12px;">
            <!-- Google Button -->
            <button class="btn social-btn google-btn waves-effect w-100" onclick={() => handleOAuth('google')} disabled={loading}>
              <i class="material-icons left">g_mobiledata</i> Googleでログイン
            </button>
            <!-- LINE Button -->
            <button class="btn social-btn line-btn waves-effect w-100" onclick={() => handleOAuth('line')} disabled={loading}>
              <i class="material-icons left">chat</i> LINEでログイン
            </button>
            <!-- Meta Button -->
            <button class="btn social-btn meta-btn waves-effect w-100" onclick={() => handleOAuth('meta')} disabled={loading}>
              <i class="material-icons left">facebook</i> Metaでログイン
            </button>
          </div>
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
    margin-top: 2rem;
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
  
  /* Social buttons styling */
  .social-btn {
    text-transform: none;
    border-radius: 6px;
    box-shadow: none;
    font-weight: 500;
    display: flex;
    align-items: center;
    justify-content: center;
    height: 42px;
  }
  .google-btn {
    background-color: #f5f5f5 !important;
    color: #424242 !important;
    border: 1px solid #e0e0e0;
  }
  .google-btn i {
    color: #ea4335 !important;
    font-size: 2.2rem !important;
  }
  .line-btn {
    background-color: #06c755 !important;
    color: #ffffff !important;
  }
  .line-btn i {
    font-size: 1.3rem !important;
  }
  .meta-btn {
    background-color: #1877f2 !important;
    color: #ffffff !important;
  }
</style>
