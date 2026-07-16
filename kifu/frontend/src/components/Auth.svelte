<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Board from './Board.svelte';
  import { SgfPlayer } from '../lib/sgfPlayer';

  let error = $state<string | null>(null);
  let loading = $state(true);
  let providers = $state<Record<string, boolean>>({
    google: false,
    line: false,
    meta: false
  });

  // SGF and Board states
  let kifuData = $state<any>(null);
  let sgfText = $state("");
  let player = $state<SgfPlayer | null>(null);
  let boardState = $state<number[][]>([]);
  let lastMove = $state<{ x: number; y: number } | null>(null);
  let boardSize = $state(19);

  // Scroll and Scrollytelling states
  let containerEl = $state<HTMLElement | null>(null);
  let scrollPercent = $state(0);
  let activeSection = $state(1);
  let boardOpacity = $state(1.0);
  let transitionTimer: any = null;

  // Autoplay states (runs when user is not scrolling / at the top)
  let autoplayInterval: any = null;
  let autoplayDirection = $state(1); // 1 = forward, -1 = backward, 0 = paused/manual
  let userInteracted = $state(false);

  // Fallback famous SGF: Shusaku's Ear-Reddening Game (first 60 moves)
  const fallbackSgf = `(;GM[1]FF[4]SZ[19]KM[0.0]PB[安田栄斎]BR[四段]PW[幻庵因碩]WR[八段]RE[B+3]DT[1846-09-11]EV[赤耳局];B[qd];W[dc];B[pq];W[oc];B[cp];W[po];B[qo];W[qn];B[ro];W[pp];B[qp];W[oq];B[op];W[pn];B[np];W[qq];B[pr];W[or];B[qr];W[rq];B[rr];W[rp];B[rn];W[rm];B[qm];W[ql];B[pm];W[om];B[pl];W[pk];B[ol];W[nl];B[ok];W[oj];B[nk];W[mk];B[nj];W[ni];B[mj];W[lj];B[mi];W[mh];B[li];W[ki];B[lh];W[lg];B[kh];W[jh];B[kg];W[kf];B[jg];W[ig];B[jf];W[je];B[if];W[hf];B[ie];W[id];B[he];W[ge])`;

  onMount(async () => {
    // 1. Fetch OAuth providers
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

    // 2. Fetch random public SGF
    try {
      const res = await fetch('/api/public/random');
      if (res.ok) {
        kifuData = await res.json();
        sgfText = kifuData.sgf_data;
      } else {
        sgfText = fallbackSgf;
      }
    } catch (err) {
      console.warn("Failed to fetch random public kifu, using fallback Shusaku SGF", err);
      sgfText = fallbackSgf;
    }

    // 3. Initialize SgfPlayer
    if (sgfText) {
      boardSize = sgfText.includes("SZ[9]") ? 9 : sgfText.includes("SZ[13]") ? 13 : 19;
      player = new SgfPlayer(sgfText, boardSize);
      updateBoardState();
      startAutoplay();
    }

    // 4. Add scroll listener
    window.addEventListener('scroll', handleScroll, { passive: true });
    setTimeout(handleScroll, 150); // Initial layout evaluation
  });

  onDestroy(() => {
    if (autoplayInterval) clearInterval(autoplayInterval);
    if (animationTimer) clearTimeout(animationTimer);
    if (transitionTimer) clearTimeout(transitionTimer);
    window.removeEventListener('scroll', handleScroll);
  });

  function updateBoardState() {
    if (!player) return;
    const state = player.getCurrentState();
    boardState = state.board;
    lastMove = state.lastMove;
  }

  function startAutoplay() {
    if (autoplayInterval) clearInterval(autoplayInterval);
    autoplayInterval = setInterval(() => {
      if (userInteracted || !player) return;
      
      const maxIdx = player.history.length - 1;
      const limit = Math.min(60, maxIdx); // Limit fallback autoplay to 60 moves to keep it engaging
      
      if (autoplayDirection === 1) {
        if (player.currentIndex < limit) {
          player.stepForward();
        } else {
          autoplayDirection = -1;
        }
      } else if (autoplayDirection === -1) {
        if (player.currentIndex > 0) {
          player.stepBackward();
        } else {
          autoplayDirection = 1;
        }
      }
      updateBoardState();
    }, 1800);
  }

  function handleOAuth(provider: string) {
    loading = true;
    error = null;
    window.location.href = `/api/auth/oauth/redirect/${provider}`;
  }

  // Scroll handler for Scrollytelling
  function handleScroll() {
    if (!containerEl || !player) return;
    const rect = containerEl.getBoundingClientRect();
    const viewportHeight = window.innerHeight;
    
    // We compute scroll progress when the top of the container hits the top of the viewport
    // and ends when the bottom of the container hits the bottom of the viewport
    const totalScrollableHeight = rect.height - viewportHeight;
    
    if (rect.top <= 0 && rect.bottom >= viewportHeight) {
      const currentScroll = -rect.top;
      scrollPercent = Math.max(0, Math.min(1, currentScroll / totalScrollableHeight));
    } else if (rect.top > 0) {
      scrollPercent = 0;
    } else {
      scrollPercent = 1;
    }

    // Determine active section (1 to 4)
    if (scrollPercent < 0.25) {
      activeSection = 1;
    } else if (scrollPercent < 0.5) {
      activeSection = 2;
    } else if (scrollPercent < 0.75) {
      activeSection = 3;
    } else {
      activeSection = 4;
    }

    // Map scroll percentage to SGF move index
    const totalMoves = Math.max(1, player.history.length - 1);
    const targetMove = Math.round(scrollPercent * totalMoves);
    
    if (targetMove !== player.currentIndex) {
      goToMoveSmoothly(targetMove);
    }
  }

  function goToMoveSmoothly(targetMove: number) {
    if (!player) return;
    userInteracted = true; // Pause auto autoplay
    
    const diff = Math.abs(player.currentIndex - targetMove);
    
    if (diff > 15) {
      // Large jump: Fade transition to avoid hectic rendering
      boardOpacity = 0.4;
      if (transitionTimer) clearTimeout(transitionTimer);
      if (animationTimer) clearTimeout(animationTimer);
      
      player.goTo(targetMove);
      updateBoardState();
      
      transitionTimer = setTimeout(() => {
        boardOpacity = 1.0;
        // Resume autoplay after 7 seconds of inactivity
        setTimeout(() => { userInteracted = false; }, 7000);
      }, 150);
    } else {
      // Short distance: Step-by-step rapid progression (time-lapse)
      animateToMove(targetMove);
    }
  }

  let animationTimer: any = null;
  function animateToMove(targetMove: number) {
    if (!player) return;
    
    if (animationTimer) clearTimeout(animationTimer);
    
    const step = () => {
      if (!player) return;
      const cur = player.currentIndex;
      if (cur === targetMove) {
        // Resume autoplay after 7 seconds of inactivity
        setTimeout(() => { userInteracted = false; }, 7000);
        return;
      }
      
      if (cur < targetMove) {
        player.stepForward();
      } else {
        player.stepBackward();
      }
      updateBoardState();
      
      animationTimer = setTimeout(step, 60); // Fast incremental step
    };
    
    step();
  }

  // Playback manual navigation
  function handlePrevMove() {
    if (!player) return;
    userInteracted = true;
    player.stepBackward();
    updateBoardState();
  }

  function handleNextMove() {
    if (!player) return;
    userInteracted = true;
    player.stepForward();
    updateBoardState();
  }

  function handleTogglePlay() {
    if (autoplayDirection !== 0) {
      autoplayDirection = 0; // Pause
      if (autoplayInterval) {
        clearInterval(autoplayInterval);
        autoplayInterval = null;
      }
    } else {
      autoplayDirection = 1; // Play
      userInteracted = false;
      startAutoplay();
    }
  }
