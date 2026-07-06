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
      <div class="nm-card pixel-border-sm" style="margin-top: 2rem;">
        <div class="card-content" style="padding: 3rem 2.5rem;">
          <div class="center-align" style="margin-bottom: 2.5rem;">
            <i class="material-icons large animate-pop-in" style="font-size: 5rem; color: var(--nm-accent);">grid_on</i>
            <h4 class="font-pixel" style="margin-top: 15px; margin-bottom: 12px; font-size: 1.8rem; color: var(--nm-accent); font-weight: 700;">ログイン</h4>
            <p style="font-size: 0.95rem; color: var(--nm-text-muted);">ソーシャルアカウントを使用してログインまたは新規登録を行います</p>
          </div>

          {#if error}
            <div class="card-panel red lighten-4 red-text text-darken-4 valign-wrapper font-pixel" style="padding: 10px 15px; border-radius: 8px; margin-bottom: 20px; border: 1px solid rgba(239, 83, 80, 0.3);">
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
              <div class="nm-panel-inset center-align" style="border-radius: 8px; padding: 1.5rem; margin-top: 15px;">
                <i class="material-icons orange-text" style="font-size: 2.5rem; margin-bottom: 8px;">warning</i>
                <p class="font-pixel" style="margin: 0; font-weight: 600; font-size: 1.05rem; color: var(--nm-text-main);">現在、ソーシャルログインは一時的に無効化されています。</p>
                <p style="margin: 5px 0 0 0; font-size: 0.9rem; color: var(--nm-text-muted);">恐れ入りますが、しばらく時間をおいてから再度お試しいただくか、管理者へお問い合わせください。</p>
              </div>
            {:else}
              <div class="social-login-grid font-pixel" style="display: flex; flex-direction: column; gap: 16px;">
                <!-- Google Button -->
                {#if providers.google}
                  <button class="nm-btn google-btn waves-effect w-100" onclick={() => handleOAuth('google')}>
                    <i class="material-icons">g_mobiledata</i> Googleでログイン / 新規登録
                  </button>
                {/if}
                <!-- LINE Button -->
                {#if providers.line}
                  <button class="nm-btn line-btn waves-effect w-100" onclick={() => handleOAuth('line')}>
                    <i class="material-icons">chat</i> LINEでログイン / 新規登録
                  </button>
                {/if}
                <!-- Meta Button -->
                {#if providers.meta}
                  <button class="nm-btn meta-btn waves-effect w-100" onclick={() => handleOAuth('meta')}>
                    <i class="material-icons">facebook</i> Metaでログイン / 新規登録
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
  .google-btn {
    background-color: var(--nm-bg-element) !important;
    color: #3c4043 !important;
    border: var(--nm-border-light) !important;
    box-shadow: var(--nm-shadow-outset-sm) !important;
  }
  .google-btn:hover {
    box-shadow: var(--nm-shadow-outset-sm-hover) !important;
  }
  .google-btn:active {
    box-shadow: var(--nm-shadow-inset) !important;
  }
  .google-btn i {
    color: #ea4335 !important;
    font-size: 2.5rem !important;
    margin-right: -4px;
    margin-left: -6px;
  }
  
  .line-btn {
    background-color: #06c755 !important;
    color: #ffffff !important;
    border: 1px solid rgba(6, 199, 85, 0.2) !important;
    box-shadow: 3px 3px 6px rgba(6, 199, 85, 0.2), -3px -3px 6px rgba(255, 255, 255, 0.7) !important;
  }
  .line-btn:hover {
    background-color: #05b34c !important;
    box-shadow: 4px 4px 8px rgba(6, 199, 85, 0.3), -4px -4px 8px rgba(255, 255, 255, 0.8) !important;
  }
  .line-btn:active {
    box-shadow: inset 3px 3px 6px rgba(0, 0, 0, 0.2) !important;
  }
  .line-btn i {
    font-size: 1.3rem !important;
  }
  
  .meta-btn {
    background-color: #1877f2 !important;
    color: #ffffff !important;
    border: 1px solid rgba(24, 119, 242, 0.2) !important;
    box-shadow: 3px 3px 6px rgba(24, 119, 242, 0.2), -3px -3px 6px rgba(255, 255, 255, 0.7) !important;
  }
  .meta-btn:hover {
    background-color: #166fe5 !important;
    box-shadow: 4px 4px 8px rgba(24, 119, 242, 0.3), -4px -4px 8px rgba(255, 255, 255, 0.8) !important;
  }
  .meta-btn:active {
    box-shadow: inset 3px 3px 6px rgba(0, 0, 0, 0.2) !important;
  }
  .meta-btn i {
    font-size: 1.3rem !important;
  }
  .w-100 {
    width: 100% !important;
  }
</style>
