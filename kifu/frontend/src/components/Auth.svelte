<script lang="ts">
  import { onMount } from 'svelte';

  let error = $state<string | null>(null);
  let loading = $state(true);
  let providers = $state<Record<string, boolean>>({
    google: false,
    line: false,
    meta: false
  });

  onMount(async () => {
    try {
      const res = await fetch('/api/auth/providers');
      if (res.ok) {
        providers = await res.json();
      }
    } catch (err) {
      console.error("プロバイダ状態の取得に失敗しました", err);
    } finally {
      loading = false;
    }
  });

  function handleOAuth(provider: string) {
    loading = true;
    error = null;
    window.location.href = `/api/auth/oauth/redirect/${provider}`;
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
          {#if loading}
            <div class="center-align" style="margin: 2rem 0;">
              <div class="preloader-wrapper small active">
                <div class="spinner-layer spinner-brown-only">
                  <div class="circle-clipper left"><div class="circle"></div></div>
                  <div class="gap-patch"><div class="circle"></div></div>
                  <div class="circle-clipper right"><div class="circle"></div></div>
                </div>
              </div>
            </div>
          {:else}
            {#if !providers.google && !providers.line && !providers.meta}
              <div class="card-panel orange lighten-5 orange-text text-darken-4 center-align" style="border-radius: 8px; padding: 1.5rem; border: 1px solid #ffe0b2; margin-top: 15px;">
                <i class="material-icons" style="font-size: 2.5rem; margin-bottom: 8px;">warning</i>
                <p style="margin: 0; font-weight: 500; font-size: 1.05rem;">現在、ソーシャルログインは一時的に無効化されています。</p>
                <p style="margin: 5px 0 0 0; font-size: 0.9rem; color: #757575;">恐れ入りますが、しばらく時間をおいてから再度お試しいただくか、管理者へお問い合わせください。</p>
              </div>
            {:else}
              <div class="social-login-grid" style="display: flex; flex-direction: column; gap: 14px;">
                <!-- Google Button -->
                {#if providers.google}
                  <button class="btn social-btn google-btn waves-effect w-100" onclick={() => handleOAuth('google')}>
                    <i class="material-icons left">g_mobiledata</i> Googleでログイン / 新規登録
                  </button>
                {/if}
                <!-- LINE Button -->
                {#if providers.line}
                  <button class="btn social-btn line-btn waves-effect w-100" onclick={() => handleOAuth('line')}>
                    <i class="material-icons left">chat</i> LINEでログイン / 新規登録
                  </button>
                {/if}
                <!-- Meta Button -->
                {#if providers.meta}
                  <button class="btn social-btn meta-btn waves-effect w-100" onclick={() => handleOAuth('meta')}>
                    <i class="material-icons left">facebook</i> Metaでログイン / 新規登録
                  </button>
                {/if}
              </div>
            {/if}
          {/if}
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
