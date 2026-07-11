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

<div class="auth-container animate-fade-in" style="margin-top: 2rem; position: relative;">
  <div class="row" style="display: flex; flex-wrap: wrap; align-items: center; gap: 30px 0;">
    <!-- Left Column: Giant Editorial Typo (Magazine Style) -->
    <div class="col s12 l7" style="position: relative; z-index: 2; padding-right: 40px; text-align: left;">
      <span class="em-collage-tag-pastel em-float-badge" style="font-size: 0.65rem; font-family: 'JetBrains Mono', monospace; box-shadow: 2px 2px 0px var(--wc-text); border-width: 1.5px; display: inline-block; margin-bottom: 20px;">
        CATALOGUE SYSTEM
      </span>
      <h2 style="font-family: 'Cormorant Garamond', serif; font-size: 6rem; font-style: italic; font-weight: 800; line-height: 0.9; color: var(--wc-text); margin: 0 0 20px 0; letter-spacing: -0.02em;">
        the gateway.
      </h2>
      <div style="border-left: 3px solid var(--wc-text); padding-left: 20px; max-width: 440px;">
        <p style="font-family: 'Shippori Mincho B1', serif; font-weight: 700; font-size: 1.1rem; line-height: 1.6; color: var(--wc-text); margin: 0 0 12px 0;">
          美しさと戦術が交錯する、私的な記録庫へ。
        </p>
        <p style="font-family: 'DM Sans', sans-serif; font-size: 0.85rem; line-height: 1.6; color: var(--wc-text-muted); margin: 0;">
          Kifu Store is a minimal, grid-driven digital archive for preserving and analyzing Go match specifications. Please verify your identity using your preferred network below.
        </p>
      </div>
      <!-- Huge Deco Number -->
      <div style="position: absolute; bottom: -80px; left: -10px; opacity: 0.05; font-family: 'Cormorant Garamond', serif; font-size: 14rem; font-weight: 900; pointer-events: none; user-select: none;">
        01
      </div>
    </div>

    <!-- Right Column: Login Card -->
    <div class="col s12 l5" style="position: relative; z-index: 3;">
      <div class="em-portfolio-section auth-card" style="border-color: var(--wc-text) !important; transform: rotate(-1.5deg); box-shadow: 8px 8px 0px var(--wc-text) !important; background: var(--wc-surface) !important; transition: transform 0.3s ease;">
        <!-- Overlap Badge with Floating animation -->
        <span class="em-collage-tag-pastel em-float-badge" style="position: absolute; top: -16px; left: 24px; font-size: 0.72rem; z-index: 10; box-shadow: 2.5px 2.5px 0px var(--wc-text); transform: rotate(1deg);">
          ACCESS GATEWAYS
        </span>

        <div class="card-content" style="padding: 3rem 2.2rem; position: relative; z-index: 1;">
          <!-- Giant Background text -->
          <div class="em-huge-title" style="position: absolute; top: 12%; left: 0; opacity: 0.04; font-size: 6rem; width: 100%; text-align: center; font-family: 'Cormorant Garamond', serif; font-weight: 700; pointer-events: none;">
            SECURE
          </div>

          <!-- Header -->
          <div class="center-align" style="margin-bottom: 2.2rem; position: relative; z-index: 2;">
            <div class="auth-icon-wrap" aria-hidden="true" style="margin-bottom: 16px;">
              <span style="font-size: 1.3rem; color: var(--wc-accent); font-weight: bold; letter-spacing: 0.15em;">●</span>
            </div>
            <h1 class="auth-title" style="font-family: 'Cormorant Garamond', serif !important; font-size: 2.8rem !important; font-weight: 900 !important; text-transform: uppercase; letter-spacing: 0.12em; margin: 0 0 6px 0 !important; line-height: 0.9 !important; color: var(--wc-text);">Kifu Store</h1>
            <p class="auth-subtitle" style="font-family: 'JetBrains Mono', sans-serif; font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.12em; color: var(--wc-accent); font-weight: bold;">
              SIGN-IN / PRIVATE ARCHIVE
            </p>
          </div>

          {#if error}
            <div class="error-panel" style="border-radius: 0px; border: 1.5px solid var(--wc-text); background: var(--wc-surface-alt); color: var(--wc-text); font-weight: 600; padding: 12px; margin-bottom: 20px; box-shadow: 3px 3px 0px var(--wc-text);">
              <i class="material-icons" style="font-size: 1.1rem; vertical-align: middle; margin-right: 6px;">error_outline</i>
              <span>{error}</span>
            </div>
          {/if}

          <!-- Social Login Buttons -->
          {#if loading}
            <div class="center-align" style="margin: 2.5rem 0;">
              <div class="nm-spinner mx-auto" style="border-top-color: var(--wc-text); width: 36px; height: 36px;"></div>
              <p class="text-muted" style="margin-top: 12px; font-size: 0.78rem; font-family: 'JetBrains Mono', monospace;">CONNECTING...</p>
            </div>
          {:else}
            {#if !providers.google && !providers.line && !providers.meta}
              <div class="center-align" style="margin-top: 15px; border: 2px solid var(--wc-text); padding: 24px 16px; background: var(--wc-surface-alt); box-shadow: 4px 4px 0px var(--wc-text);">
                <i class="material-icons" style="font-size: 2.2rem; color: var(--wc-accent); margin-bottom: 8px; display: block;">info_outline</i>
                <p class="font-outfit" style="margin: 0; font-weight: 700; font-size: 0.95rem; color: var(--wc-text);">ソーシャルログインは現在無効化されています</p>
                <p style="margin: 6px 0 0 0; font-size: 0.78rem; color: var(--wc-text-muted);">管理者へお問い合わせください。</p>
              </div>
            {:else}
              <div class="social-login-grid" style="display: flex; flex-direction: column; gap: 14px;">
                <!-- Google Button -->
                {#if providers.google}
                  <button class="social-btn google-btn em-pulse-button" onclick={() => handleOAuth('google')} style="border-radius: 0px !important; box-shadow: 3px 3px 0px var(--wc-text) !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important; transition: all 0.2s ease;">
                    <span class="social-btn-icon google-icon" style="font-family: 'JetBrains Mono', monospace; font-weight: 700; border: 1.5px solid var(--wc-text); border-radius: 0; background: var(--wc-surface-alt); color: var(--wc-text);">G</span>
                    <span style="font-weight: 700; font-size: 0.85rem; font-family: 'DM Sans', sans-serif; color: var(--wc-text);">Continue with Google</span>
                    <span class="social-btn-arrow" style="font-family: 'JetBrains Mono', monospace; font-weight: bold; color: var(--wc-text);">→</span>
                  </button>
                {/if}

                <!-- LINE Button -->
                {#if providers.line}
                  <button class="social-btn line-btn em-pulse-button" onclick={() => handleOAuth('line')} style="border-radius: 0px !important; box-shadow: 3px 3px 0px var(--wc-text) !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important; transition: all 0.2s ease; animation-delay: -1s;">
                    <i class="material-icons social-btn-icon" style="border: 1.5px solid var(--wc-text); border-radius: 0; padding: 4px; font-size: 1.1rem; width: 28px; height: 28px; display: flex; align-items: center; justify-content: center; background: var(--wc-surface-alt); color: var(--wc-text);">chat</i>
                    <span style="font-weight: 700; font-size: 0.85rem; font-family: 'DM Sans', sans-serif; color: var(--wc-text);">Continue with LINE</span>
                    <span class="social-btn-arrow" style="font-family: 'JetBrains Mono', monospace; font-weight: bold; color: var(--wc-text);">→</span>
                  </button>
                {/if}

                <!-- Meta Button -->
                {#if providers.meta}
                  <button class="social-btn meta-btn em-pulse-button" onclick={() => handleOAuth('meta')} style="border-radius: 0px !important; box-shadow: 3px 3px 0px var(--wc-text) !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important; transition: all 0.2s ease; animation-delay: -2s;">
                    <i class="material-icons social-btn-icon" style="border: 1.5px solid var(--wc-text); border-radius: 0; padding: 4px; font-size: 1.1rem; width: 28px; height: 28px; display: flex; align-items: center; justify-content: center; background: var(--wc-surface-alt); color: var(--wc-text);">facebook</i>
                    <span style="font-weight: 700; font-size: 0.85rem; font-family: 'DM Sans', sans-serif; color: var(--wc-text);">Continue with Meta</span>
                    <span class="social-btn-arrow" style="font-family: 'JetBrains Mono', monospace; font-weight: bold; color: var(--wc-text);">→</span>
                  </button>
                {/if}
              </div>
            {/if}
          {/if}

          <!-- Footer deco -->
          <p class="auth-footer-deco" style="font-family: 'JetBrains Mono', monospace; font-size: 0.62rem; color: var(--wc-text-muted); opacity: 0.7; margin-top: 30px; letter-spacing: 0.22em; text-transform: uppercase; text-align: center;">○ ARCHIVE ACCESS PLATFORM ○</p>
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

  /* モバイル・タッチデバイスでのログインボタン・ソーシャルボタンの拡大 */
  @media (pointer: coarse), only screen and (max-width: 1024px) {
    .social-btn {
      padding: 14px 24px !important;
      font-size: 1.05rem !important;
      min-height: 48px !important;
      border-radius: 0px !important;
    }
    .social-btn-icon {
      width: 32px !important;
      height: 32px !important;
      font-size: 1.2rem !important;
      line-height: 32px !important;
    }
    .social-btn span:last-child {
      font-size: 0.95rem !important;
    }
    .auth-title {
      font-size: 2.2rem !important;
    }
  }
</style>
