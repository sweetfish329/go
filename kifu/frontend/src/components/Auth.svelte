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

      <!-- Decorative washi dots -->
      <div class="auth-washi-dot auth-washi-dot--1" aria-hidden="true"></div>
      <div class="auth-washi-dot auth-washi-dot--2" aria-hidden="true"></div>

      <div class="nm-card auth-card" style="margin-top: 2rem;">
        <div class="card-content" style="padding: 3rem 2.5rem; position: relative; z-index: 1;">

          <!-- Header -->
          <div class="center-align" style="margin-bottom: 2.5rem;">
            <div class="auth-icon-wrap" aria-hidden="true">
              <span class="auth-main-icon">●</span>
              <span class="auth-go-white">○</span>
            </div>
            <h1 class="auth-title">kifu store</h1>
            <p class="auth-subtitle">
              ソーシャルアカウントでログイン・新規登録
            </p>
          </div>

          {#if error}
            <div class="error-panel">
              <i class="material-icons" style="font-size: 1.1rem;">error_outline</i>
              <span>{error}</span>
            </div>
          {/if}

          <!-- Social Login Buttons -->
          {#if loading}
            <div class="center-align" style="margin: 2.5rem 0;">
              <div class="nm-spinner mx-auto"></div>
              <p class="text-muted" style="margin-top: 12px; font-size: 0.9rem;">読み込み中...</p>
            </div>
          {:else}
            {#if !providers.google && !providers.line && !providers.meta}
              <div class="nm-panel-inset center-align" style="margin-top: 15px;">
                <i class="material-icons" style="font-size: 2.5rem; color: var(--nm-accent); margin-bottom: 8px; display: block;">info_outline</i>
                <p class="font-outfit" style="margin: 0; font-weight: 600; font-size: 1.05rem; color: var(--nm-text-main);">ソーシャルログインは現在無効化されています</p>
                <p style="margin: 8px 0 0 0; font-size: 0.88rem; color: var(--nm-text-muted);">管理者へお問い合わせください。</p>
              </div>
            {:else}
              <div class="social-login-grid" style="display: flex; flex-direction: column; gap: 14px;">
                <!-- Google Button -->
                {#if providers.google}
                  <button class="social-btn google-btn" onclick={() => handleOAuth('google')}>
                    <span class="social-btn-icon google-icon">G</span>
                    <span>Googleでログイン / 新規登録</span>
                    <span class="social-btn-arrow">→</span>
                  </button>
                {/if}

                <!-- LINE Button -->
                {#if providers.line}
                  <button class="social-btn line-btn" onclick={() => handleOAuth('line')}>
                    <i class="material-icons social-btn-icon">chat</i>
                    <span>LINEでログイン / 新規登録</span>
                    <span class="social-btn-arrow">→</span>
                  </button>
                {/if}

                <!-- Meta Button -->
                {#if providers.meta}
                  <button class="social-btn meta-btn" onclick={() => handleOAuth('meta')}>
                    <i class="material-icons social-btn-icon">facebook</i>
                    <span>Metaでログイン / 新規登録</span>
                    <span class="social-btn-arrow">→</span>
                  </button>
                {/if}
              </div>
            {/if}
          {/if}

          <!-- Footer deco -->
          <p class="auth-footer-deco" aria-hidden="true">● ○ ● ○ ●</p>
        </div>
      </div>

    </div>
  </div>
</div>

<style>
  .auth-container {
    margin-top: 3rem;
    position: relative;
  }

  /* Decorative washi dots (静的・アニメーションなし) */
  .auth-washi-dot {
    position: absolute;
    border-radius: 50%;
    pointer-events: none;
    z-index: 0;
  }
  .auth-washi-dot--1 {
    width: 120px;
    height: 120px;
    top: -20px;
    right: 8%;
    background: radial-gradient(circle, rgba(200, 149, 108, 0.12) 0%, transparent 70%);
  }
  .auth-washi-dot--2 {
    width: 90px;
    height: 90px;
    bottom: 30px;
    left: 6%;
    background: radial-gradient(circle, rgba(124, 107, 82, 0.1) 0%, transparent 70%);
  }

  .auth-card {
    position: relative;
    z-index: 1;
  }

  /* Icon — 碁石モチーフ */
  .auth-icon-wrap {
    display: inline-flex;
    align-items: center;
    position: relative;
    margin-bottom: 16px;
    gap: 6px;
  }

  .auth-main-icon {
    font-size: 2.5rem;
    color: var(--wc-go-black);
    line-height: 1;
    text-shadow: 2px 3px 6px rgba(0,0,0,0.25);
  }

  .auth-go-white {
    position: relative;
    top: -10px;
    font-size: 1.8rem;
    color: transparent;
    text-shadow: 0 0 0 var(--wc-go-white);
    filter: drop-shadow(1px 2px 3px rgba(0,0,0,0.15));
    line-height: 1;
  }

  /* Title */
  .auth-title {
    margin: 10px 0 8px !important;
    font-size: 2rem !important;
    font-weight: 600 !important;
    letter-spacing: 0.06em;
    color: var(--wc-text);
    font-family: 'Cormorant Garamond', 'Shippori Mincho B1', serif;
    font-style: italic;
    line-height: 1.3 !important;
  }

  .auth-subtitle {
    font-size: 0.88rem;
    color: var(--wc-text-muted);
    margin: 0;
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
    letter-spacing: 0.01em;
  }

  /* Error panel */
  .error-panel {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(160, 50, 40, 0.08);
    border: 1px solid rgba(160, 50, 40, 0.2);
    border-radius: 12px;
    padding: 10px 16px;
    margin-bottom: 20px;
    color: #8B2020;
    font-size: 0.88rem;
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
  }

  /* Social Buttons — Washi Clay Pill */
  .social-btn {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 13px 20px;
    border-radius: var(--wc-radius-pill);
    border: 1px solid var(--wc-border);
    background: var(--wc-surface);
    box-shadow: var(--nm-shadow-outset-sm);
    cursor: pointer;
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
    font-size: 0.93rem;
    font-weight: 500;
    color: var(--wc-text);
    transition: all 0.28s cubic-bezier(0.25, 0.46, 0.45, 0.94);
    text-align: left;
  }

  .social-btn:hover {
    transform: translateY(-2px);
    box-shadow: var(--nm-shadow-outset-sm-hover);
    border-color: var(--wc-accent);
    color: var(--wc-accent);
  }

  .social-btn:active {
    transform: translateY(1px) scale(0.99);
    box-shadow: var(--nm-shadow-inset);
  }

  .social-btn-icon {
    width: 28px;
    height: 28px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    flex-shrink: 0;
    font-size: 1rem;
  }

  .social-btn-arrow {
    margin-left: auto;
    font-size: 1rem;
    opacity: 0;
    transform: translateX(-4px);
    transition: all 0.2s ease;
    color: var(--wc-accent);
  }

  .social-btn:hover .social-btn-arrow {
    opacity: 1;
    transform: translateX(0);
  }

  /* Google */
  .google-btn .social-btn-icon.google-icon {
    background: linear-gradient(135deg, #ea4335, #fbbc05, #34a853, #4285f4);
    color: white;
    font-weight: 900;
    font-size: 0.85rem;
  }

  /* LINE */
  .line-btn {
    border-color: rgba(6, 199, 85, 0.3) !important;
  }
  .line-btn .social-btn-icon {
    background: #06c755;
    color: white;
  }
  .line-btn:hover {
    border-color: rgba(6, 199, 85, 0.6) !important;
  }

  /* Meta */
  .meta-btn {
    border-color: rgba(24, 119, 242, 0.3) !important;
  }
  .meta-btn .social-btn-icon {
    background: #1877f2;
    color: white;
  }
  .meta-btn:hover {
    border-color: rgba(24, 119, 242, 0.5) !important;
  }

  /* Footer decoration — 碁石パターン */
  .auth-footer-deco {
    text-align: center;
    margin-top: 2rem;
    margin-bottom: 0;
    font-size: 0.9rem;
    letter-spacing: 0.6em;
    color: var(--wc-accent);
    opacity: 0.35;
  }

  /* Loading spinner override */
  .mx-auto {
    margin-left: auto;
    margin-right: auto;
  }
</style>
