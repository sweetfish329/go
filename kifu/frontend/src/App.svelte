<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Toaster } from 'svelte-sonner';
  import KifuList from './components/KifuList.svelte';
  import KifuDetail from './components/KifuDetail.svelte';
  import Auth from './components/Auth.svelte';
  import UsernameDialog from './components/UsernameDialog.svelte';
  import KifuCreator from './components/KifuCreator.svelte';
  import AdminAuth from './components/AdminAuth.svelte';
  import AdminDashboard from './components/AdminDashboard.svelte';
  import { auth } from './lib/auth.svelte';

  let currentView = $state<"list" | "detail" | "auth" | "create" | "admin_auth" | "admin_dashboard" | "public_list" | "loading">("loading");
  let selectedKifuId = $state("");
  let selectedShareToken = $state("");
  let selectedUserId = $state("");
  let showUsernameDialog = $state(false);

  // 共有URL経由アクセスかどうか（ヘッダーをミニマルにする判定）
  const isSharedView = $derived(
    !!(selectedShareToken) ||
    (currentView === 'detail' && !!selectedUserId && !auth.isLoggedIn) ||
    (currentView === 'public_list' && !!selectedUserId && !auth.isLoggedIn)
  );

  let siteSettings = $state({
    title: 'kifu_store',
    tab_name: 'kifu_store',
    favicon: '',
    theme_color: '#4e342e'
  });
  


  // Dark mode state: "light" | "dark" | "system"
  let themeMode = $state<"light" | "dark" | "system">("system");
  let mediaQuery: MediaQueryList;

  // Apply theme to document element
  function applyTheme(mode: "light" | "dark" | "system") {
    if (typeof window === 'undefined') return;
    const root = document.documentElement;
    if (mode === "light") {
      root.classList.add("light");
      root.classList.remove("dark");
    } else if (mode === "dark") {
      root.classList.add("dark");
      root.classList.remove("light");
    } else {
      root.classList.remove("light");
      root.classList.remove("dark");
    }
  }

  // Toggle dark/light/system theme
  function toggleTheme() {
    if (themeMode === "light") {
      themeMode = "dark";
    } else if (themeMode === "dark") {
      themeMode = "system";
    } else {
      themeMode = "light";
    }
    localStorage.setItem("theme", themeMode);
    applyTheme(themeMode);
  }

  // System theme change handler
  function handleSystemThemeChange() {
    if (themeMode === "system") {
      applyTheme("system");
    }
  }

  // Perform routing based on URL path
  function handleRouting() {
    const path = window.location.pathname;
    const params = new URLSearchParams(window.location.search);
    const admin = params.has('admin');

    // Handle OAuth2 callback redirect parameters
    const oauthSuccess = params.get('oauth_success');
    if (oauthSuccess) {
      // Clean query parameters from URL without reloading
      const url = new URL(window.location.href);
      url.searchParams.delete('oauth_success');
      window.history.replaceState({}, '', url.pathname + url.search);
    }

    // Pattern: /u/:userId/:kifuId
    const kifuDetailMatch = path.match(/^\/u\/([^/]+)\/([^/]+)\/?$/);
    // Pattern: /u/:userId
    const publicListMatch = path.match(/^\/u\/([^/]+)\/?$/);

    if (kifuDetailMatch) {
      selectedUserId = kifuDetailMatch[1];
      selectedKifuId = kifuDetailMatch[2];
      selectedShareToken = "";
      currentView = "detail";
    } else if (publicListMatch) {
      selectedUserId = publicListMatch[1];
      selectedKifuId = "";
      selectedShareToken = "";
      currentView = "public_list";
    } else if (admin) {
      const adminToken = localStorage.getItem("admin_token");
      if (adminToken) {
        currentView = "admin_dashboard";
      } else {
        currentView = "admin_auth";
      }
    } else {
      // For backwards compatibility, still support ?share=TOKEN
      const share = params.get('share');
      if (share) {
        selectedShareToken = share;
        selectedUserId = "";
        selectedKifuId = "";
        currentView = "detail";
      } else {
        selectedUserId = "";
        selectedKifuId = "";
        selectedShareToken = "";
        if (!auth.isLoggedIn) {
          currentView = "auth";
        } else {
          currentView = "list";
        }
      }
    }
  }

  // Determine view on mount based on URL query params & auth state
  onMount(async () => {
    // 1. Fetch current authentication status first
    await auth.checkAuth();

    // 2. Run routing to navigate to appropriate view
    handleRouting();

    // 2. Load theme from localStorage
    const savedTheme = localStorage.getItem("theme");
    if (savedTheme === "light" || savedTheme === "dark" || savedTheme === "system") {
      themeMode = savedTheme;
    }
    applyTheme(themeMode);

    // Setup OS theme change listener and scroll listener
    if (typeof window !== 'undefined') {
      mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
      mediaQuery.addEventListener('change', handleSystemThemeChange);
    }

    // 3. Fetch site settings in the background without blocking render flow
    try {
      const res = await fetch('/api/site-settings');
      if (res.ok) {
        const data = await res.json();
        siteSettings.title = data.title || 'kifu_store';
        siteSettings.tab_name = data.tab_name || 'kifu_store';
        siteSettings.favicon = data.favicon || '';
        siteSettings.theme_color = data.theme_color || '#4e342e';
      }
    } catch (err) {
      console.error("Failed to load site settings:", err);
    }

    window.addEventListener('popstate', handleRouting);
  });

  onDestroy(() => {
    if (typeof window !== 'undefined') {
      window.removeEventListener('popstate', handleRouting);
      if (mediaQuery) {
        mediaQuery.removeEventListener('change', handleSystemThemeChange);
      }
    }
  });

  // Reactively apply settings to DOM
  $effect(() => {
    document.documentElement.style.setProperty('--theme-color', siteSettings.theme_color);
    
    // Dynamically update document title based on the current view for client-side SEO
    if (currentView !== "detail" && currentView !== "loading") {
      if (currentView === "public_list") {
        document.title = `公開棋譜一覧 | ${siteSettings.tab_name}`;
      } else if (currentView === "admin_dashboard") {
        document.title = `管理ダッシュボード | ${siteSettings.tab_name}`;
      } else if (currentView === "admin_auth") {
        document.title = `管理者ログイン | ${siteSettings.tab_name}`;
      } else if (currentView === "create") {
        document.title = `新規棋譜記録 | ${siteSettings.tab_name}`;
      } else if (currentView === "list") {
        document.title = `マイ棋譜一覧 | ${siteSettings.tab_name}`;
      } else if (currentView === "auth") {
        document.title = `ログイン・新規登録 | ${siteSettings.tab_name}`;
      } else {
        document.title = siteSettings.tab_name;
      }
    }
    
    let faviconLink = document.querySelector("link[rel~='icon']") as HTMLLinkElement;
    if (!faviconLink) {
      faviconLink = document.createElement('link');
      faviconLink.rel = 'icon';
      document.head.appendChild(faviconLink);
    }
    if (siteSettings.favicon) {
      faviconLink.href = siteSettings.favicon;
    } else {
      faviconLink.href = '/kifu-favicon.ico';
    }
  });

  function handleSelectKifu(event: CustomEvent<string>) {
    selectedKifuId = event.detail;
    if (selectedUserId) {
      window.history.pushState({}, '', `/u/${selectedUserId}/${selectedKifuId}`);
      handleRouting();
    } else {
      currentView = "detail";
    }
  }

  function handleBackToList() {
    if (selectedUserId && currentView === "detail") {
      window.history.pushState({}, '', `/u/${selectedUserId}`);
      handleRouting();
      return;
    }

    if (selectedShareToken) {
      window.history.replaceState({}, '', window.location.pathname);
      selectedShareToken = "";
    }
    
    selectedKifuId = "";
    selectedUserId = "";
    if (auth.isLoggedIn) {
      currentView = "list";
    } else {
      currentView = "auth";
    }
  }

  function handleGoHome() {
    window.history.pushState({}, '', '/');
    selectedUserId = "";
    selectedKifuId = "";
    selectedShareToken = "";
    handleRouting();
  }

  function handleLoginSuccess() {
    currentView = "list";
  }

  function handleLogout() {
    auth.logout();
    currentView = "auth";
  }