</script>

<div class="auth-container animate-fade-in" style="margin-top: 2rem; position: relative;">
  <!-- Washi Decorative Background Dots -->
  <div class="auth-washi-dot auth-washi-dot--1"></div>
  <div class="auth-washi-dot auth-washi-dot--2"></div>

  <!-- Hero Section -->
  <div class="row hero-section" style="display: flex; flex-wrap: wrap; align-items: center; gap: 30px 0; min-height: 70vh;">
    <!-- Left Column: Giant Editorial Typo (Magazine Style) -->
    <div class="col s12 l7" style="position: relative; z-index: 2; padding-right: 40px; text-align: left;">
      <span class="em-collage-tag-pastel em-float-badge" style="font-size: 0.65rem; font-family: 'JetBrains Mono', monospace; box-shadow: 2px 2px 0px var(--wc-text); border-width: 1.5px; display: inline-block; margin-bottom: 20px;">
        CATALOGUE SYSTEM
      </span>
      <h2 style="font-family: 'Cormorant Garamond', serif; font-size: 5rem; font-style: italic; font-weight: 800; line-height: 0.9; color: var(--wc-text); margin: 0 0 20px 0; letter-spacing: -0.02em;">
        kifu_store.
      </h2>
      <div style="border-left: 3px solid var(--wc-text); padding-left: 20px; max-width: 440px;">
        <p style="font-family: 'Shippori Mincho B1', serif; font-weight: 700; font-size: 1.1rem; line-height: 1.6; color: var(--wc-text); margin: 0 0 12px 0;">
          美しさと戦術が交錯する、私的な記録庫へ。
        </p>
        <p style="font-family: 'DM Sans', sans-serif; font-size: 0.85rem; line-height: 1.6; color: var(--wc-text-muted); margin: 0; margin-bottom: 24px;">
          Kifu Storeは、囲碁の対局記録（棋譜）を美しく保存・再現し、AI分析や共有を行うためのデジタル・アーカイブです。
        </p>
        <div class="font-sans" style="font-size: 0.8rem; font-weight: 700; color: var(--wc-accent); display: flex; align-items: center; gap: 8px;">
          <span>下へスクロールして機能を体験する</span>
          <i class="material-icons animate-bounce" style="font-size: 1rem;">arrow_downward</i>
        </div>
      </div>
      <!-- Huge Deco Number -->
      <div style="position: absolute; bottom: -80px; left: -10px; opacity: 0.05; font-family: 'Cormorant Garamond', serif; font-size: 14rem; font-weight: 900; pointer-events: none; user-select: none;">
        01
      </div>
    </div>

    <!-- Right Column: Login Card -->
    <div class="col s12 l5" style="position: relative; z-index: 3;">
      <div class="em-portfolio-section auth-card" style="border-color: var(--wc-text) !important; transform: rotate(-1.5deg); box-shadow: 8px 8px 0px var(--wc-text) !important; background: var(--wc-surface) !important; transition: transform 0.3s ease;">
        <span class="em-collage-tag-pastel em-float-badge" style="position: absolute; top: -16px; left: 24px; font-size: 0.72rem; z-index: 10; box-shadow: 2.5px 2.5px 0px var(--wc-text); transform: rotate(1deg);">
          ACCESS GATEWAYS
        </span>

        <div class="card-content" style="padding: 3rem 2.2rem; position: relative; z-index: 1;">
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
            <div class="error-panel" aria-live="polite" style="border-radius: 0px; border: 1.5px solid var(--wc-text); background: var(--wc-surface-alt); color: var(--wc-text); font-weight: 600; padding: 12px; margin-bottom: 20px; box-shadow: 3px 3px 0px var(--wc-text);">
              <i class="material-icons" aria-hidden="true" style="font-size: 1.1rem; vertical-align: middle; margin-right: 6px;">error_outline</i>
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
                <i class="material-icons" aria-hidden="true" style="font-size: 2.2rem; color: var(--wc-accent); margin-bottom: 8px; display: block;">info_outline</i>
                <p class="font-outfit" style="margin: 0; font-weight: 700; font-size: 0.95rem; color: var(--wc-text);">ソーシャルログインは現在無効化されています</p>
                <p style="margin: 6px 0 0 0; font-size: 0.78rem; color: var(--wc-text-muted);">管理者へお問い合わせください。</p>
              </div>
            {:else}
              <div class="social-login-grid" style="display: flex; flex-direction: column; gap: 14px;">
                <!-- Google -->
                {#if providers.google}
                  <button class="social-btn google-btn em-pulse-button" onclick={() => handleOAuth('google')} style="border-radius: 0px !important; box-shadow: 3px 3px 0px var(--wc-text) !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important; transition: all 0.2s ease;">
                    <span class="social-btn-icon google-icon" style="font-family: 'JetBrains Mono', monospace; font-weight: 700; border: 1.5px solid var(--wc-text); border-radius: 0; background: var(--wc-surface-alt); color: var(--wc-text);">G</span>
                    <span style="font-weight: 700; font-size: 0.85rem; font-family: 'DM Sans', sans-serif; color: var(--wc-text);">Continue with Google</span>
                    <span class="social-btn-arrow" style="font-family: 'JetBrains Mono', monospace; font-weight: bold; color: var(--wc-text);">→</span>
                  </button>
                {/if}

                <!-- LINE -->
                {#if providers.line}
                  <button class="social-btn line-btn em-pulse-button" onclick={() => handleOAuth('line')} style="border-radius: 0px !important; box-shadow: 3px 3px 0px var(--wc-text) !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important; transition: all 0.2s ease; animation-delay: -1s;">
                    <i class="material-icons social-btn-icon" aria-hidden="true" style="border: 1.5px solid var(--wc-text); border-radius: 0; padding: 4px; font-size: 1.1rem; width: 28px; height: 28px; display: flex; align-items: center; justify-content: center; background: var(--wc-surface-alt); color: var(--wc-text);">chat</i>
                    <span style="font-weight: 700; font-size: 0.85rem; font-family: 'DM Sans', sans-serif; color: var(--wc-text);">Continue with LINE</span>
                    <span class="social-btn-arrow" style="font-family: 'JetBrains Mono', monospace; font-weight: bold; color: var(--wc-text);">→</span>
                  </button>
                {/if}

                <!-- Meta -->
                {#if providers.meta}
                  <button class="social-btn meta-btn em-pulse-button" onclick={() => handleOAuth('meta')} style="border-radius: 0px !important; box-shadow: 3px 3px 0px var(--wc-text) !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important; transition: all 0.2s ease; animation-delay: -2s;">
                    <i class="material-icons social-btn-icon" aria-hidden="true" style="border: 1.5px solid var(--wc-text); border-radius: 0; padding: 4px; font-size: 1.1rem; width: 28px; height: 28px; display: flex; align-items: center; justify-content: center; background: var(--wc-surface-alt); color: var(--wc-text);">facebook</i>
                    <span style="font-weight: 700; font-size: 0.85rem; font-family: 'DM Sans', sans-serif; color: var(--wc-text);">Continue with Meta</span>
                    <span class="social-btn-arrow" style="font-family: 'JetBrains Mono', monospace; font-weight: bold; color: var(--wc-text);">→</span>
                  </button>
                {/if}
              </div>
            {/if}
          {/if}

          <p class="auth-footer-deco" style="font-family: 'JetBrains Mono', monospace; font-size: 0.62rem; color: var(--wc-text-muted); opacity: 0.7; margin-top: 30px; letter-spacing: 0.22em; text-transform: uppercase; text-align: center;">○ ARCHIVE ACCESS PLATFORM ○</p>
        </div>
      </div>
    </div>
  </div>

  <!-- Scrollytelling Section -->
  <div class="scrolly-narrative-container" bind:this={containerEl}>
    <!-- Sticky Board Wrapper -->
    <div class="sticky-board-wrapper">
      <div class="sticky-board-container" style="opacity: {boardOpacity}; transition: opacity 0.15s ease;">
        <span class="em-collage-tag-pastel em-float-badge" style="position: absolute; top: -16px; left: 24px; font-size: 0.65rem; font-family: 'JetBrains Mono', monospace; box-shadow: 2px 2px 0px var(--wc-text); border-width: 1.5px; z-index: 12; transform: rotate(-1deg);">
          LIVE REPLAY ARCHIVE
        </span>

        {#if player && boardState.length > 0}
          <div class="board-wrapper" style="width: 100%; max-width: 320px; aspect-ratio: 1/1; margin: 16px auto 0 auto; box-shadow: 4px 4px 0px var(--wc-text); border: 1.5px solid var(--wc-text);">
            <Board
              board={boardState}
              size={boardSize}
              lastMove={lastMove}
              interactive={false}
            />
          </div>

          <!-- Playback Mini Controls -->
          <div class="board-mini-controls" style="margin-top: 16px; display: flex; align-items: center; justify-content: center; gap: 10px; font-family: 'JetBrains Mono', monospace; font-size: 0.78rem; width: 100%;">
            <button type="button" class="nm-btn-flat font-mono" onclick={handlePrevMove} aria-label="1手戻る" style="padding: 2px 10px; border: 1.5px solid var(--wc-text) !important; border-radius: 0 !important; background: var(--wc-surface) !important; cursor: pointer; height: 30px; display: inline-flex; align-items: center; justify-content: center; box-shadow: 2px 2px 0px var(--wc-text) !important; color: var(--wc-text) !important;">
              <i class="material-icons" style="font-size: 1.15rem;">chevron_left</i>
            </button>
            <button type="button" class="nm-btn-flat font-mono" onclick={handleTogglePlay} aria-label={autoplayDirection === 0 ? "再生" : "一時停止"} style="padding: 2px 10px; border: 1.5px solid var(--wc-text) !important; border-radius: 0 !important; background: var(--wc-surface) !important; cursor: pointer; height: 30px; display: inline-flex; align-items: center; justify-content: center; box-shadow: 2px 2px 0px var(--wc-text) !important; color: var(--wc-text) !important;">
              <i class="material-icons" style="font-size: 1.15rem;">{autoplayDirection === 0 ? 'play_arrow' : 'pause'}</i>
            </button>
            <button type="button" class="nm-btn-flat font-mono" onclick={handleNextMove} aria-label="1手進む" style="padding: 2px 10px; border: 1.5px solid var(--wc-text) !important; border-radius: 0 !important; background: var(--wc-surface) !important; cursor: pointer; height: 30px; display: inline-flex; align-items: center; justify-content: center; box-shadow: 2px 2px 0px var(--wc-text) !important; color: var(--wc-text) !important;">
              <i class="material-icons" style="font-size: 1.15rem;">chevron_right</i>
            </button>
            <span class="move-counter font-mono" style="font-weight: 700; color: var(--wc-text); margin-left: 12px; font-size: 0.85rem; border: 1.5px solid var(--wc-text); padding: 4px 8px; background: var(--wc-surface);">
              {player.currentIndex} / {player.history.length - 1} 手目
            </span>
          </div>

          <!-- Game Info -->
          <div class="game-info-badge" style="margin-top: 16px; width: 100%; max-width: 320px; border: 1.5px solid var(--wc-text); padding: 8px 12px; background: var(--wc-surface); box-shadow: 4px 4px 0px var(--wc-text); text-align: left; box-sizing: border-box;">
            {#if kifuData}
              <div style="font-family: 'Shippori Mincho B1', serif; font-weight: 700; font-size: 0.88rem; color: var(--wc-text); margin-bottom: 4px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;" title={kifuData.title}>
                {kifuData.title}
              </div>
              <div style="font-family: 'DM Sans', sans-serif; font-size: 0.75rem; color: var(--wc-text-muted); display: flex; justify-content: space-between; align-items: center; border-top: 1px dashed var(--wc-border); padding-top: 4px; margin-top: 4px;">
                <span>● {kifuData.black_player || '黒'}</span>
                <span>VS</span>
                <span>○ {kifuData.white_player || '白'}</span>
              </div>
            {:else}
              <div style="font-family: 'Shippori Mincho B1', serif; font-weight: 700; font-size: 0.88rem; color: var(--wc-text); margin-bottom: 4px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
                第12回 秀策の耳赤の一局 (Fallback)
              </div>
              <div style="font-family: 'DM Sans', sans-serif; font-size: 0.75rem; color: var(--wc-text-muted); display: flex; justify-content: space-between; align-items: center; border-top: 1px dashed var(--wc-border); padding-top: 4px; margin-top: 4px;">
                <span>● 安田栄斎</span>
                <span>VS</span>
                <span>○ 幻庵因碩</span>
              </div>
            {/if}
          </div>
        {:else}
          <div style="padding: 2rem 0; text-align: center; color: var(--wc-text-muted); font-family: 'JetBrains Mono', monospace; font-size: 0.8rem; width: 100%;">
            <div class="nm-spinner mx-auto" style="border-top-color: var(--wc-text); width: 24px; height: 24px; margin-bottom: 8px;"></div>
            LOADING BOARD STATE...
          </div>
        {/if}
      </div>
    </div>

    <!-- Scrolling Text Steps -->
    <div class="scrolly-scroll-track">
      <div class="scrolly-step left-side" id="scrolly-sec-1" class:active={activeSection === 1}>
        <div class="scrolly-card">
          <span class="scrolly-tag font-mono">01 / REPLAY</span>
          <h3 class="scrolly-title font-mincho">盤上の美しき再現</h3>
          <p class="scrolly-text font-sans">
            対局の流れを一手ずつ、美しい碁盤上に再現。歴史的銘局からあなたの自戦記まで、生命を吹き込むように棋譜をたどることができます。
          </p>
        </div>
      </div>

      <div class="scrolly-step right-side" id="scrolly-sec-2" class:active={activeSection === 2}>
        <div class="scrolly-card">
          <span class="scrolly-tag font-mono">02 / AI GRAPH</span>
          <h3 class="scrolly-title font-mincho">AI 勝率・目数差分析</h3>
          <p class="scrolly-text font-sans">
            一手ごとの勝率グラフと詳細な形勢判断を視覚化。最善手と悪手の落差を一目で把握し、直感的な形勢・戦術分析を実現します。
          </p>
        </div>
      </div>

      <div class="scrolly-step left-side" id="scrolly-sec-3" class:active={activeSection === 3}>
        <div class="scrolly-card">
          <span class="scrolly-tag font-mono">03 / INSTANT SHARE</span>
          <h3 class="scrolly-title font-mincho">スマート共有とカスタムOGP</h3>
          <p class="scrolly-text font-sans">
            対局の最もエキサイティングな瞬間をOGP画像に指定し、SNSで一発共有。限定公開・一般公開もワンクリックでシームレスに制御できます。
          </p>
        </div>
      </div>

      <div class="scrolly-step right-side" id="scrolly-sec-4" class:active={activeSection === 4}>
        <div class="scrolly-card">
          <span class="scrolly-tag font-mono">04 / COLLABORATIVE REVIEWS</span>
          <h3 class="scrolly-title font-mincho">双方向の添削指導とコメント</h3>
          <p class="scrolly-text font-sans">
            盤上の任意の局面に変化図や解説コメントを書き込み、対局を豊かに分析。他者との添削コラボレーションにより、技術の深まりをサポートします。
          </p>
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

  /* Scrollytelling narrative container */
  .scrolly-narrative-container {
    position: relative;
    width: 100%;
    margin-top: 4rem;
  }

  /* Sticky wrapper that holds the board vertically centered on screen */
  .sticky-board-wrapper {
    position: sticky;
    top: 0;
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none; /* Let scroll events pass to track elements below */
    z-index: 5;
  }

  .sticky-board-container {
    pointer-events: auto; /* Re-enable click for board controls */
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 1.5rem;
    border: 2.5px solid var(--wc-text);
    background: var(--wc-surface-alt);
    box-shadow: 6px 6px 0px var(--wc-text);
    max-width: 380px;
    width: 90%;
    will-change: transform, opacity;
  }

  /* Scrolling track containing the descriptive steps */
  .scrolly-scroll-track {
    position: relative;
    margin-top: -100vh; /* Overlap scroll track with sticky wrapper */
    z-index: 6;
  }

  /* Each step represents a screen of scroll height */
  .scrolly-step {
    height: 100vh;
    display: flex;
    align-items: center;
    padding: 0 10%;
    box-sizing: border-box;
  }

  /* Alternating left and right sides */
  .scrolly-step.left-side {
    justify-content: flex-start;
  }

  .scrolly-step.right-side {
    justify-content: flex-end;
  }

  .scrolly-card {
    width: 35%;
    min-width: 290px;
    padding: 2rem;
    border: 2px solid var(--wc-text);
    background: var(--wc-surface);
    box-shadow: 4px 4px 0px var(--wc-text);
    transition: all 0.4s cubic-bezier(0.25, 0.46, 0.45, 0.94);
    opacity: 0.3;
    transform: translateY(20px);
    will-change: transform, opacity;
  }

  .scrolly-step.active .scrolly-card {
    opacity: 1;
    transform: translateY(0);
    border-color: var(--wc-accent);
    box-shadow: 6px 6px 0px var(--wc-accent);
  }

  .scrolly-tag {
    font-size: 0.72rem;
    font-weight: 700;
    color: var(--wc-accent);
    display: block;
    margin-bottom: 0.5rem;
    letter-spacing: 0.08em;
  }

  .scrolly-title {
    font-size: 1.45rem !important;
    font-weight: 700 !important;
    margin: 0 0 0.8rem 0 !important;
    color: var(--wc-text);
  }

  .scrolly-text {
    font-size: 0.88rem;
    line-height: 1.6;
    color: var(--wc-text-muted);
    margin: 0;
  }

  /* Decorative Washi Dots */
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

  /* Footer decoration */
  .auth-footer-deco {
    text-align: center;
    margin-top: 2rem;
    margin-bottom: 0;
    font-size: 0.9rem;
    letter-spacing: 0.6em;
    color: var(--wc-accent);
    opacity: 0.35;
  }

  .mx-auto {
    margin-left: auto;
    margin-right: auto;
  }

  /* Animations */
  @keyframes bounce {
    0%, 100% { transform: translateY(0); }
    50% { transform: translateY(-4px); }
  }
  :global(.animate-bounce) {
    animation: bounce 1.8s infinite ease-in-out;
  }

  /* Responsive styling for Tablet / Mobile */
  @media (max-width: 800px) {
    .sticky-board-wrapper {
      position: relative;
      height: auto;
      top: 0;
      margin-bottom: 2rem;
      pointer-events: auto;
    }
    
    .scrolly-scroll-track {
      margin-top: 0;
    }

    .scrolly-step {
      height: auto;
      padding: 1.5rem 0;
      justify-content: center !important;
    }

    .scrolly-card {
      width: 100%;
      opacity: 1;
      transform: none;
    }
  }

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
