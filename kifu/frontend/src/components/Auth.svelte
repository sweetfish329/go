<script lang="ts">
  import { auth } from '../lib/auth.svelte';

  let { onLoginSuccess } = $props<{ onLoginSuccess: () => void }>();

  let error = $state<string | null>(null);
  let loading = $state(false);

  const getM = () => (window as any).M;

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
        <div class="card-content" style="padding: 2.5rem 2rem;">
          <div class="center-align" style="margin-bottom: 2rem;">
            <i class="material-icons large brown-text text-darken-2" style="font-size: 5rem;">grid_on</i>
            <h4 class="brown-text text-darken-3 font-weight-500" style="margin-top: 15px; margin-bottom: 8px; font-size: 1.8rem;">ログイン</h4>
            <p class="grey-text text-darken-1" style="font-size: 0.95rem;">ソーシャルアカウントを使用してログインまたは新規登録を行います</p>
          </div>

          {#if error}
            <div class="card-panel red lighten-4 red-text text-darken-4 valign-wrapper" style="padding: 10px 15px; border-radius: 6px; margin-bottom: 20px;">
              <i class="material-icons left" style="margin-right: 8px;">error</i>
              <span>{error}</span>
            </div>
          {/if}

          <!-- Social Login Buttons -->
          <div class="social-login-grid" style="display: flex; flex-direction: column; gap: 14px;">
            <!-- Google Button -->
            <button class="btn social-btn google-btn waves-effect w-100" onclick={() => handleOAuth('google')} disabled={loading}>
              <i class="material-icons left">g_mobiledata</i> Googleでログイン / 新規登録
            </button>
            <!-- LINE Button -->
            <button class="btn social-btn line-btn waves-effect w-100" onclick={() => handleOAuth('line')} disabled={loading}>
              <i class="material-icons left">chat</i> LINEでログイン / 新規登録
            </button>
            <!-- Meta Button -->
            <button class="btn social-btn meta-btn waves-effect w-100" onclick={() => handleOAuth('meta')} disabled={loading}>
              <i class="material-icons left">facebook</i> Metaでログイン / 新規登録
            </button>
          </div>
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
    background: rgba(255, 255, 255, 0.96);
    border-radius: 12px;
    box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.08);
    border: 1px solid rgba(0, 0, 0, 0.04);
  }
  .font-weight-500 {
    font-weight: 500;
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
    height: 48px;
    line-height: 48px;
    font-size: 1rem;
    transition: background-color 0.2s, box-shadow 0.2s;
  }
  .social-btn i {
    margin-right: 10px;
  }
  .google-btn {
    background-color: #f8f9fa !important;
    color: #3c4043 !important;
    border: 1px solid #dadce0;
  }
  .google-btn:hover {
    background-color: #f1f3f4 !important;
    box-shadow: 0 1px 3px rgba(60,64,67,0.15);
  }
  .google-btn i {
    color: #ea4335 !important;
    font-size: 2.5rem !important;
  }
  .line-btn {
    background-color: #06c755 !important;
    color: #ffffff !important;
  }
  .line-btn:hover {
    background-color: #05b34c !important;
    box-shadow: 0 1px 3px rgba(6,199,85,0.25);
  }
  .line-btn i {
    font-size: 1.4rem !important;
  }
  .meta-btn {
    background-color: #1877f2 !important;
    color: #ffffff !important;
  }
  .meta-btn:hover {
    background-color: #166fe5 !important;
    box-shadow: 0 1px 3px rgba(24,119,242,0.25);
  }
  .meta-btn i {
    font-size: 1.4rem !important;
  }
</style>