</script>

<!-- svelte-sonner Toast Container -->
<Toaster
  richColors
  position="top-center"
  toastOptions={{
    style: 'font-family: Outfit, sans-serif; border-radius: 16px; backdrop-filter: blur(12px); border: 1px solid rgba(255,255,255,0.5);'
  }}
/>

<div class="app-shell" style="min-height: 100vh; padding-bottom: 2rem; overflow-x: hidden; position: relative;">
  <!-- Parallax Scrolling Dots Background: Moves slower than content to create depth -->
  <div 
    class="em-bg-pulse-dots em-bg-pulse-dots-parallax" 
    style="position: absolute; top: 0; left: 0; width: 100%; height: 100%; z-index: -2; pointer-events: none;"
  ></div>

  <!-- Newspaper/Portfolio Masthead Header -->
  <header class="container" class:compact-header={currentView === 'detail' || currentView === 'create'} class:shared-header={isSharedView} style="position: relative;">
    <!-- Huge background decoration text for extreme editorial contrast (gorgous deep parallax) -->
    <div 
      class="em-huge-title masthead-bg-title masthead-bg-title-parallax" 
    >
      RECORDINGS
    </div>

    <div class="em-newspaper-masthead" style="position: relative; padding: 20px 0 10px 0;">
      <div class="masthead-flex-row">
        <!-- Left decoration: collage tag (Parallax layered) -->
        <div class="parallax-tag-left" style="display: inline-block;">
          <span class="em-collage-tag-pastel em-float-badge" style="font-size: 0.65rem; font-family: 'JetBrains Mono', monospace; box-shadow: 3px 3px 0px var(--wc-text); border-width: 2px;">
            EDITION II // TOKYO
          </span>
        </div>

        <!-- Center Title (Giant jump-rate) -->
        <a href="/" onclick={(e) => { e.preventDefault(); handleGoHome(); }} class="cursor-pointer masthead-title-link" style="text-decoration: none; position: relative; display: block;">
          <!-- Overlap ornament text -->
          <span class="masthead-ornament-text" style="position: absolute; top: -18px; left: -14px; font-family: 'Cormorant Garamond', serif; font-size: 0.95rem; font-style: italic; color: var(--wc-accent); letter-spacing: 0.18em; font-weight: 600; text-shadow: 2px 2px 0 var(--wc-bg);">
            the collection of
          </span>
          <span class="masthead-site-title">
            {siteSettings.title}
          </span>
        </a>

        <!-- Right decoration: collage tag (slanted yellow, Parallax layered) -->
        <div class="parallax-tag-right" style="display: inline-block;">
          <span class="em-collage-tag em-float-badge" style="font-size: 0.65rem; font-family: 'JetBrains Mono', monospace; border-width: 2px; animation-delay: -2s;">
            ISSUE // 2026
          </span>
        </div>
      </div>

      <!-- Meta Bar: Slanted/Collaged actions -->
      <div class="em-newspaper-meta-bar" style="border: 1.5px solid var(--wc-text); background: var(--wc-surface); box-shadow: 3px 3px 0px var(--wc-text); padding: 8px 16px; margin-bottom: 15px;">
        <div class="meta-info-group">
          <span style="font-weight: 600;">{new Date().toLocaleDateString('en-US', { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' })}</span>
          <span style="opacity: 0.5; font-family: 'JetBrains Mono', monospace;">●</span>
          <span style="font-weight: 600; letter-spacing: 0.05em; font-family: 'JetBrains Mono', sans-serif;">CHRONICLE SELECTOR</span>
        </div>
        
        <div class="meta-action-group">
          <!-- Home Link -->
          <a href="/" onclick={(e) => { e.preventDefault(); handleGoHome(); }} class="cursor-pointer nav-link" style="color: var(--wc-text); text-decoration: underline; text-underline-offset: 3px; font-size: 0.72rem; letter-spacing: 0.05em; font-family: 'JetBrains Mono', sans-serif; font-weight: bold;">Home</a>
          
          {#if auth.isLoggedIn}
            <!-- Username chip -->
            <button type="button" onclick={() => showUsernameDialog = true} class="cursor-pointer nav-link" style="color: var(--wc-text); text-decoration: none; font-size: 0.72rem; font-weight: bold; letter-spacing: 0.05em; font-family: 'JetBrains Mono', sans-serif; background: var(--wc-surface-alt); border: 1px solid var(--wc-text); padding: 2px 8px; box-shadow: 2px 2px 0px var(--wc-text); height: auto !important; line-height: normal !important;" aria-label="ユーザー名を変更する">
              @{auth.username}
            </button>
            <!-- Logout Link -->
            <button type="button" onclick={handleLogout} class="cursor-pointer nav-link" style="color: var(--wc-text); text-decoration: underline; text-underline-offset: 3px; font-size: 0.72rem; letter-spacing: 0.05em; font-family: 'JetBrains Mono', sans-serif; background: transparent; border: none; padding: 0; cursor: pointer; display: inline-flex; align-items: center;">Logout</button>
          {/if}
          
          <!-- Theme Toggle -->
          <button type="button" onclick={toggleTheme} class="cursor-pointer" style="color: var(--wc-text); display: flex; align-items: center; border: 1px solid var(--wc-text); background: var(--wc-surface); padding: 4px; box-shadow: 2px 2px 0px var(--wc-text); cursor: pointer;" aria-label="テーマの切り替え">
            <i class="material-icons" aria-hidden="true" style="font-size: 0.95rem; color: var(--wc-accent-warm);">
              {themeMode === 'light' ? 'wb_sunny' : themeMode === 'dark' ? 'brightness_2' : 'brightness_auto'}
            </i>
          </button>
        </div>
      </div>
    </div>
  </header>

  <!-- Main Container -->
  <main class="container" class:shared-main={isSharedView} style="padding-bottom: 5rem; padding-top: 2rem;">
    {#if currentView === "loading"}
      <div class="center-align" style="margin-top: 5rem;">
        <div class="nm-spinner" style="width: 48px; height: 48px; margin: 0 auto;"></div>
      </div>
    {:else if currentView === "auth"}
      <Auth />
    {:else if currentView === "list"}
      <KifuList on:selectKifu={handleSelectKifu} on:createKifu={() => currentView = "create"} />
    {:else if currentView === "public_list"}
      <KifuList userId={selectedUserId} on:selectKifu={handleSelectKifu} />
    {:else if currentView === "create"}
      <KifuCreator onSaveSuccess={handleLoginSuccess} onCancel={handleBackToList} />
    {:else if currentView === "detail" && (selectedKifuId || selectedShareToken)}
      <KifuDetail kifuId={selectedKifuId} shareToken={selectedShareToken} userId={selectedUserId} onBack={handleBackToList} />
    {:else if currentView === "admin_auth"}
      <AdminAuth onLoginSuccess={() => currentView = "admin_dashboard"} />
    {:else if currentView === "admin_dashboard"}
      <AdminDashboard onLogout={() => currentView = "admin_auth"} />
    {/if}
  </main>
</div>

{#if showUsernameDialog}
  <UsernameDialog 
    onClose={() => showUsernameDialog = false} 
    onSuccess={() => {
      showUsernameDialog = false;
    }} 
  />
{/if}

<style>
  /* ---- Masthead Header Styling ---- */
  header.container {
    margin-top: 3rem !important;
    margin-bottom: 2rem !important;
    transition: margin 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);
  }

  /* 棋譜詳細・作成画面の時にヘッダーを小さくして碁盤の表示領域を最大化 */
  header.container.compact-header {
    margin-top: 1rem !important;
    margin-bottom: 0.8rem !important;
  }
  header.container.compact-header .masthead-bg-title {
    display: none !important;
  }
  header.container.compact-header .masthead-site-title {
    font-size: 2.2rem !important;
  }
  header.container.compact-header .em-newspaper-masthead {
    padding: 10px 0 5px 0 !important;
  }
  header.container.compact-header .masthead-flex-row {
    margin-bottom: 10px !important;
  }
  header.container.compact-header .em-newspaper-meta-bar {
    margin-bottom: 5px !important;
    padding: 6px 12px !important;
  }
  header.container.compact-header .masthead-title-link > span:first-child {
    display: none !important;
  }

  /* ---- 共有URLアクセス時：ミニマルヘッダー ---- */
  header.container.shared-header {
    margin-top: 0 !important;
    margin-bottom: 0 !important;
    padding-top: 0 !important;
    padding-bottom: 0 !important;
  }
  /* すべてのデコレーション・マストヘッドを非表示 */
  header.container.shared-header .masthead-bg-title,
  header.container.shared-header .parallax-tag-left,
  header.container.shared-header .parallax-tag-right,
  header.container.shared-header .masthead-ornament-text,
  header.container.shared-header .em-newspaper-meta-bar {
    display: none !important;
  }
  /* マストヘッド全体を最小化 */
  header.container.shared-header .em-newspaper-masthead {
    padding: 8px 0 4px 0 !important;
    border-bottom: 1px solid var(--wc-border, rgba(37,53,48,0.15)) !important;
    margin-bottom: 0 !important;
  }
  header.container.shared-header .masthead-flex-row {
    justify-content: center !important;
    margin-bottom: 0 !important;
  }
  /* サイト名をコンパクトに */
  header.container.shared-header .masthead-site-title {
    font-size: 1.1rem !important;
    letter-spacing: 0.25em !important;
    opacity: 0.55 !important;
  }
  /* mainの上部余白もリセット */
  main.container.shared-main {
    padding-top: 0.75rem !important;
  }

  .app-shell {
    min-height: 100vh;
    position: relative;
  }

  /* ---- Masthead responsive classes ---- */
  .masthead-bg-title {
    position: absolute;
    top: -20px;
    left: 50%;
    transform: translateX(-50%);
    opacity: 0.08;
    font-size: 9.5rem;
    letter-spacing: 0.08em;
    width: 100%;
    text-align: center;
    font-family: 'Cormorant Garamond', serif;
    font-weight: 700;
    white-space: nowrap;
    overflow: hidden;
    pointer-events: none;
    z-index: -1;
  }

  .masthead-flex-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 24px;
    gap: 16px;
    flex-wrap: nowrap;
  }

  .meta-info-group {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  .meta-action-group {
    display: flex;
    align-items: center;
    gap: 18px;
    flex-wrap: wrap;
  }

  .masthead-site-title {
    font-family: 'Cormorant Garamond', serif;
    font-size: 4.8rem;
    font-weight: 900;
    text-transform: uppercase;
    letter-spacing: 0.12em;
    color: var(--wc-text);
    line-height: 0.85;
    display: block;
    white-space: nowrap;
  }

  /* スマホ向けレスポンシブ */
  @media (max-width: 480px) {
    .masthead-bg-title {
      font-size: 4rem;
    }

    .masthead-flex-row {
      flex-wrap: wrap;
      justify-content: center;
      align-items: center;
      gap: 6px;
    }

    /* モバイルではバッジを上段に、タイトルを2段目に全幅で */
    .masthead-flex-row > :first-child {
      order: 1;
    }
    .masthead-flex-row > :last-child {
      order: 2;
    }
    .masthead-title-link {
      order: 3;
      width: 100%;
      text-align: center;
    }

    .masthead-site-title {
      font-size: clamp(1.8rem, 8vw, 2.4rem);
      letter-spacing: 0.04em;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      max-width: 100%;
    }

    .masthead-title-link > span:first-child {
      left: 50%;
      transform: translateX(-50%);
      text-align: center;
      width: max-content;
    }

    .masthead-ornament-text {
      display: none;
    }

    .em-newspaper-meta-bar {
      flex-direction: column;
      align-items: center;
      gap: 12px;
      padding: 12px 16px;
    }

    .meta-info-group {
      justify-content: center;
      font-size: 0.65rem;
    }

    .meta-action-group {
      justify-content: center;
      gap: 12px;
      width: 100%;
    }
  }

  @media (max-width: 360px) {
    .masthead-site-title {
      font-size: 2rem;
    }
  }

  main {
    min-height: 80vh;
    position: relative;
  }

  .cursor-pointer {
    cursor: pointer !important;
  }

  /* ---- Nav Logo ---- */
  .nav-logo-icon {
    font-size: 1rem;
    color: var(--wc-accent-warm);
    display: inline-block;
    line-height: 1;
    transition: var(--wc-transition-fast);
    opacity: 0.85;
  }

  .brand-logo:hover .nav-logo-icon {
    opacity: 1;
    transform: rotate(180deg);
  }

  .nav-logo-text {
    font-family: 'Cormorant Garamond', serif !important;
    font-size: 1.25rem;
    font-weight: 500;
    letter-spacing: 0.16em;
    color: var(--wc-text);
    text-transform: uppercase;
  }

  /* ---- Nav Links ---- */
  .nav-link {
    display: inline-flex !important;
    align-items: center;
    gap: 5px;
    height: 40px !important;
    line-height: 40px !important;
    border-radius: 999px !important;
    padding: 0 14px !important;
    font-family: 'Outfit', sans-serif;
    font-weight: 500;
    font-size: 0.93rem;
    transition: all 0.2s ease;
    color: var(--nm-text-main) !important;
  }

  .nav-link:hover {
    background: var(--wc-accent-soft) !important;
    color: var(--wc-accent) !important;
  }

  /* User chip */
  .nav-user-chip {
    border: 1px solid var(--wc-border) !important;
    background: rgba(245, 240, 232, 0.45) !important;
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
  }

  .nav-user-chip:hover {
    background: var(--wc-accent-soft) !important;
    border-color: rgba(124, 107, 82, 0.4) !important;
  }

  .user-avatar {
    font-size: 0.95rem;
    color: var(--wc-accent);
    opacity: 0.7;
  }

  /* Theme toggle button */
  .nav-theme-btn {
    display: inline-flex !important;
    align-items: center;
    justify-content: center;
    width: 40px !important;
    height: 40px !important;
    border-radius: 50% !important;
    padding: 0 !important;
    color: var(--wc-accent) !important;
    border: 1px solid var(--wc-border) !important;
    background: rgba(245, 240, 232, 0.45) !important;
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    transition: all 0.25s ease;
  }

  .nav-theme-btn:hover {
    background: var(--wc-accent-soft) !important;
    border-color: rgba(124, 107, 82, 0.4) !important;
    transform: rotate(15deg);
  }

  /* ---- Scroll-Driven Parallax ---- */
  .masthead-bg-title-parallax {
    transform: translate(-50%, -20px);
  }

  @keyframes parallax-bg-dots {
    to { transform: translateY(300px); }
  }
  @keyframes parallax-bg-title {
    to { transform: translate(-50%, 630px); }
  }
  @keyframes parallax-tag-left {
    to { transform: translateY(-150px); }
  }
  @keyframes parallax-tag-right {
    to { transform: translateY(-220px); }
  }

  @supports (animation-timeline: scroll()) {
    .em-bg-pulse-dots-parallax {
      animation: parallax-bg-dots linear forwards;
      animation-timeline: scroll(root);
    }
    .masthead-bg-title-parallax {
      animation: parallax-bg-title linear forwards;
      animation-timeline: scroll(root);
    }
    .parallax-tag-left {
      animation: parallax-tag-left linear forwards;
      animation-timeline: scroll(root);
    }
    .parallax-tag-right {
      animation: parallax-tag-right linear forwards;
      animation-timeline: scroll(root);
    }
  }
</style>
